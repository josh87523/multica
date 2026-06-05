# 基于 Multica 的 AI 开发协作流程

这份文档面向准备把 coding agent 接入真实研发流程的团队。它不是工具说明书，而是一套协作合同：人、AI agent、代码仓库、运行时验证和项目管理状态应该如何对齐。

核心目标很简单：让 agent 像同事一样接任务、汇报、交付、复盘，同时避免把“跑过测试”“开了 PR”“合并了代码”“线上可用”“项目管理系统已同步”混成同一件事。

## 适用范围

适合：

- 用 Multica 管理 AI agent 任务的研发团队
- 多个 agent 并行处理 issue、PR、测试、文档或运维任务
- 需要把最终状态回写到项目看板、GitHub、Feishu、Linear/Jira 等系统的团队
- 对合规、上线验证、客户可见状态有明确要求的团队

不适合：

- 一次性 prompt 实验
- 没有代码仓库、没有验收标准的泛聊天任务
- 只需要个人临时自动化、不需要团队可追踪状态的任务

## 核心原则

### 1. Issue 是协作单元

每个真实任务都应该落在一个 issue 上。issue 不是聊天记录的副本，而是任务的权威入口：

- 业务问题是什么
- 期望产出是什么
- 谁负责决策
- agent 负责哪一段
- 验收标准是什么
- 最终证据写回哪里

### 2. Agent 是队友，不是后台脚本

Agent 应该有明确身份、职责和任务边界：

- 被分配任务，而不是被塞一大段无边界 prompt
- 遇到 blocker 要报告，而不是静默重试
- 交付时要给证据，而不是只说 done
- 学到的模式要沉淀成可复用 skill 或模板

### 3. 流程状态必须落盘

长任务不能依赖当前聊天上下文。以下状态应该有持久化记录：

- 需求或任务卡
- 技术方案或实现计划
- 测试计划
- 执行分片
- PR / commit / branch
- 验证证据
- closeout 回写
- 剩余 TODO

### 4. 验证分层，不互相冒充

研发闭环至少分五层。不同任务不一定全都需要，但需要哪层就必须读回哪层：

| 层 | 证明什么 | 不能替代什么 |
|---|---|---|
| 本地测试 | 当前代码在本地或 CI-like 环境可运行 | 不能证明已合并 |
| PR / CI | 代码进入主线前通过 review 和检查 | 不能证明线上已消费 |
| Release / Promote | 已合并内容被发布到实际消费面 | 不能证明用户可见状态正确 |
| Live runtime | 长期运行入口正在消费新版本 | 不能证明外部系统已同步 |
| External endstate | 用户或外部系统能读到最终状态 | 不能证明代码质量 |

### 5. Closeout 必须同目标读回

如果任务承诺“已同步到看板 / issue / 文档 / 线上系统”，必须回读同一个目标。写入成功不等于读回成功，本地缓存也不能代替远端真相。

## 推荐流程

### 0. 创建项目和角色

在 Multica 里创建一个 project，按真实业务目标命名，例如：

- `Checkout reliability`
- `Cloud publish parity`
- `Agent SDLC pilot`

然后准备三类 agent：

- **Planner**：负责读需求、拆任务、识别风险
- **Implementer**：负责代码或文档改动
- **Reviewer / Verifier**：负责 review、测试和证据读回

小团队可以先只配一个实现 agent，但 review 和最终验收仍应由人或另一个上下文完成。

### 1. Intake：需求卡

每个 project 下先有一张需求卡。最低字段：

```md
## Problem
用户或团队遇到的真实问题是什么？

## Outcome
完成后用户可见的变化是什么？

## Scope
这次做什么？

## Non-goals
这次明确不做什么？

## Authority surfaces
- Source repo:
- PR / CI:
- Release / runtime:
- External writeback:

## Acceptance
- [ ] 可观察验收点 1
- [ ] 可观察验收点 2
```

### 2. Plan：方案和测试先行

进入实现前，先让 agent 输出两份材料：

- **Implementation plan**：改哪些模块、为什么这么改、风险在哪里
- **Test plan**：哪些行为要测、哪些层级需要读回、哪些不适用

不要让 agent 先写代码再倒推测试口径。测试计划应该在实现前出现。

### 3. Slice：拆成可执行分片

把大任务拆成小 issue 或子 issue。每个分片都应该能回答：

- 输入材料在哪里
- 允许改哪些路径
- 不允许碰哪些路径
- 用哪个 branch / worktree
- 完成后跑哪些检查
- 证据写回哪张卡

推荐分片大小：一个 agent 能在一个上下文窗口内理解、修改、验证、报告。

### 4. Execution：认领和执行

Agent 认领任务后，应在 issue 评论里写清：

- 本轮要做的 slice
- 当前假设
- 将使用的验证路径
- 预计产物

执行中遇到 blocker 时，不要把任务标 done。应写：

- blocker 是什么
- 已经验证了什么
- 需要人类决策、权限、环境还是产品取舍

### 5. Review：不要让同一个上下文自证完成

至少保留一个 review 面：

- 人类 review PR
- 另一个 agent 做 fresh-context review
- reviewer 只读方案、diff、测试结果，不继承 implementer 的乐观假设

