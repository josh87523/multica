import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Issue } from "@multica/core/types";
import { I18nProvider } from "@multica/core/i18n/react";
import enLayout from "../../locales/en/layout.json";
import enMyIssues from "../../locales/en/my-issues.json";

const TEST_RESOURCES = { en: { layout: enLayout, "my-issues": enMyIssues } };

const mockListIssues = vi.hoisted(() => vi.fn());
const mockListAgents = vi.hoisted(() => vi.fn());

vi.mock("@multica/core/api", () => ({
  api: {
    listIssues: (...args: unknown[]) => mockListIssues(...args),
    listAgents: (...args: unknown[]) => mockListAgents(...args),
    getChildIssueProgress: () => Promise.resolve({ progress: [] }),
    updateIssue: vi.fn(),
  },
}));

vi.mock("@multica/core/auth", () => ({
  useAuthStore: (selector?: any) => {
    const state = {
      user: { id: "user-1", email: "test@example.com", name: "Test User" },
      isAuthenticated: true,
    };
    return selector ? selector(state) : state;
  },
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/paths", () => ({
  useCurrentWorkspace: () => ({ id: "ws-1", name: "Test WS", slug: "test" }),
}));

vi.mock("../../workspace/workspace-avatar", () => ({
  WorkspaceAvatar: ({ name }: { name: string }) => (
    <span data-testid="workspace-avatar">{name.charAt(0)}</span>
  ),
}));

vi.mock("./my-issues-header", () => ({
  MyIssuesHeader: () => <div data-testid="my-issues-header" />,
}));

vi.mock("../../issues/components/board-view", () => ({
  BoardView: ({ visibleStatuses, hiddenStatuses }: any) => (
    <div>
      <div data-testid="board-visible">{visibleStatuses.join(",")}</div>
      <div data-testid="board-hidden">{hiddenStatuses.join(",")}</div>
    </div>
  ),
}));

vi.mock("../../issues/components/list-view", () => ({
  ListView: () => <div data-testid="list-view" />,
}));

vi.mock("../../issues/components/batch-action-toolbar", () => ({
  BatchActionToolbar: () => <div data-testid="batch-toolbar" />,
}));

vi.mock("sonner", () => ({
  toast: { error: vi.fn() },
}));

const baseIssue: Issue = {
  id: "issue-1",
  workspace_id: "ws-1",
  number: 1,
  identifier: "CAM-1",
  title: "First assigned issue",
  description: null,
  status: "todo",
  priority: "none",
  assignee_type: "member",
  assignee_id: "user-1",
  creator_type: "member",
  creator_id: "user-1",
  parent_issue_id: null,
  project_id: null,
  position: 0,
  due_date: null,
  created_at: "2026-06-06T00:00:00Z",
  updated_at: "2026-06-06T00:00:00Z",
};

import { myIssuesViewStore } from "@multica/core/issues/stores/my-issues-view-store";
import { MyIssuesPage } from "./my-issues-page";

function renderWithQuery(ui: React.ReactElement) {
  const qc = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });
  return render(
    <I18nProvider locale="en" resources={TEST_RESOURCES}>
      <QueryClientProvider client={qc}>{ui}</QueryClientProvider>
    </I18nProvider>,
  );
}

describe("MyIssuesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    myIssuesViewStore.setState({
      viewMode: "board",
      statusFilters: [],
      priorityFilters: [],
      scope: "assigned",
    });
    mockListAgents.mockResolvedValue([]);
    mockListIssues.mockImplementation((params: { status?: string }) =>
      Promise.resolve({
        issues: params.status === "todo" ? [baseIssue] : [],
        total: params.status === "todo" ? 1 : 0,
      }),
    );
  });

  it("starts the default board with Not started before Design", async () => {
    renderWithQuery(<MyIssuesPage />);

    await expect.poll(() => screen.getByTestId("board-visible").textContent).toBe(
      "todo,backlog,in_progress,in_review,review,done,blocked",
    );
    expect(screen.getByTestId("board-hidden")).toHaveTextContent("");
  });
});
