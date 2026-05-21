import { describe, expect, it } from "vitest";
import { computeManualPosition, sortIssues } from "./sort";
import type { Issue } from "@multica/core/types";

function issue(id: string, position: number): Issue {
  const now = "2026-05-21T00:00:00.000Z";
  return {
    id,
    workspace_id: "ws-1",
    number: Number.parseInt(id.replace(/\D/g, ""), 10) || 1,
    identifier: `MUL-${id}`,
    title: id,
    description: null,
    status: "todo",
    priority: "medium",
    position,
    creator_id: "user-1",
    creator_type: "member",
    parent_issue_id: null,
    assignee_id: null,
    assignee_type: null,
    project_id: null,
    due_date: null,
    created_at: now,
    updated_at: now,
    labels: [],
  };
}

describe("sortIssues", () => {
  it("sorts manual order ascending", () => {
    const issues = [issue("b", 2), issue("a", 1), issue("c", 3)];
    expect(sortIssues(issues, "position", "asc").map((item) => item.id)).toEqual(["a", "b", "c"]);
  });

  it("sorts manual order descending", () => {
    const issues = [issue("b", 2), issue("a", 1), issue("c", 3)];
    expect(sortIssues(issues, "position", "desc").map((item) => item.id)).toEqual(["c", "b", "a"]);
  });
});

describe("computeManualPosition", () => {
  const map = new Map<string, Issue>([
    ["a", issue("a", 10)],
    ["b", issue("b", 20)],
    ["c", issue("c", 30)],
  ]);

  it("places the top card before the next card in ascending mode", () => {
    expect(computeManualPosition(["a", "b", "c"], "a", map, "asc")).toBe(19);
  });

  it("places the top card after the next card in descending mode", () => {
    expect(computeManualPosition(["a", "b", "c"], "a", map, "desc")).toBe(21);
  });

  it("places the bottom card before the previous card in descending mode", () => {
    expect(computeManualPosition(["a", "b", "c"], "c", map, "desc")).toBe(19);
  });
});