Review 重点不是语法，而是：

- 是否真的解决用户问题
- 是否改错层
- 是否遗漏权限、数据、安全、迁移、运行时影响
- 是否有测试和读回证据

### 6. Validation：按任务影响选择最小充分验证

建议把任务分三档：

| 档位 | 类型 | 验证要求 |
|---|---|---|
| L0 | 只读分析、评论、一次性回答 | 不创建 PR，不跑重验证 |
| L1 | 普通文档、方案、外部写回 | 做链接/格式/敏感信息检查，验证外部终态 |
| L2 | 代码、脚本、CI、运行时、权限、规则文档 | 本地测试 + PR/CI + 必要的 runtime/external readback |

如果不涉及运行时或外部系统，要明确写 `online regression: not applicable`，不要为了显得完整而伪造线上验证。

### 7. Closeout：固定结构回写

任务完成后，在同一个 issue 写 closeout。推荐结构：

```md
## 完成状态
Done / Partial / Blocked

## 问题
本轮要解决的用户或团队问题。

## 根因
为什么之前会出错或缺失。区分需求不清、实现缺口、验证缺口、运行时缺口、回写缺口。

## 解决方案
本轮改了什么，为什么这样改。

## 关键验证结论
- Source:
- PR / CI:
- Release / runtime:
- External readback:

## 剩余 blocker / TODO
没有就写 None。

## 下一步
如果还有 TODO，给出唯一下一步。
```

正式 closeout 不要只写“已完成”。它应该让新同事不看聊天记录也能理解发生了什么。

### 8. Learn：沉淀成团队能力

当某个问题会再次出现时，才进入学习闭环：

- 写成 skill：agent 下次可以直接复用
- 写成模板：需求、方案、测试、closeout 可以复用
- 写成 guardrail：能机械检查的就交给脚本或 CI
- 写成 runbook：需要人类判断的操作保留步骤和风险

学习材料也要有消费面：只是写在某个聊天记录里，不算沉淀。

## Multica 看板约定

推荐列：

- **Intake**：需求刚进入，未确认验收
- **Plan**：正在写方案或测试计划
- **Ready**：可被 agent 认领
- **In Progress**：agent 或人正在执行
- **Review**：等待 review
- **Verify**：等待测试、runtime 或 external readback
- **Done**：已 closeout 并同目标读回
- **Blocked**：需要人类决策或外部条件

推荐标签：

- `L0-readonly`
- `L1-docs-writeback`
- `L2-code-runtime`
- `needs-human-decision`
- `needs-external-readback`
- `runtime-impact`
- `security-risk`

推荐 project 组织方式：

- Project 表示业务目标或流程改造目标
- Issue 表示可验收的工作单元
- Sub-issue 表示分片或验收卡
- Comment 表示执行 receipt 和 closeout

## Definition of Done

一个 AI 协作任务只有在下面条件满足时才算完成：

- 问题和目标在 issue 上可读
- 实现或文档产物已落到目标仓库
- 对应验证层已经完成，或明确不适用
- PR / commit / release / runtime / external readback 分层清楚
- Closeout 写回同一个目标 issue
- 剩余 TODO 明确为 None，或给出唯一下一步

## 7 天试点建议

第 1 天：

- 选一个低风险真实项目
- 建一个 Multica project
- 写一张需求卡和 3 到 5 个小 issue

第 2 到 3 天：

- 让 agent 做 L1 文档或测试补充任务
- 强制 closeout 模板
- 不允许只说 done

第 4 到 5 天：

- 让 agent 做一个小型 L2 代码任务
- 要求本地测试、PR/CI、必要读回分层

第 6 天：

- 做 fresh-context review
- 看 closeout 是否能让新同事独立恢复上下文

第 7 天：

- 把重复问题沉淀成模板、skill 或 checklist
- 决定哪些流程应该变成硬 gate，哪些保持人工判断

## 常见误区

### 把 agent 当成脚本

脚本只需要输入输出。Agent 需要任务边界、上下文、权限、验收和反馈通道。

### 把测试绿当成完成

测试绿只证明当前测试覆盖的行为成立。它不能证明 PR 已合并、线上已消费、外部系统已同步。

### 把 closeout 写到新卡

Closeout 应写回原需求或原任务卡。新建总结卡会让后续追溯断链。

### 让同一个 agent 自己计划、实现、review、验收

这会放大原始假设错误。至少让 review 从 fresh context 开始。

### 把内部运行状态发给外部同事

公开分享流程时，不要包含 token、机器名、内部路径、远端 IP、私有仓库名、客户数据或未脱敏日志。

## 最小模板索引

团队可以从四个模板开始：

- `需求卡`：Problem / Outcome / Scope / Non-goals / Authority surfaces / Acceptance
- `任务分片`：Inputs / Allowed paths / Forbidden paths / Branch / Checks / Writeback target
- `验证报告`：Local / PR-CI / Release / Runtime / External endstate / Gaps
- `Closeout`：完成状态 / 问题 / 根因 / 解决方案 / 关键验证结论 / 剩余 TODO / 下一步

先把这四个模板跑顺，再考虑更复杂的调度、预算、自动分派和多 agent 编排。
