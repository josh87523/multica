import type { Issue } from "../types";

const SECTION_RE = /^##\s+(.+?)\s*$/gm;

export interface IssueBusinessSummary {
  problem: string;
  reason: string;
  solution: string;
}

export function extractMarkdownSection(markdown: string | null | undefined, heading: string): string {
  if (!markdown) return "";

  const sections = Array.from(markdown.matchAll(SECTION_RE));
  for (let index = 0; index < sections.length; index += 1) {
    const match = sections[index]!;
    if (match[1]?.trim() !== heading) continue;

    const start = (match.index ?? 0) + match[0].length;
    const next = sections[index + 1];
    const end = next?.index ?? markdown.length;
    return markdown
      .slice(start, end)
      .replace(/<!--[\s\S]*?-->/g, "")
      .split("\n")
      .map((line) => line.trim())
      .filter(Boolean)
      .join(" ");
  }

  return "";
}

export function issueBusinessSummary(issue: Issue): IssueBusinessSummary {
  return {
    problem: extractMarkdownSection(issue.description, "问题"),
    reason: extractMarkdownSection(issue.description, "原因"),
    solution: extractMarkdownSection(issue.description, "处理方案"),
  };
}

export function issueCardDescription(issue: Issue): string {
  const summary = issueBusinessSummary(issue);
  return summary.problem || issue.description || "";
}

export function issueDisplayTitle(issue: Issue): string {
  const problem = issueBusinessSummary(issue).problem;
  if (!problem || !issue.workspace_control?.source_type) return issue.title;

  const sourceType = issue.workspace_control.source_type;
  if ((sourceType === "ledger" || sourceType === "ledger-milestone") && /闭环|遗留|里程碑|执行记录/.test(issue.title)) {
    return `闭环缺口：${problem}`;
  }
  if (sourceType === "legion" && /军团任务|task-[0-9a-f]/i.test(issue.title)) {
    return `补齐军团任务业务目标：${problem}`;
  }
  if ((sourceType === "launchd" || sourceType === "cron") && /定时|自动化|launchd|cron/i.test(issue.title)) {
    return `确认定时自动化仍符合业务预期：${problem}`;
  }

  return issue.title;
}
