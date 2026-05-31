import type { Issue } from "@multica/core/types";
import type { SortDirection, SortField } from "@multica/core/issues/stores/view-store";

export interface IssueDateGroup {
  key: string;
  issues: Issue[];
}

export function buildIssueDateGroups(
  issues: Issue[],
  sortBy: SortField,
): IssueDateGroup[] | null {
  if (sortBy !== "due_date" && sortBy !== "created_at") return null;
  const groups = new Map<string, Issue[]>();
  for (const issue of issues) {
    const rawValue = sortBy === "due_date" ? issue.due_date : issue.created_at;
    const key = rawValue ? rawValue.slice(0, 10) : "__none__";
    const existing = groups.get(key);
    if (existing) {
      existing.push(issue);
    } else {
      groups.set(key, [issue]);
    }
  }
  return Array.from(groups, ([key, groupedIssues]) => ({ key, issues: groupedIssues }));
}

export function formatIssueDateGroupLabel(params: {
  key: string;
  sortBy: Extract<SortField, "due_date" | "created_at">;
  t: (selector: any, options?: Record<string, unknown>) => unknown;
  locale: string;
  now?: Date;
}): string {
  const { key, sortBy, t, locale, now = new Date() } = params;
  if (key === "__none__") return String(t(($: any) => $.list.group_no_due_date));

  const target = new Date(`${key}T00:00:00`);
  const today = startOfDay(now);
  const diffDays = Math.round((target.getTime() - today.getTime()) / DAY_MS);

  if (diffDays === 0) {
    return sortBy === "due_date"
      ? String(t(($: any) => $.list.group_due_today))
      : String(t(($: any) => $.list.group_created_today));
  }
  if (diffDays === -1) return String(t(($: any) => $.list.group_yesterday));
  if (diffDays === 1 && sortBy === "due_date") return String(t(($: any) => $.list.group_due_tomorrow));

  return target.toLocaleDateString(toLocaleTag(locale), {
    month: "short",
    day: "numeric",
    weekday: "short",
    ...(target.getFullYear() !== today.getFullYear() ? { year: "numeric" } : {}),
  });
}

export function shouldUseDateGrouping(sortBy: SortField): sortBy is "due_date" | "created_at" {
  return sortBy === "due_date" || sortBy === "created_at";
}

export function sortDateGroupItems(
  groups: IssueDateGroup[],
  direction: SortDirection,
): IssueDateGroup[] {
  if (groups.length <= 1) return groups;
  return [...groups].sort((a, b) => compareGroupKeys(a.key, b.key, direction));
}

const DAY_MS = 24 * 60 * 60 * 1000;

function startOfDay(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate());
}

function toLocaleTag(locale: string): string {
  return locale === "zh-Hans" ? "zh-CN" : locale;
}

function compareGroupKeys(a: string, b: string, direction: SortDirection): number {
  if (a === b) return 0;
  if (a === "__none__") return direction === "asc" ? 1 : -1;
  if (b === "__none__") return direction === "asc" ? -1 : 1;
  return direction === "asc" ? a.localeCompare(b) : b.localeCompare(a);
}
