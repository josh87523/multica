import type { Issue } from "../types";

const SECTION_RE = /^##\s+(.+?)\s*$/gm;
const TITLE_LIMIT = 180;

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

function clampTitle(value: string): string {
  const text = value.trim();
  if (text.length <= TITLE_LIMIT) return text;

  const clipped = text.slice(0, TITLE_LIMIT - 3);
  const boundary = Math.max(
    clipped.lastIndexOf("。"),
    clipped.lastIndexOf("，"),
    clipped.lastIndexOf("、"),
    clipped.lastIndexOf(" "),
  );
  return `${clipped.slice(0, boundary > 60 ? boundary : clipped.length).trim()}...`;
}

function cleanMarkdownPreview(markdown: string | null | undefined): string {
  if (!markdown) return "";
  return markdown
    .replace(/<!--[\s\S]*?-->/g, "")
    .replace(/```[\s\S]*?```/g, "")
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line && !line.startsWith("#") && !/^[-*]\s*(workspace-source-id|source|来源|阶段)[:：]/.test(line))
    .join(" ")
    .trim();
}

export function issueCardDescription(issue: Issue): string {
  const summary = issueBusinessSummary(issue);
  if (summary.solution) return `处理方案：${summary.solution}`;
  if (summary.reason) return `原因：${summary.reason}`;
  if (summary.problem) return summary.problem;
  return cleanMarkdownPreview(issue.description);
}

export function issueDisplayTitle(issue: Issue): string {
  const problem = issueBusinessSummary(issue).problem;
  if (!problem || !issue.workspace_control?.source_type) return issue.title;

  const sourceType = issue.workspace_control.source_type;
  if ((sourceType === "ledger" || sourceType === "ledger-milestone") && /闭环|遗留|里程碑|执行记录/.test(issue.title)) {
    return clampTitle(`闭环缺口：${problem}`);
  }
  if (sourceType === "legion" && /军团任务|task-[0-9a-f]/i.test(issue.title)) {
    return clampTitle(`补齐军团任务业务目标：${problem}`);
  }
  if ((sourceType === "launchd" || sourceType === "cron") && /定时|自动化|launchd|cron/i.test(issue.title)) {
    return clampTitle(`确认定时自动化仍符合业务预期：${problem}`);
  }

  if (sourceType === "ledger" || sourceType === "ledger-milestone") return clampTitle(`闭环缺口：${problem}`);
  if (sourceType === "legion") return clampTitle(`补齐军团任务业务目标：${problem}`);
  if (sourceType === "launchd" || sourceType === "cron") return clampTitle(`确认定时自动化仍符合业务预期：${problem}`);
  return clampTitle(`待判断业务结果：${problem}`);
}
