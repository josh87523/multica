import { describe, expect, it } from "vitest";
import { ACTIVE_BOARD_STATUSES, ALL_STATUSES, BOARD_STATUSES, STATUS_ORDER } from "./status";

describe("issue status board order", () => {
  it("uses the same actionable-first order across status lists and board columns", () => {
    const workflowStatuses = [
      "todo",
      "backlog",
      "in_progress",
      "in_review",
      "review",
      "done",
      "blocked",
    ];

    expect(STATUS_ORDER).toEqual([...workflowStatuses, "cancelled"]);
    expect(ALL_STATUSES).toEqual([...workflowStatuses, "cancelled"]);
    expect(BOARD_STATUSES).toEqual(workflowStatuses);
    expect(ACTIVE_BOARD_STATUSES).toEqual(workflowStatuses);
  });
});
