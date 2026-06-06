import { describe, expect, it } from "vitest";
import { BOARD_STATUSES, MY_ISSUES_BOARD_STATUSES } from "./status";

describe("issue status board order", () => {
  it("keeps workspace boards planning-first while My Issues starts with actionable work", () => {
    expect(BOARD_STATUSES[0]).toBe("backlog");
    expect(MY_ISSUES_BOARD_STATUSES).toEqual([
      "todo",
      "backlog",
      "in_progress",
      "in_review",
      "review",
      "done",
      "blocked",
    ]);
  });
});
