# Agent 接入与自动触发 Runbook

这份 runbook 用来把同事自己的 Claude Code、Codex、Copilot CLI、OpenCode、OpenClaw、Hermes、Gemini、Pi、Cursor Agent、Kimi 或 Kiro CLI 接入 Multica，并让它们消费 [AI 开发协作流程](ai-development-collaboration-workflow.md)。

目标不是复制某个团队的私有 hook，而是把协作能力变成可迁移的四层：

1. **Runtime 接入**：同事的机器运行 `multica daemon`，Multica 能看到可用 provider。
2. **Agent 配置**：创建 agent，绑定 runtime、模型、并发和安全参数。
3. **Workflow skill 同步**：把团队 SDLC/closeout/验证合同挂到 agent 上。
4. **自动触发**：通过 issue assignment、comment @mention、chat、rerun 或 autopilot 产生任务。

## 0. 当前能力边界

| 能力 | 当前状态 | 说明 |
|---|---|---|
| Daemon 自动检测 agent CLI | 可用 | daemon 启动时扫描 PATH 中的支持命令，并注册 runtime |
| Issue 分配触发 agent | 可用 | issue assignee 是 agent 且 agent 有 runtime 时，会入队任务 |
| Comment @mention 触发 agent | 可用 | 评论中提到 agent 会产生带 `trigger_comment_id` 的任务 |
| Chat 触发 agent | 可用 | 适合探索和问答，不替代正式任务卡 |
| Manual rerun | 可用 | 对同一 issue 重新产生任务 |
| Autopilot manual trigger | 可用 | `multica autopilot trigger <id>` |
| Autopilot cron schedule | 可用 | CLI 暴露 `schedule` trigger |
| Autopilot webhook/API trigger | 暂不作为对外能力 | 数据模型有字段，但当前 CLI 文档说明没有可触发的 server endpoint |
| Run-only autopilot | 暂不作为 CLI 主路径 | CLI 当前只暴露 `create_issue`，让每次自动运行都有 issue audit trail |

## 1. 同事机器接入 runtime

每个要执行 agent 的人，在自己的机器上安装 Multica CLI 和至少一个支持的 agent CLI。

```bash
# Install Multica CLI first. See README for Homebrew, install script, or PowerShell.

# Connect to Multica Cloud, authenticate, and start daemon.
multica setup

# Verify daemon, detected CLIs, and watched workspaces.
multica daemon status --output json
multica runtime list --output json
multica workspace list
```

如果 workspace 没有被 watch：

```bash
multica workspace watch <workspace-id>
multica daemon restart
```

验证标准：

- `multica daemon status --output json` 能看到 daemon 正在运行。
- `multica runtime list --output json` 能看到目标 provider 的 online runtime。
- `multica workspace list` 中目标 workspace 带 `*`。

## 2. 创建或更新 agent

先从 runtime 列表里选一个 online runtime：

```bash
multica runtime list --output json
```

下面的命令用 `jq` 从 JSON 输出里取 id；没有 `jq` 时，也可以直接复制 `--output json` 里的 `id`。

创建 agent：

```bash
RUNTIME_ID="<runtime-id>"

AGENT_ID="$(
  multica agent create \
    --name "Team SDLC Codex" \
    --description "Agent that follows the team AI SDLC workflow" \
    --runtime-id "$RUNTIME_ID" \
    --visibility workspace \
    --max-concurrent-tasks 2 \
    --instructions "Use the Team AI SDLC skill. Treat Multica issues as the task authority. Always close out on the same issue with layered validation." \
    --output json | jq -r '.id'
)"
```

如果 agent 已存在，只更新关键字段：

```bash
multica agent update "$AGENT_ID" \
  --runtime-id "$RUNTIME_ID" \
  --visibility workspace \
  --max-concurrent-tasks 2 \
  --instructions "Use the Team AI SDLC skill. Treat Multica issues as the task authority. Always close out on the same issue with layered validation."
```

建议先把 `max-concurrent-tasks` 控制在 `1` 到 `2`。等 closeout 质量稳定后再提高并发。

## 3. 同步 Team AI SDLC skill

本仓库提供一个可直接复制的 skill 模板：

- [Team AI SDLC skill](examples/team-ai-sdlc-skill.md)

创建 skill：

```bash
SKILL_ID="$(
  multica skill create \
    --name "Team AI SDLC" \
    --description "Issue authority, layered validation, closeout, and auto-trigger hygiene for team AI development." \
    --content "$(cat docs/examples/team-ai-sdlc-skill.md)" \
    --output json | jq -r '.id'
)"
```

绑定到 agent：

```bash
multica agent skills set "$AGENT_ID" --skill-ids "$SKILL_ID"
multica agent skills list "$AGENT_ID"
```

如果团队同时使用 Claude Code 和 Codex，本质上不要复制两套规则。把同一份 skill 当成源头，然后用各 agent 的本地规则入口做 adapter：

| Agent | 推荐同步面 |
|---|---|
| Claude Code | `.claude/skills/` 或 Multica skill |
| Codex | `$CODEX_HOME/skills/` 或 Multica skill |
| Other CLI agents | Agent instructions + Multica skill |

