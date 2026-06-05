# Team AI SDLC

Use this skill when you are assigned a real software task through Multica.

## Operating Contract

1. Treat the Multica issue as the task authority.
2. Restate the current slice before doing work.
3. Separate source changes, PR/CI, runtime proof, and external writeback.
4. Do not report "done" until the required validation layers are complete or explicitly marked not applicable.
5. If blocked, report the blocker type: product decision, permission, environment, data, runtime, external system, or unclear requirement.
6. Write final results back to the same Multica issue.

## Before Implementation

Confirm these fields from the issue or comments:

- Problem
- Expected outcome
- Scope
- Non-goals
- Repository or artifact target
- Required validation layers
- External writeback target

If a field is missing and the task is high risk, ask for that missing decision before making irreversible changes.

## During Work

Keep work bounded:

- Use the assigned repository and branch/worktree.
- Avoid unrelated refactors.
- Keep secrets out of comments, commits, logs, and screenshots.
- Prefer structured commands and stable APIs over UI automation unless the UI behavior is the product contract.
- If a task asks you to continue prior work, read the latest issue comments and durable handoff before assuming the previous state.

## Validation Layers

Report validation by layer:

- Local: tests, lint, build, document/link check, or smoke command.
- PR / CI: PR URL, checks, review state, or not applicable.
- Release / runtime: deployed/promoted version or not applicable.
- Live behavior: runtime readback or not applicable.
- External endstate: same-target readback in Multica, GitHub, Linear, Jira, Feishu, or the target product.

Never use one layer as proof for another.

## Closeout Template

Post the final result to the same issue:

```md
## Status
Done / Partial / Blocked

## Problem
What user or team problem this task addressed.

## Root Cause
Why the issue existed or why the workflow failed before.

## Solution
What changed and why.

## Validation
- Local:
- PR / CI:
- Release / runtime:
- Live behavior:
- External endstate:

## Remaining TODO
None, or the exact remaining item.

## Next Step
Only include this if a human or next session has to act.
```

## Auto-Trigger Hygiene

When you are triggered by an issue assignment, comment mention, chat, rerun, or autopilot:

- Identify which trigger created the task.
- For comment-triggered work, respond to the actual triggering comment.
- If another agent mentioned you only to acknowledge or sign off, do not create a reply loop.
- For autopilot-created work, keep the generated issue as the audit trail and close it out like any other issue.
- For scheduled work, include the schedule name or autopilot title in the closeout.

## Done Means

The task is done only when:

- The requested artifact exists in the target place.
- The required validation layers are complete.
- The same issue has a closeout comment.
- Remaining TODO is either `None` or a single explicit next step.
