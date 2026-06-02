import type { Issue } from "@multica/core/types";

const ORCHESTRATION_RE = /<!--\s*multica-orchestration:\s*(\{[\s\S]*?\})\s*-->/;

interface IssueOrchestrationState {
  stage?: string;
  status?: string;
}

function parseIssueOrchestrationState(description: string | null | undefined): IssueOrchestrationState | null {
  if (!description) return null;
  const match = description.match(ORCHESTRATION_RE);
  if (!match) return null;
  try {
    const parsed = JSON.parse(match[1] ?? "");
    return parsed && typeof parsed === "object" ? parsed as IssueOrchestrationState : null;
  } catch {
    return null;
  }
}

export function orchestrationBadgeLabel(issue: Issue): string | null {
  const state = parseIssueOrchestrationState(issue.description);
  if (!state) return null;
  const stage = state.stage && state.stage !== "idle" ? state.stage : "queued";
  const status = state.status ?? "pending";
  if (status === "awaiting_human") return `${stage} · human gate`;
  if (status === "failed") return `${stage} · failed`;
  if (status === "completed" && state.stage === "done") return "orchestrated";
  return `${stage} · ${status}`;
}
