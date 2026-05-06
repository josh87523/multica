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
    expect(issueCardDescription({ ...baseIssue, description })).toBe("PR 已合并，但远程任务分支仍存在。");
  });

  it("falls back to the raw description for ordinary issues", () => {
    expect(issueCardDescription({ ...baseIssue, description: "Add JWT authentication" })).toBe("Add JWT authentication");
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
});
