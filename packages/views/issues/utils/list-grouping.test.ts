import { describe, expect, it } from "vitest";
import type { Issue } from "@multica/core/types";
import { buildIssueDateGroups, formatIssueDateGroupLabel, sortDateGroupItems } from "./list-grouping";

function issue(id: string, overrides: Partial<Issue> = {}): Issue {
  return {
    id,
    workspace_id: "ws-1",
    number: 1,
    identifier: `MUL-${id}`,
    title: id,
    description: null,
    status: "todo",
    priority: "medium",
    assignee_type: null,
    assignee_id: null,
    creator_type: "member",
    creator_id: "user-1",
    parent_issue_id: null,
    project_id: null,
    position: 1,
    due_date: null,
    labels: [],
    created_at: "2026-05-21T09:00:00Z",
    updated_at: "2026-05-21T09:00:00Z",
    ...overrides,
  };
}

describe("buildIssueDateGroups", () => {
  it("groups due dates by calendar day and keeps missing dates separate", () => {
    const groups = buildIssueDateGroups(
      [
        issue("a", { due_date: "2026-05-21T03:00:00Z" }),
        issue("b", { due_date: "2026-05-21T18:00:00Z" }),
        issue("c", { due_date: null }),
      ],
      "due_date",
    );

    expect(groups).toEqual([
      { key: "2026-05-21", issues: expect.arrayContaining([expect.objectContaining({ id: "a" }), expect.objectContaining({ id: "b" })]) },
      { key: "__none__", issues: [expect.objectContaining({ id: "c" })] },
    ]);
  });

  it("returns null for non-date sort fields", () => {
    expect(buildIssueDateGroups([issue("a")], "priority")).toBeNull();
  });
});

describe("sortDateGroupItems", () => {
  const groups = [
    { key: "__none__", issues: [] },
    { key: "2026-05-20", issues: [] },
    { key: "2026-05-21", issues: [] },
  ];

  it("keeps no-date bucket last in ascending order", () => {
    expect(sortDateGroupItems(groups, "asc").map((group) => group.key)).toEqual([
      "2026-05-20",
      "2026-05-21",
      "__none__",
    ]);
  });

  it("keeps no-date bucket first in descending order", () => {
    expect(sortDateGroupItems(groups, "desc").map((group) => group.key)).toEqual([
      "__none__",
      "2026-05-21",
      "2026-05-20",
    ]);
  });
});

describe("formatIssueDateGroupLabel", () => {
  const t = (selector: (data: any) => string) =>
    selector({
      list: {
        group_due_today: "Due today",
        group_due_tomorrow: "Due tomorrow",
        group_created_today: "Created today",
        group_yesterday: "Yesterday",
        group_no_due_date: "No due date",
      },
    });

  it("renders relative labels for due-date buckets", () => {
    expect(
      formatIssueDateGroupLabel({
        key: "2026-05-21",
        sortBy: "due_date",
        t,
        locale: "en",
        now: new Date("2026-05-21T10:00:00Z"),
      }),
    ).toBe("Due today");
  });

  it("renders no-due-date label", () => {
    expect(
      formatIssueDateGroupLabel({
        key: "__none__",
        sortBy: "due_date",
        t,
        locale: "en",
        now: new Date("2026-05-21T10:00:00Z"),
      }),
    ).toBe("No due date");
  });
});