没有稳定 hook surface 的 provider，不要声称已经实现自动治理。先用 skill + issue closeout 模板约束行为。

## 4. 配置自动触发入口

### 4.1 Issue assignment

最稳定的自动触发方式是把 issue 分配给 agent。

```bash
multica issue create \
  --title "Smoke: Team AI SDLC closeout" \
  --description-stdin \
  --assignee-id "$AGENT_ID" \
  --output json <<'EOF'
## Problem
Verify this agent can consume the Team AI SDLC skill.

## Outcome
Agent replies with a same-issue closeout and layered validation.

## Acceptance
- Agent identifies this as an assignment-triggered task.
- Agent marks local validation as not applicable for this docs-only smoke.
- Agent writes a closeout comment to this issue.
EOF
```

### 4.2 Comment @mention

Use this when a human or another agent wants to pull a specific agent into an existing issue.

In the Multica UI, mention the agent from the editor. The stored form is a mention link like:

```md
[@Team SDLC Codex](mention://agent/<agent-id>) please review the latest plan and close out with the Team AI SDLC template.
```

Avoid repeatedly mentioning agents for acknowledgements. That can create comment loops.

### 4.3 Manual rerun

Use rerun when the prior output was wrong or stale:

```bash
multica issue rerun <issue-id> --output json
```

Rerun should be treated as a fresh attempt, not as proof that the previous result was valid.

### 4.4 Scheduled autopilot

Use autopilot for recurring jobs that should leave an issue audit trail.

```bash
AUTOPILOT_ID="$(
  multica autopilot create \
    --title "Daily SDLC triage" \
    --description "Review open AI development issues. Pick one blocked or stale issue, summarize the blocker, and write a same-issue closeout or next step." \
    --agent "$AGENT_ID" \
    --mode create_issue \
    --output json | jq -r '.id'
)"

multica autopilot trigger-add "$AUTOPILOT_ID" \
  --cron "0 9 * * 1-5" \
  --timezone "America/New_York"

multica autopilot trigger "$AUTOPILOT_ID"
multica autopilot runs "$AUTOPILOT_ID" --output json
```

Current recommendation: keep autopilot in `create_issue` mode so every run creates a visible issue and can be reviewed like normal work.

## 5. Verification checklist

Run this after onboarding each teammate or runtime:

```bash
multica daemon status --output json
multica runtime list --output json
multica agent get "$AGENT_ID" --output json
multica agent skills list "$AGENT_ID" --output json
```

Then verify one trigger path:

1. Create a smoke issue assigned to the agent.
2. Wait for a task to appear.
3. Check the issue comments for a closeout.
4. Confirm the closeout separates Local / PR-CI / Release-runtime / Live behavior / External endstate.
5. If using autopilot, check `multica autopilot runs <id> --output json`.

Do not mark onboarding complete only because the daemon is online. The accepted proof is: **runtime online + agent configured + skill attached + at least one trigger produced a same-issue closeout**.

## 6. Security and permission rules

- Do not put API keys in `--custom-env` on the command line. Use `--custom-env-stdin` or `--custom-env-file`.
- Keep each teammate's provider login on their own machine.
- Use `--visibility workspace` only for agents intended to be shared by the workspace.
- Keep autopilot prompts narrow. Scheduled agents should not make irreversible production changes without a human approval path.
- If a task touches code, CI, runtime, secrets, permissions, deploys, or external writes, treat it as L2 and require stronger validation.

## 7. Recommended rollout

Start with one agent and one scheduled autopilot:

1. One teammate connects daemon and provider CLI.
2. Create one workspace-visible agent with `max-concurrent-tasks=1`.
3. Attach the Team AI SDLC skill.
4. Run one assignment-trigger smoke issue.
5. Run one comment-mention smoke.
6. Add one weekday autopilot in `create_issue` mode.
7. Review the first three closeouts before adding more agents.

Scale only after the closeouts are consistently useful to a new reader.

## 8. Troubleshooting

| Symptom | Likely cause | Check |
|---|---|---|
| Agent never starts | Daemon offline, workspace not watched, or agent has no runtime | `multica daemon status --output json`, `multica workspace list`, `multica agent get <id>` |
| Agent does not see workflow rules | Skill not attached or instructions overwritten | `multica agent skills list <agent-id>` |
| @mention does not trigger | Comment did not contain an agent mention link, agent archived, or no runtime | Use UI mention picker; check `multica agent get <id>` |
| Autopilot does not run | No enabled cron trigger or autopilot paused | `multica autopilot get <id> --output json` |
| Closeout says done but lacks proof | Skill/instructions too weak or review gate missing | Ask for layered validation and same-issue closeout |
| Agent loops with another agent | Agents keep mentioning each other in sign-off comments | Stop using @mentions for acknowledgements |

## 9. What not to sync yet

Do not promise these as generic colleague onboarding features until they are implemented and verified in the target environment:

- Webhook/API autopilot trigger endpoints.
- Provider-specific hooks that only exist in one CLI.
- Private workspace scripts, local paths, launchd jobs, machine profiles, or personal memories.
- Any rule that depends on a private Multica board, Feishu table, or internal release surface.

Keep the shared contract portable: Multica issue authority, runtime daemon, attached skill, supported trigger path, and same-target readback.
