"use client";

import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, CheckCircle2, FileText, MessageSquareText, Send, ShieldAlert, Sparkles } from "lucide-react";
import { toast } from "sonner";
import { api } from "@multica/core/api";
import { chatKeys, projectChatContextOptions } from "@multica/core/chat/queries";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import type {
  ProjectChatActionResponse,
  ProjectChatArtifact,
  ProjectChatAssetPatchPreview,
  ProjectChatContext,
} from "@multica/core/types";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button, buttonVariants } from "@multica/ui/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@multica/ui/components/ui/card";
import { ScrollArea } from "@multica/ui/components/ui/scroll-area";
import { Separator } from "@multica/ui/components/ui/separator";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { cn } from "@multica/ui/lib/utils";
import { AppLink } from "../../navigation";
import { PageHeader } from "../../layout/page-header";

type TimelineEntry =
  | { id: string; role: "user"; text: string }
  | {
      id: string;
      role: "assistant";
      text: string;
      action: ProjectChatActionResponse;
      pending?: boolean;
    };

export function ProjectChatWorkbench({ projectId }: { projectId: string }) {
  const wsId = useWorkspaceId();
  const wsPaths = useWorkspacePaths();
  const queryClient = useQueryClient();
  const { data: context, isLoading } = useQuery(
    projectChatContextOptions(wsId, projectId),
  );
  const [draft, setDraft] = useState("");
  const [timeline, setTimeline] = useState<TimelineEntry[]>([]);
  const quickPrompts = useMemo(
    () => buildQuickPrompts(context),
    [context],
  );
  const snapshotItems = useMemo(() => {
    if (!context) return [];
    return [
      ...context.creative_asset_snapshot.title_preferences.map((item) => ({
        group: "标题偏好",
        value: item,
      })),
      ...context.creative_asset_snapshot.shape_preferences.map((item) => ({
        group: "方向偏好",
        value: item,
      })),
      ...context.creative_asset_snapshot.style_examples.map((item) => ({
        group: "风格样本",
        value: item,
      })),
      ...context.creative_asset_snapshot.historical_notes.map((item) => ({
        group: "历史备注",
        value: item,
      })),
    ];
  }, [context]);

  useEffect(() => {
    if (!context) return;
    setTimeline((prev) => {
      if (prev.length > 0) return prev;
      return [
        {
          id: "starter",
          role: "assistant",
          text: context.status_summary,
          action: {
            action_type: "ask",
            normalized_payload: { project_id: context.project_id, source: "starter" },
            requires_confirmation: false,
            result_title: "当前项目上下文",
            result_summary: context.status_summary,
            result_items: context.next_recommended_actions,
          },
        },
      ];
    });
  }, [context]);

  const actionMutation = useMutation({
    mutationFn: async ({
      inputText,
    }: {
      inputText: string;
      pendingId: string;
    }) =>
      api.runProjectChatAction(projectId, {
        input_text: inputText,
        context_hint: context?.status_summary,
      }),
    onSuccess: (action, variables) => {
      setTimeline((prev) =>
        prev.map((entry) =>
          entry.id === variables.pendingId && entry.role === "assistant"
            ? {
                id: entry.id,
                role: "assistant",
                text: action.result_summary,
                action,
              }
            : entry,
        ),
      );
    },
    onError: (error, variables) => {
      const message = error instanceof Error ? error.message : "提交失败";
      setTimeline((prev) =>
        prev.map((entry) =>
          entry.id === variables.pendingId && entry.role === "assistant"
            ? {
                id: entry.id,
                role: "assistant",
                text: message,
                action: {
                  action_type: "ask",
                  normalized_payload: {
                    project_id: projectId,
                    failed_input: variables.inputText,
                  },
                  requires_confirmation: false,
                  result_title: "请求失败",
                  result_summary: message,
                  result_items: [
                    "聊天页保留了你的输入，问题通常在接口或网络层",
                    "可以直接重试，或改成更短的 Ask/Shape/Create 请求定位问题",
                  ],
                },
              }
            : entry,
        ),
      );
      toast.error(message);
    },
  });

  const applyPatchMutation = useMutation({
    mutationFn: async ({
      preview,
    }: {
      entryId: string;
      preview: ProjectChatAssetPatchPreview;
    }) =>
      api.applyProjectChatAssetPatch(projectId, {
        asset_target: preview.asset_target,
        patch: preview.patch,
      }),
    onSuccess: async (_response, variables) => {
      setTimeline((prev) =>
        prev.map((entry) =>
          entry.id === variables.entryId && entry.role === "assistant"
            ? {
                ...entry,
                action: {
                  ...entry.action,
                  requires_confirmation: false,
                  normalized_payload: {
                    ...entry.action.normalized_payload,
                    patch_applied: true,
                  },
                },
              }
            : entry,
        ),
      );
      await queryClient.invalidateQueries({
        queryKey: chatKeys.projectContext(wsId, projectId),
      });
      toast.success("已写入项目长期偏好");
    },
    onError: (error) => {
      const message = error instanceof Error ? error.message : "写入偏好失败";
      toast.error(message);
    },
  });

  const submit = () => {
    const input = draft.trim();
    if (!input || actionMutation.isPending) return;
    const timestamp = Date.now();
    const pendingId = `assistant-${timestamp}`;
    setTimeline((prev) => [
      ...prev,
      { id: `user-${timestamp}`, role: "user", text: input },
      {
        id: pendingId,
        role: "assistant",
        text: "正在路由这条请求...",
        pending: true,
        action: {
          action_type: "ask",
          normalized_payload: {
            project_id: projectId,
            pending: true,
          },
          requires_confirmation: false,
          result_title: "请求处理中",
          result_summary: "正在路由这条请求...",
          result_items: [],
        },
      },
    ]);
    setDraft("");
    actionMutation.mutate({ inputText: input, pendingId });
  };

  return (
    <div className="flex h-full flex-col bg-[radial-gradient(circle_at_top,_rgba(252,211,77,0.12),_transparent_28%),linear-gradient(180deg,_rgba(255,255,255,0.98),_rgba(250,250,249,1))]">
      <PageHeader className="gap-3 border-b bg-background/80 backdrop-blur">
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <AppLink
            href={wsPaths.projectDetail(projectId)}
            className={cn(
              buttonVariants({ variant: "ghost", size: "icon-sm" }),
              "text-muted-foreground",
            )}
          >
            <ArrowLeft className="size-4" />
          </AppLink>
          <div className="min-w-0">
            <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">
              LOFTER Pure Chat
            </div>
            <div className="truncate text-sm font-medium">
              {context?.project_title ?? "Loading project..."}
            </div>
          </div>
        </div>
        <AppLink
          href={wsPaths.projectDetail(projectId)}
          className={buttonVariants({ variant: "outline", size: "sm" })}
        >
          返回项目详情
        </AppLink>
      </PageHeader>

      <div className="mx-auto flex w-full max-w-7xl flex-1 flex-col gap-6 px-4 py-4 lg:flex-row lg:px-6">
        <div className="flex min-h-0 flex-1 flex-col gap-4">
          <Card className="border-none bg-white/70 shadow-sm">
            <CardHeader className="pb-3">
              <CardTitle className="flex items-center gap-2 text-base">
                <MessageSquareText className="size-4 text-amber-600" />
                纯聊天工作台
              </CardTitle>
              <CardDescription>
                首版先打通项目上下文、Ask/Shape/Create/Operate 路由和 Operate 隔离。
              </CardDescription>
            </CardHeader>
            <CardContent className="flex flex-wrap gap-2 pt-0">
              {quickPrompts.map((prompt) => (
                <button
                  key={prompt}
                  type="button"
                  onClick={() => setDraft(prompt)}
                  className="rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-xs text-amber-900 transition-colors hover:bg-amber-100"
                >
                  {prompt}
                </button>
              ))}
            </CardContent>
          </Card>

          <Card className="flex min-h-0 flex-1 flex-col overflow-hidden border-none bg-white shadow-sm">
            <CardContent className="flex min-h-0 flex-1 flex-col p-0">
              <ScrollArea className="flex-1 px-4 py-4">
                <div className="space-y-4">
                  {timeline.map((entry) => (
                    <div
                      key={entry.id}
                      className={cn(
                        "max-w-3xl rounded-2xl px-4 py-3",
                        entry.role === "user"
                          ? "ml-auto bg-slate-900 text-slate-50"
                          : "border border-slate-200 bg-slate-50 text-slate-900",
                      )}
                    >
                      <div className="text-sm leading-6">{entry.text}</div>
                      {entry.role === "assistant" && (
                        <ActionCard
                          action={entry.action}
                          pending={entry.pending}
                          className="mt-3"
                          isApplying={applyPatchMutation.isPending && applyPatchMutation.variables?.entryId === entry.id}
                          onApplyPatch={
                            entry.action.asset_patch_preview
                              ? () =>
                                  applyPatchMutation.mutate({
                                    entryId: entry.id,
                                    preview: entry.action.asset_patch_preview!,
                                  })
                              : undefined
                          }
                        />
                      )}
                    </div>
                  ))}
                </div>
              </ScrollArea>
              <Separator />
              <div className="space-y-3 p-4">
                <Textarea
                  value={draft}
                  onChange={(e) => setDraft(e.target.value)}
                  placeholder="描述你想问的状态、要调整的风格，或要生成的 LOFTER 低风险创作动作"
                  className="min-h-28 resize-none border-slate-200 bg-white"
                />
                <div className="flex items-center justify-between gap-3">
                  <div className="text-xs text-muted-foreground">
                    Operate 类请求只会生成转交 payload，不会在这里直接执行。
                  </div>
                  <Button onClick={submit} disabled={!draft.trim() || actionMutation.isPending}>
                    <Send className="mr-1.5 size-4" />
                    提交
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>

        <div className="w-full shrink-0 space-y-4 lg:w-[360px]">
          <Card className="border-none bg-slate-950 text-slate-50 shadow-sm">
            <CardHeader className="pb-3">
              <CardDescription className="text-slate-300">当前项目上下文</CardDescription>
              <CardTitle className="text-lg">{context?.project_title ?? "加载中"}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3 text-sm">
              <div className="flex flex-wrap gap-2">
                <Badge variant="secondary" className="bg-white/10 text-white hover:bg-white/10">
                  {context?.project_status ?? "unknown"}
                </Badge>
                <Badge variant="secondary" className="bg-white/10 text-white hover:bg-white/10">
                  {context?.project_priority ?? "none"}
                </Badge>
              </div>
              <p className="leading-6 text-slate-200">
                {isLoading ? "正在读取项目上下文..." : context?.status_summary}
              </p>
              <div className="rounded-xl border border-white/10 bg-white/5 p-3 text-xs text-slate-200">
                {context?.current_draft_label}
              </div>
            </CardContent>
          </Card>

          <Card className="border-none bg-white shadow-sm">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <Sparkles className="size-4 text-amber-600" />
                推荐下一步
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              {context?.next_recommended_actions.map((item) => (
                <div key={item} className="rounded-xl bg-amber-50 px-3 py-2 text-amber-950">
                  {item}
                </div>
              ))}
            </CardContent>
          </Card>

          <Card className="border-none bg-white shadow-sm">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <ShieldAlert className="size-4 text-emerald-600" />
                最近检查结果
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              {context?.latest_review_summary.length ? (
                context.latest_review_summary.map((item) => (
                  <div key={item} className="rounded-xl border border-emerald-100 bg-emerald-50 px-3 py-2 text-emerald-950">
                    {item}
                  </div>
                ))
              ) : (
                <div className="rounded-xl border border-dashed border-slate-200 px-3 py-3 text-muted-foreground">
                  当前还没有最近检查结果。
                </div>
              )}
            </CardContent>
          </Card>

          <Card className="border-none bg-white shadow-sm">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <CheckCircle2 className="size-4 text-emerald-600" />
                资产快照
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              {snapshotItems.length ? (
                snapshotItems.map((item) => (
                  <div
                    key={`${item.group}:${item.value}`}
                    className="rounded-xl border border-slate-200 px-3 py-2"
                  >
                    <div className="text-[11px] uppercase tracking-[0.14em] text-muted-foreground">
                      {item.group}
                    </div>
                    <div className="mt-1 leading-6 text-slate-800">{item.value}</div>
                  </div>
                ))
              ) : (
                <div className="rounded-xl border border-dashed border-slate-200 px-3 py-3 text-muted-foreground">
                  当前还没有明确写入的创作偏好，首版会先用项目描述和资源做协作。
                </div>
              )}
            </CardContent>
          </Card>

          <Card className="border-none bg-white shadow-sm">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <FileText className="size-4 text-sky-600" />
                最近产物
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              {context?.latest_artifacts.length ? (
                context.latest_artifacts.map((artifact) => (
                  <div key={`${artifact.kind}-${artifact.label}`} className="rounded-xl border border-slate-200 px-3 py-2">
                    <div className="font-medium">{artifact.label}</div>
                    <div className="text-xs text-muted-foreground">{artifact.summary}</div>
                    {artifact.created_at && (
                      <div className="mt-1 text-[11px] text-muted-foreground">{formatRelativeStamp(artifact.created_at)}</div>
                    )}
                    <div className="mt-1 break-all text-[11px] text-muted-foreground">
                      {formatArtifactRef(artifact)}
                    </div>
                  </div>
                ))
              ) : (
                <div className="rounded-xl border border-dashed border-slate-200 px-3 py-3 text-muted-foreground">
                  当前还没有工作台生成的阶段产物。
                </div>
              )}
            </CardContent>
          </Card>

          <Card className="border-none bg-white shadow-sm">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <MessageSquareText className="size-4 text-emerald-600" />
                最近动作
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              {context?.recent_actions.length ? (
                context.recent_actions.map((action, index) => (
                  <div key={`${action.action_type}-${action.result_title}-${index}`} className="rounded-xl border border-slate-200 px-3 py-2">
                    <div className="flex items-center justify-between gap-2">
                      <div className="font-medium">{action.result_title}</div>
                      <Badge variant="outline">{action.action_type.toUpperCase()}</Badge>
                    </div>
                    <div className="mt-1 text-xs text-muted-foreground">{action.result_summary}</div>
                  </div>
                ))
              ) : (
                <div className="rounded-xl border border-dashed border-slate-200 px-3 py-3 text-muted-foreground">
                  当前还没有可回看的最近动作。
                </div>
              )}
            </CardContent>
          </Card>

          <Card className="border-none bg-white shadow-sm">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <FileText className="size-4 text-sky-600" />
                已挂接资源
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              {context?.attached_resources.length ? (
                context.attached_resources.map((artifact) => (
                  <div key={`${artifact.kind}-${artifact.label}`} className="rounded-xl border border-slate-200 px-3 py-2">
                    <div className="font-medium">{artifact.label}</div>
                    <div className="text-xs text-muted-foreground">{artifact.summary}</div>
                    <div className="mt-1 break-all text-[11px] text-muted-foreground">
                      {formatArtifactRef(artifact)}
                    </div>
                  </div>
                ))
              ) : (
                <div className="rounded-xl border border-dashed border-slate-200 px-3 py-3 text-muted-foreground">
                  当前项目还没有挂接创作资源。
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}

function ActionCard({
  action,
  pending,
  className,
  isApplying,
  onApplyPatch,
}: {
  action: ProjectChatActionResponse;
  pending?: boolean;
  className?: string;
  isApplying?: boolean;
  onApplyPatch?: () => void;
}) {
  const tone =
    action.action_type === "operate"
      ? "border-rose-200 bg-rose-50"
      : action.action_type === "shape"
        ? "border-amber-200 bg-amber-50"
        : "border-slate-200 bg-white";

  return (
    <div className={cn("rounded-2xl border p-3", tone, className)}>
      <div className="flex flex-wrap items-center gap-2">
        <Badge variant="outline">{action.action_type.toUpperCase()}</Badge>
        <span className="text-sm font-medium">{action.result_title}</span>
        {pending && <span className="text-xs text-muted-foreground">处理中</span>}
        {action.normalized_payload.adapter_status === "live" && (
          <span className="text-xs text-emerald-700">live adapter</span>
        )}
        {action.normalized_payload.adapter_status === "fallback" && (
          <span className="text-xs text-amber-700">fallback</span>
        )}
        {action.requires_confirmation && (
          <span className="inline-flex items-center gap-1 text-xs text-amber-700">
            <CheckCircle2 className="size-3.5" />
            需要确认
          </span>
        )}
      </div>
      <p className="mt-2 text-sm leading-6 text-slate-700">{action.result_summary}</p>
      {action.result_items.length > 0 && (
        <div className="mt-3 space-y-1.5 text-xs text-slate-600">
          {action.result_items.map((item) => (
            <div key={item} className="rounded-lg bg-white/80 px-2.5 py-2">
              {item}
            </div>
          ))}
        </div>
      )}
      {extractStringList(action.normalized_payload.applied_rules).length > 0 && (
        <div className="mt-3 rounded-xl border border-slate-200 bg-white/70 p-3 text-xs text-slate-600">
          <div className="font-medium text-slate-800">adapter 处理摘要</div>
          <div className="mt-2 space-y-1">
            {extractStringList(action.normalized_payload.applied_rules).map((item) => (
              <div key={item}>{item}</div>
            ))}
          </div>
        </div>
      )}
      {extractStringList(action.normalized_payload.quality_summary).length > 0 && (
        <div className="mt-3 rounded-xl border border-emerald-200 bg-white/70 p-3 text-xs text-slate-600">
          <div className="font-medium text-slate-800">最近检查结果</div>
          <div className="mt-2 space-y-1">
            {extractStringList(action.normalized_payload.quality_summary).map((item) => (
              <div key={item}>{item}</div>
            ))}
          </div>
        </div>
      )}
      {action.asset_patch_preview && (
        <div className="mt-3 rounded-xl border border-amber-200 bg-white/80 p-3 text-xs text-slate-700">
          <div className="font-medium">{action.asset_patch_preview.asset_target}</div>
          <div className="mt-1">{action.asset_patch_preview.summary}</div>
          <div className="mt-2 rounded-lg bg-amber-50 px-2.5 py-2 text-amber-950">
            {action.asset_patch_preview.patch}
          </div>
          <div className="mt-3 flex items-center gap-2">
            <Button
              type="button"
              size="sm"
              variant="outline"
              onClick={onApplyPatch}
              disabled={!onApplyPatch || isApplying || action.normalized_payload.patch_applied === true}
            >
              {action.normalized_payload.patch_applied === true
                ? "已写入偏好"
                : isApplying
                  ? "写入中..."
                  : "写入偏好"}
            </Button>
            {action.normalized_payload.patch_applied === true && (
              <span className="text-xs text-emerald-700">下次 Create/Shape 会直接带上这个长期约束</span>
            )}
          </div>
        </div>
      )}
      {action.operate_handoff && (
        <div className="mt-3 rounded-xl border border-rose-200 bg-white/80 p-3 text-xs text-slate-700">
          <div className="flex items-center gap-1.5 font-medium text-rose-700">
            <ShieldAlert className="size-3.5" />
            {action.operate_handoff.operation}
          </div>
          <div className="mt-1">{action.operate_handoff.risk_reason}</div>
          <div className="mt-2 text-muted-foreground">
            destination: {action.operate_handoff.destination}
          </div>
        </div>
      )}
    </div>
  );
}

function buildQuickPrompts(context?: ProjectChatContext) {
  const projectName = context?.project_title ?? "这个项目";
  return [
    `先总结${projectName}现在适合从哪一步开始`,
    "给我 3 个更克制一点的标题方向",
    "把这个标题“【朝俞】他都收手了，贺朝还在装不在意”改得更克制一点",
    "以后标题更克制一点，不要太狗血",
    "把这段去 AI 味：“空气仿佛凝固，贺朝喉咙发紧，眼神动了动。”",
  ];
}

function formatArtifactRef(artifact: ProjectChatArtifact) {
  if (artifact.kind === "github_repo") {
    try {
      const parsed = JSON.parse(artifact.ref) as { url?: string };
      if (parsed.url) return parsed.url;
    } catch {}
  }
  return artifact.ref;
}

function formatRelativeStamp(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function extractStringList(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  return value.filter((item): item is string => typeof item === "string" && item.length > 0);
}
