import { describe, expect, it } from "vitest";
import type { Issue } from "../types";
import { extractMarkdownSection, issueCardDescription, issueDisplayTitle } from "./business-summary";

const baseIssue: Issue = {
  id: "issue-1",
  workspace_id: "workspace-1",
  number: 1,
  identifier: "MUL-1",
  title: "Issue",
  description: null,
  status: "todo",
  priority: "none",
  assignee_type: null,
  assignee_id: null,
  creator_type: "member",
  creator_id: "user-1",
  parent_issue_id: null,
  project_id: null,
  position: 0,
  due_date: null,
  created_at: "2026-05-06T00:00:00Z",
  updated_at: "2026-05-06T00:00:00Z",
};

describe("issue business summary", () => {
  it("extracts a readable problem section from Workspace sync markdown", () => {
    const description = [
      "<!-- workspace-source-id: ledger:task-1 -->",
      "",
      "## 问题",
      "PR 已合并，但远程任务分支仍存在。",
      "",
      "## 原因",
      "闭环检查没有通过。",
      "",
      "## 处理方案",
      "删除远程任务分支后重新运行 finisher。",
    ].join("\n");

    expect(extractMarkdownSection(description, "问题")).toBe("PR 已合并，但远程任务分支仍存在。");
    expect(issueCardDescription({ ...baseIssue, description })).toBe("处理方案：删除远程任务分支后重新运行 finisher。");
  });

  it("prefers the problem when workspace-sync reason and solution are generic boilerplate", () => {
    const description = [
      "## 问题",
      "T-093 含蓄科技 LOFTER 自动化资产 private 但仍有 live operational risk，缺 owner/patch/secret/runtime 明细",
      "",
      "## 原因",
      "这条记录来自人工维护的 TODO / backlog 文档，属于「小需求」。它代表 IndustryInsights 当前仍未闭环的业务、产品或交付缺口。",
      "",
      "## 处理方案",
      "原始记录没有结构化方案时，先按这条问题补齐负责人、验收标准和下一步；已有方案则按原文推进，并在完成后关闭对应 TODO。",
    ].join("\n");

    expect(issueCardDescription({ ...baseIssue, description })).toBe(
      "T-093 含蓄科技 LOFTER 自动化资产 private 但仍有 live operational risk，缺 owner/patch/secret/runtime 明细",
    );
  });

  it("falls back to the raw description for ordinary issues", () => {
    expect(issueCardDescription({ ...baseIssue, description: "Add JWT authentication" })).toBe("Add JWT authentication");
  });

  it("cleans markdown metadata when ordinary issues lack business sections", () => {
    const description = [
      "<!-- workspace-source-id: ledger:task-1 -->",
      "",
      "## 来源信息",
      "- workspace-source-id: ledger:task-1",
      "- 阶段：Review",
      "",
      "需要业务确认这条任务是否仍然有效。",
    ].join("\n");

    expect(issueCardDescription({ ...baseIssue, description })).toBe("需要业务确认这条任务是否仍然有效。");
  });

  it("derives a decision-oriented title for read-only Workspace ledger issues", () => {
    const description = [
      "<!-- workspace-source-id: ledger-milestone:/tmp/task -->",
      "",
      "## 问题",
      "线上真实运行目录还没有回归。",
      "",
      "## 原因",
      "闭环检查没有通过。",
      "",
      "## 处理方案",
      "补跑 live-path regression。",
    ].join("\n");

    expect(
      issueDisplayTitle({
        ...baseIssue,
        title: "AI 开发闭环存在遗留问题：共享上下文",
        description,
        workspace_control: {
          source_type: "ledger-milestone",
          source_id: "ledger-milestone:/tmp/task",
          writable: false,
        },
      }),
    ).toBe("闭环缺口：线上真实运行目录还没有回归。");
  });

  it("uses source-type fallback titles even when legacy titles do not match regexes", () => {
    const description = [
      "## 问题",
      "已有军团任务只有技术 ID，没有说明业务目标。",
      "",
      "## 处理方案",
      "补充用户目标和验收条件。",
    ].join("\n");

    expect(
      issueDisplayTitle({
        ...baseIssue,
        title: "old imported item",
        description,
        workspace_control: {
          source_type: "legion",
          source_id: "legion:task-1",
          writable: false,
        },
      }),
    ).toBe("补齐军团任务业务目标：已有军团任务只有技术 ID，没有说明业务目标。");
  });

  it("clips derived titles on a semantic boundary", () => {
    const longProblem = "这条自动化已经连续多次执行，但用户看不到它对应的业务目标，也不知道失败后应该判断什么。" + "需要补充上下文。".repeat(20);

    const title = issueDisplayTitle({
      ...baseIssue,
      title: "cron task",
      description: `## 问题\n${longProblem}`,
      workspace_control: {
        source_type: "cron",
        source_id: "cron:task",
        writable: false,
      },
    });

    expect(title.length).toBeLessThanOrEqual(180);
    expect(title.endsWith("...")).toBe(true);
  });
});
