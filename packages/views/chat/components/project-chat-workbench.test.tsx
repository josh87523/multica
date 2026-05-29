import { describe, it, expect, beforeEach, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import type { ProjectChatContext } from "@multica/core/types";

const mockApi = vi.hoisted(() => ({
  getProjectChatContext: vi.fn(),
  runProjectChatAction: vi.fn(),
  applyProjectChatAssetPatch: vi.fn(),
}));

vi.mock("@multica/core/api", () => ({
  api: mockApi,
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    projectDetail: (id: string) => `/test-ws/projects/${id}`,
  }),
}));

vi.mock("../../navigation", () => ({
  AppLink: ({
    href,
    className,
    children,
  }: {
    href: string;
    className?: string;
    children: ReactNode;
  }) => (
    <a href={href} className={className}>
      {children}
    </a>
  ),
}));

vi.mock("sonner", () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn(),
  },
}));

vi.mock("@multica/ui/components/ui/scroll-area", () => ({
  ScrollArea: ({ children, className }: { children: ReactNode; className?: string }) => (
    <div className={className}>{children}</div>
  ),
}));

import { ProjectChatWorkbench } from "./project-chat-workbench";

const baseContext: ProjectChatContext = {
  project_id: "project-1",
  project_title: "棉花兔兔 LOFTER",
  project_status: "in_progress",
  project_priority: "high",
  status_summary: "项目已有描述信息，纯聊天首版会把它作为创作背景。",
  latest_review_summary: [
    "标题质量: 标题和正文主事件基本一致",
    "AI 味风险: 低，暂未命中明显规则",
  ],
  current_draft_label: "当前以项目描述作为首版创作背景",
  next_recommended_actions: [
    "Ask: 先确认当前项目状态、资源和下一步创作建议",
    "Create: 生成标题候选、改写文本或做去 AI 味预览",
  ],
  creative_asset_snapshot: {
    style_examples: [],
    title_preferences: ["标题更克制，不要太狗血"],
    shape_preferences: ["CP 拉扯强度再收一点"],
    historical_notes: ["现代都市背景，偏轻一点"],
  },
  latest_artifacts: [
    {
      kind: "title_candidate",
      label: "lofter_title_rewrite",
      summary: "Create 标题改写",
      ref: "【朝俞】嘴上说收住了，贺朝却先乱了分寸",
      created_at: "2026-05-29T01:23:45Z",
    },
  ],
  attached_resources: [
    {
      kind: "github_repo",
      label: "LOFTER source",
      summary: "Attached project resource",
      ref: JSON.stringify({ url: "https://github.com/multica-ai/lofter-chat" }),
    },
  ],
  recent_actions: [
    {
      action_type: "create",
      normalized_payload: { mode: "rewrite_title", adapter_status: "live" },
      requires_confirmation: false,
      result_title: "Create 标题改写",
      result_summary: "已通过 LOFTER 标题改写链路生成一个更贴近当前指令的版本。",
      result_items: ["【朝俞】嘴上说收住了，贺朝却先乱了分寸"],
    },
  ],
};

function renderWorkbench() {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  return render(
    <QueryClientProvider client={client}>
      <ProjectChatWorkbench projectId="project-1" />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  mockApi.getProjectChatContext.mockReset();
  mockApi.runProjectChatAction.mockReset();
  mockApi.applyProjectChatAssetPatch.mockReset();
  mockApi.getProjectChatContext.mockResolvedValue(baseContext);
  mockApi.applyProjectChatAssetPatch.mockResolvedValue({
    updated_asset_snapshot: {
      ...baseContext.creative_asset_snapshot,
      title_preferences: [
        ...baseContext.creative_asset_snapshot.title_preferences,
        "以后标题更克制一点，不要太狗血",
      ],
    },
  });
});

describe("ProjectChatWorkbench", () => {
  it("renders project context, asset snapshot, and normalized artifact refs", async () => {
    renderWorkbench();

    await waitFor(() => {
      expect(screen.getAllByText("棉花兔兔 LOFTER").length).toBeGreaterThan(0);
    });
    expect(screen.getByText("标题更克制，不要太狗血")).toBeInTheDocument();
    expect(screen.getByText("CP 拉扯强度再收一点")).toBeInTheDocument();
    expect(screen.getByText("https://github.com/multica-ai/lofter-chat")).toBeInTheDocument();
    expect(screen.getAllByText("当前项目上下文").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Create 标题改写").length).toBeGreaterThan(0);
    expect(screen.getByText("标题质量: 标题和正文主事件基本一致")).toBeInTheDocument();
  });

  it("appends a user bubble immediately and replaces pending assistant state on success", async () => {
    mockApi.runProjectChatAction.mockResolvedValue({
      action_type: "shape",
      normalized_payload: { project_id: "project-1", asset_target: "title_preferences" },
      requires_confirmation: true,
      result_title: "Shape 补丁预览",
      result_summary: "已把你的方向性要求整理成可确认补丁。",
      result_items: ["首版先回显系统理解摘要，再等待确认"],
      asset_patch_preview: {
        asset_target: "title_preferences",
        summary: "系统理解到这是对后续创作方向的长期约束",
        patch: "以后标题更克制一点，不要太狗血",
      },
    });

    renderWorkbench();
    await waitFor(() => {
      expect(screen.getAllByText("棉花兔兔 LOFTER").length).toBeGreaterThan(0);
    });

    fireEvent.change(
      screen.getByPlaceholderText("描述你想问的状态、要调整的风格，或要生成的 LOFTER 低风险创作动作"),
      { target: { value: "以后标题更克制一点，不要太狗血" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    expect(screen.getAllByText("以后标题更克制一点，不要太狗血").length).toBeGreaterThan(1);
    expect(screen.getByText("请求处理中")).toBeInTheDocument();

    await waitFor(() => {
      expect(mockApi.runProjectChatAction).toHaveBeenCalledWith("project-1", {
        input_text: "以后标题更克制一点，不要太狗血",
        context_hint: baseContext.status_summary,
      });
      expect(screen.getByText("Shape 补丁预览")).toBeInTheDocument();
      expect(screen.getByText("系统理解到这是对后续创作方向的长期约束")).toBeInTheDocument();
    });
  });

  it("applies a shape patch after confirmation", async () => {
    mockApi.runProjectChatAction.mockResolvedValue({
      action_type: "shape",
      normalized_payload: { project_id: "project-1", asset_target: "title_preferences" },
      requires_confirmation: true,
      result_title: "Shape 补丁预览",
      result_summary: "已把你的方向性要求整理成可确认补丁。",
      result_items: ["首版先回显系统理解摘要，再等待确认"],
      asset_patch_preview: {
        asset_target: "title_preferences",
        summary: "系统理解到这是对后续创作方向的长期约束",
        patch: "以后标题更克制一点，不要太狗血",
      },
    });

    renderWorkbench();
    await waitFor(() => {
      expect(screen.getAllByText("棉花兔兔 LOFTER").length).toBeGreaterThan(0);
    });

    fireEvent.change(
      screen.getByPlaceholderText("描述你想问的状态、要调整的风格，或要生成的 LOFTER 低风险创作动作"),
      { target: { value: "以后标题更克制一点，不要太狗血" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "写入偏好" })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "写入偏好" }));

    await waitFor(() => {
      expect(mockApi.applyProjectChatAssetPatch).toHaveBeenCalledWith("project-1", {
        asset_target: "title_preferences",
        patch: "以后标题更克制一点，不要太狗血",
      });
      expect(screen.getByRole("button", { name: "已写入偏好" })).toBeDisabled();
    });
  });
});
