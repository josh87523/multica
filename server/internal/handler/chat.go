package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// ---------------------------------------------------------------------------
// Chat Sessions
// ---------------------------------------------------------------------------

type CreateChatSessionRequest struct {
	AgentID string `json:"agent_id"`
	Title   string `json:"title"`
}

func (h *Handler) CreateChatSession(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())

	var req CreateChatSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.AgentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}
	agentID, ok := parseUUIDOrBadRequest(w, req.AgentID, "agent_id")
	if !ok {
		return
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}

	// Verify agent exists in workspace.
	agent, err := h.Queries.GetAgentInWorkspace(r.Context(), db.GetAgentInWorkspaceParams{
		ID:          agentID,
		WorkspaceID: workspaceUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	if agent.ArchivedAt.Valid {
		writeError(w, http.StatusBadRequest, "agent is archived")
		return
	}

	session, err := h.Queries.CreateChatSession(r.Context(), db.CreateChatSessionParams{
		WorkspaceID: workspaceUUID,
		AgentID:     agentID,
		CreatorID:   parseUUID(userID),
		Title:       req.Title,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create chat session")
		return
	}

	writeJSON(w, http.StatusCreated, chatSessionToResponse(session))
}

func (h *Handler) ListChatSessions(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())

	status := r.URL.Query().Get("status")

	// Two call sites → two row types with identical shape. Collect into a
	// common response slice via small per-branch loops.
	var resp []ChatSessionResponse
	if status == "all" {
		rows, err := h.Queries.ListAllChatSessionsByCreator(r.Context(), db.ListAllChatSessionsByCreatorParams{
			WorkspaceID: parseUUID(workspaceID),
			CreatorID:   parseUUID(userID),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list chat sessions")
			return
		}
		resp = make([]ChatSessionResponse, len(rows))
		for i, s := range rows {
			resp[i] = ChatSessionResponse{
				ID:          uuidToString(s.ID),
				WorkspaceID: uuidToString(s.WorkspaceID),
				AgentID:     uuidToString(s.AgentID),
				CreatorID:   uuidToString(s.CreatorID),
				Title:       s.Title,
				Status:      s.Status,
				HasUnread:   s.HasUnread,
				CreatedAt:   timestampToString(s.CreatedAt),
				UpdatedAt:   timestampToString(s.UpdatedAt),
			}
		}
	} else {
		rows, err := h.Queries.ListChatSessionsByCreator(r.Context(), db.ListChatSessionsByCreatorParams{
			WorkspaceID: parseUUID(workspaceID),
			CreatorID:   parseUUID(userID),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list chat sessions")
			return
		}
		resp = make([]ChatSessionResponse, len(rows))
		for i, s := range rows {
			resp[i] = ChatSessionResponse{
				ID:          uuidToString(s.ID),
				WorkspaceID: uuidToString(s.WorkspaceID),
				AgentID:     uuidToString(s.AgentID),
				CreatorID:   uuidToString(s.CreatorID),
				Title:       s.Title,
				Status:      s.Status,
				HasUnread:   s.HasUnread,
				CreatedAt:   timestampToString(s.CreatedAt),
				UpdatedAt:   timestampToString(s.UpdatedAt),
			}
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) loadChatSessionForUser(w http.ResponseWriter, r *http.Request, userID, workspaceID, sessionID string) (db.ChatSession, bool) {
	sessionUUID, ok := parseUUIDOrBadRequest(w, sessionID, "chat session id")
	if !ok {
		return db.ChatSession{}, false
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return db.ChatSession{}, false
	}
	session, err := h.Queries.GetChatSessionInWorkspace(r.Context(), db.GetChatSessionInWorkspaceParams{
		ID:          sessionUUID,
		WorkspaceID: workspaceUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "chat session not found")
		return db.ChatSession{}, false
	}
	if uuidToString(session.CreatorID) != userID {
		writeError(w, http.StatusForbidden, "not your chat session")
		return db.ChatSession{}, false
	}
	return session, true
}

func (h *Handler) GetChatSession(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	sessionID := chi.URLParam(r, "sessionId")

	session, ok := h.loadChatSessionForUser(w, r, userID, workspaceID, sessionID)
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, chatSessionToResponse(session))
}

type ProjectChatArtifactResponse struct {
	Kind      string `json:"kind"`
	Label     string `json:"label"`
	Summary   string `json:"summary"`
	Ref       string `json:"ref"`
	CreatedAt string `json:"created_at,omitempty"`
}

type ProjectCreativeAssetSnapshotResponse struct {
	StyleExamples    []string `json:"style_examples"`
	TitlePreferences []string `json:"title_preferences"`
	ShapePreferences []string `json:"shape_preferences"`
	HistoricalNotes  []string `json:"historical_notes"`
}

type ProjectChatContextResponse struct {
	ProjectID              string                               `json:"project_id"`
	ProjectTitle           string                               `json:"project_title"`
	ProjectStatus          string                               `json:"project_status"`
	ProjectPriority        string                               `json:"project_priority"`
	StatusSummary          string                               `json:"status_summary"`
	LatestReviewSummary    []string                             `json:"latest_review_summary"`
	CurrentDraftLabel      string                               `json:"current_draft_label"`
	NextRecommendedActions []string                             `json:"next_recommended_actions"`
	CreativeAssetSnapshot  ProjectCreativeAssetSnapshotResponse `json:"creative_asset_snapshot"`
	LatestArtifacts        []ProjectChatArtifactResponse        `json:"latest_artifacts"`
	AttachedResources      []ProjectChatArtifactResponse        `json:"attached_resources"`
	RecentActions          []ProjectChatActionResponse          `json:"recent_actions"`
}

type RunProjectChatActionRequest struct {
	InputText   string `json:"input_text"`
	ContextHint string `json:"context_hint"`
}

type ProjectChatAssetPatchPreview struct {
	AssetTarget string `json:"asset_target"`
	Summary     string `json:"summary"`
	Patch       string `json:"patch"`
}

type ProjectChatOperateHandoff struct {
	Operation   string         `json:"operation"`
	Payload     map[string]any `json:"payload"`
	RiskReason  string         `json:"risk_reason"`
	Destination string         `json:"destination"`
}

type ProjectChatActionResponse struct {
	ActionType           string                        `json:"action_type"`
	NormalizedPayload    map[string]any                `json:"normalized_payload"`
	RequiresConfirmation bool                          `json:"requires_confirmation"`
	ResultTitle          string                        `json:"result_title"`
	ResultSummary        string                        `json:"result_summary"`
	ResultItems          []string                      `json:"result_items"`
	AssetPatchPreview    *ProjectChatAssetPatchPreview `json:"asset_patch_preview,omitempty"`
	OperateHandoff       *ProjectChatOperateHandoff    `json:"operate_handoff,omitempty"`
}

type ApplyProjectChatAssetPatchRequest struct {
	AssetTarget string `json:"asset_target"`
	Patch       string `json:"patch"`
}

type ApplyProjectChatAssetPatchResponse struct {
	UpdatedAssetSnapshot ProjectCreativeAssetSnapshotResponse `json:"updated_asset_snapshot"`
}

func (h *Handler) GetProjectChatContext(w http.ResponseWriter, r *http.Request) {
	project, resources, ok := h.loadProjectChatContext(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.buildProjectChatContext(project, resources))
}

func (h *Handler) RunProjectChatAction(w http.ResponseWriter, r *http.Request) {
	project, resources, ok := h.loadProjectChatContext(w, r)
	if !ok {
		return
	}

	var req RunProjectChatActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.InputText = strings.TrimSpace(req.InputText)
	if req.InputText == "" {
		writeError(w, http.StatusBadRequest, "input_text is required")
		return
	}

	ctx := h.buildProjectChatContext(project, resources)
	actionType := classifyProjectChatAction(req.InputText)
	resp := h.buildProjectChatActionResponse(r.Context(), project, ctx, req.InputText, actionType)
	h.persistProjectChatAction(project, req.InputText, resp)
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) ApplyProjectChatAssetPatch(w http.ResponseWriter, r *http.Request) {
	project, _, ok := h.loadProjectChatContext(w, r)
	if !ok {
		return
	}
	var req ApplyProjectChatAssetPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.AssetTarget = strings.TrimSpace(req.AssetTarget)
	req.Patch = strings.TrimSpace(req.Patch)
	if req.AssetTarget == "" {
		writeError(w, http.StatusBadRequest, "asset_target is required")
		return
	}
	if req.Patch == "" {
		writeError(w, http.StatusBadRequest, "patch is required")
		return
	}
	snapshot, err := h.ProjectChatAssets.ApplyPatch(
		uuidToString(project.WorkspaceID),
		uuidToString(project.ID),
		req.AssetTarget,
		req.Patch,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to apply asset patch")
		return
	}
	writeJSON(w, http.StatusOK, ApplyProjectChatAssetPatchResponse{
		UpdatedAssetSnapshot: snapshot,
	})
	h.persistProjectChatShapePatch(project, req)
}

func (h *Handler) loadProjectChatContext(w http.ResponseWriter, r *http.Request) (db.Project, []db.ProjectResource, bool) {
	projectID := chi.URLParam(r, "projectId")
	workspaceID := h.resolveWorkspaceID(r)
	projectUUID, ok := parseUUIDOrBadRequest(w, projectID, "project id")
	if !ok {
		return db.Project{}, nil, false
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return db.Project{}, nil, false
	}
	project, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{
		ID:          projectUUID,
		WorkspaceID: workspaceUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return db.Project{}, nil, false
	}
	resources, err := h.Queries.ListProjectResources(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load project resources")
		return db.Project{}, nil, false
	}
	return project, resources, true
}

func (h *Handler) buildProjectChatContext(project db.Project, resources []db.ProjectResource) ProjectChatContextResponse {
	projectID := uuidToString(project.ID)
	attachedResources := make([]ProjectChatArtifactResponse, 0, len(resources))
	for _, resource := range resources {
		ref := string(resource.ResourceRef)
		label := resource.ResourceType
		if resource.Label.Valid && strings.TrimSpace(resource.Label.String) != "" {
			label = strings.TrimSpace(resource.Label.String)
		}
		attachedResources = append(attachedResources, ProjectChatArtifactResponse{
			Kind:    resource.ResourceType,
			Label:   label,
			Summary: "Attached project resource",
			Ref:     ref,
		})
	}

	statusSummary := "项目已接入纯聊天工作台骨架，可先用 Ask 梳理上下文，再用 Create/Shape 走低风险创作协作。"
	nextActions := []string{
		"Ask: 先确认当前项目状态、资源和下一步创作建议",
		"Create: 生成标题候选、改写文本或做去 AI 味预览",
		"Shape: 把标题风格、角色口吻、CP 拉扯强度整理成可确认补丁",
	}
	if len(resources) == 0 {
		statusSummary = "项目存在但还没有挂接创作资源，首版建议先补资源，再进入低风险创作协作。"
		nextActions[0] = "先补充项目资源，让工作台有可引用的创作依据"
	}
	if project.Description.Valid && strings.TrimSpace(project.Description.String) != "" {
		statusSummary = "项目已有描述信息，纯聊天首版会把它作为创作背景，再结合挂接资源做 Ask/Shape/Create 路由。"
	}

	historicalNotes := []string{}
	if project.Description.Valid {
		desc := strings.TrimSpace(project.Description.String)
		if desc != "" {
			historicalNotes = append(historicalNotes, desc)
		}
	}
	assetSnapshot := emptyProjectCreativeAssetSnapshot()
	assetSnapshot.HistoricalNotes = historicalNotes
	if h.ProjectChatAssets != nil {
		if stored, err := h.ProjectChatAssets.Load(uuidToString(project.WorkspaceID), projectID); err == nil {
			assetSnapshot = stored
			assetSnapshot.HistoricalNotes = appendUniqueStrings(assetSnapshot.HistoricalNotes, historicalNotes...)
		}
	}
	latestArtifacts := make([]ProjectChatArtifactResponse, 0, len(attachedResources)+4)
	latestArtifacts = append(latestArtifacts, attachedResources...)
	recentActions := []ProjectChatActionResponse{}
	latestReviewSummary := []string{}
	if h.ProjectChatRecords != nil {
		if storedArtifacts, err := h.ProjectChatRecords.ListArtifacts(uuidToString(project.WorkspaceID), projectID); err == nil {
			for _, artifact := range limitProjectChatArtifacts(storedArtifacts, 4) {
				latestArtifacts = append([]ProjectChatArtifactResponse{{
					Kind:      artifact.ArtifactType,
					Label:     artifact.SourceAdapter,
					Summary:   artifact.Summary,
					Ref:       artifact.Content,
					CreatedAt: artifact.CreatedAt,
				}}, latestArtifacts...)
			}
		}
		if storedActions, err := h.ProjectChatRecords.ListActions(uuidToString(project.WorkspaceID), projectID); err == nil {
			recentActions = projectChatActionRecordsToResponse(limitProjectChatActions(storedActions, 6))
			latestReviewSummary = extractLatestProjectChatQualitySummary(storedActions)
		}
	}
	if len(latestReviewSummary) == 0 {
		latestReviewSummary = []string{"最近还没有生成质量评审摘要，先通过 Create 产出一个候选再读回 review。"}
	}

	return ProjectChatContextResponse{
		ProjectID:           projectID,
		ProjectTitle:        project.Title,
		ProjectStatus:       project.Status,
		ProjectPriority:     project.Priority,
		StatusSummary:       statusSummary,
		LatestReviewSummary: latestReviewSummary,
		CurrentDraftLabel: func() string {
			if project.Description.Valid && strings.TrimSpace(project.Description.String) != "" {
				return "当前以项目描述作为首版创作背景"
			}
			return "当前无明确草稿，先从项目上下文启动"
		}(),
		NextRecommendedActions: nextActions,
		CreativeAssetSnapshot:  assetSnapshot,
		LatestArtifacts:        latestArtifacts,
		AttachedResources:      attachedResources,
		RecentActions:          recentActions,
	}
}

func classifyProjectChatAction(input string) string {
	raw := strings.TrimSpace(input)
	lower := strings.ToLower(raw)
	operateKeywords := []string{
		"publish", "schedule", "delete comment", "reply comment", "relogin",
		"发掉", "发布", "删评", "删除评论", "回复评论", "重登", "排期", "跑批",
	}
	for _, keyword := range operateKeywords {
		if strings.Contains(lower, keyword) || strings.Contains(raw, keyword) {
			return "operate"
		}
	}

	shapeKeywords := []string{
		"以后", "偏好", "风格", "口吻", "不要太", "更克制", "更轻", "拉扯", "ai味规避",
	}
	for _, keyword := range shapeKeywords {
		if strings.Contains(lower, keyword) || strings.Contains(raw, keyword) {
			return "shape"
		}
	}

	createKeywords := []string{
		"生成", "给我", "改写", "重写", "标题", "去ai", "去 ai", "润色", "候选", "版本", "rewrite", "generate",
	}
	for _, keyword := range createKeywords {
		if strings.Contains(lower, keyword) || strings.Contains(raw, keyword) {
			return "create"
		}
	}

	if strings.Contains(raw, "？") || strings.Contains(raw, "?") {
		return "ask"
	}
	return "ask"
}

func (h *Handler) buildProjectChatActionResponse(ctx context.Context, project db.Project, projectCtx ProjectChatContextResponse, inputText, actionType string) ProjectChatActionResponse {
	base := ProjectChatActionResponse{
		ActionType: actionType,
		NormalizedPayload: map[string]any{
			"project_id":  uuidToString(project.ID),
			"input_text":  inputText,
			"project_tag": project.Title,
		},
		ResultItems: []string{},
	}

	switch actionType {
	case "operate":
		operation := "operate_request"
		switch {
		case strings.Contains(inputText, "发"):
			operation = "publish_post"
		case strings.Contains(inputText, "删"):
			operation = "delete_comment"
		case strings.Contains(strings.ToLower(inputText), "relogin") || strings.Contains(inputText, "重登"):
			operation = "relogin"
		case strings.Contains(strings.ToLower(inputText), "schedule") || strings.Contains(inputText, "排期"):
			operation = "schedule_run"
		}
		base.NormalizedPayload["operation"] = operation
		base.ResultTitle = "Operate request 已识别"
		base.ResultSummary = "首版工作台只生成结构化转交请求，不在聊天页直接触发外部 side effect。"
		base.ResultItems = []string{
			"该请求会转交给运营执行台而不是当前聊天页",
			"后续需要执行面做确认、执行和终态读回",
		}
		base.OperateHandoff = &ProjectChatOperateHandoff{
			Operation: operation,
			Payload: map[string]any{
				"project_id": uuidToString(project.ID),
				"text":       inputText,
			},
			RiskReason:  "涉及真实平台 side effect，首版纯聊天工作台不直接执行",
			Destination: "operations-console",
		}
	case "shape":
		target := "shape_preferences"
		switch {
		case strings.Contains(inputText, "标题"):
			target = "title_preferences"
		case strings.Contains(inputText, "口吻"), strings.Contains(inputText, "角色"):
			target = "voice_preferences"
		}
		base.NormalizedPayload["asset_target"] = target
		base.RequiresConfirmation = true
		base.ResultTitle = "Shape 补丁预览"
		base.ResultSummary = "已把你的方向性要求整理成可确认补丁；确认前只作为本轮建议，不写入长期偏好层。"
		base.ResultItems = []string{
			"首版先回显系统理解摘要，再等待确认",
			"如果理解不稳，应降级成一次性建议而不是长期偏好",
		}
		base.AssetPatchPreview = &ProjectChatAssetPatchPreview{
			AssetTarget: target,
			Summary:     "系统理解到这是对后续创作方向的长期约束",
			Patch:       inputText,
		}
	case "create":
		mode := classifyProjectChatCreateMode(inputText)
		base.NormalizedPayload["mode"] = mode
		if mode == "title_candidates" {
			req := ProjectChatTitleRequest{
				ProjectTitle:       project.Title,
				ProjectDescription: strings.Join(projectCtx.CreativeAssetSnapshot.HistoricalNotes, "\n"),
				InputText:          inputText,
			}
			base.NormalizedPayload["cp"] = deriveProjectChatCP(req)
			if h.ProjectChatTitles != nil {
				titles, err := h.ProjectChatTitles.GenerateTitles(ctx, req)
				if err == nil && len(titles) > 0 {
					base.NormalizedPayload["adapter"] = "lofter_title"
					base.NormalizedPayload["adapter_status"] = "live"
					base.ResultTitle = "Create 标题候选"
					base.ResultSummary = "已通过 LOFTER 标题 adapter 生成首批候选，可继续让工作台按这个方向细化。"
					base.ResultItems = titles
					attachProjectChatQualitySummary(ctx, h, project, projectCtx, &base, titles[0], strings.Join(projectCtx.CreativeAssetSnapshot.HistoricalNotes, "\n"))
					return base
				}
				base.NormalizedPayload["adapter"] = "lofter_title"
				base.NormalizedPayload["adapter_status"] = "fallback"
				base.NormalizedPayload["adapter_error"] = err.Error()
			}
			base.ResultTitle = "Create 标题候选"
			base.ResultSummary = "LOFTER 标题 adapter 当前不可用，先回退到本地候选，避免聊天页完全失去创作反馈。"
			base.ResultItems = buildFallbackProjectChatTitles(req)
			attachProjectChatQualitySummary(ctx, h, project, projectCtx, &base, firstString(base.ResultItems), strings.Join(projectCtx.CreativeAssetSnapshot.HistoricalNotes, "\n"))
			return base
		}
		if mode == "rewrite_title" {
			req := ProjectChatRewriteTitleRequest{
				CP:             deriveProjectChatCP(ProjectChatTitleRequest{ProjectTitle: project.Title}),
				OriginalTitle:  extractProjectChatQuotedText(inputText),
				Instruction:    inputText,
				ReferenceTitle: firstProjectChatTitlePreference(projectCtx.CreativeAssetSnapshot),
			}
			if req.OriginalTitle == "" {
				req.OriginalTitle = firstProjectChatTitlePreference(projectCtx.CreativeAssetSnapshot)
			}
			base.NormalizedPayload["adapter"] = "lofter_title_rewrite"
			base.NormalizedPayload["original_title"] = req.OriginalTitle
			if h.ProjectChatCreate != nil {
				rewritten, err := h.ProjectChatCreate.RewriteTitle(ctx, req)
				if err == nil && strings.TrimSpace(rewritten) != "" {
					base.NormalizedPayload["adapter_status"] = "live"
					base.ResultTitle = "Create 标题改写"
					base.ResultSummary = "已通过 LOFTER 标题改写链路生成一个更贴近当前指令的版本。"
					base.ResultItems = []string{rewritten}
					attachProjectChatQualitySummary(ctx, h, project, projectCtx, &base, rewritten, strings.Join(projectCtx.CreativeAssetSnapshot.HistoricalNotes, "\n"))
					return base
				}
				base.NormalizedPayload["adapter_status"] = "fallback"
				if err != nil {
					base.NormalizedPayload["adapter_error"] = err.Error()
				} else {
					base.NormalizedPayload["adapter_error"] = "lofter rewrite adapter returned empty title"
				}
			}
			base.ResultTitle = "Create 标题改写"
			base.ResultSummary = "LOFTER 标题改写 adapter 当前不可用，先回退到本地改写结果。"
			base.ResultItems = []string{buildFallbackProjectChatRewriteTitle(req)}
			attachProjectChatQualitySummary(ctx, h, project, projectCtx, &base, firstString(base.ResultItems), strings.Join(projectCtx.CreativeAssetSnapshot.HistoricalNotes, "\n"))
			return base
		}
		if mode == "deai_rewrite" {
			req := ProjectChatDeAIRequest{
				Text:        extractProjectChatQuotedText(inputText),
				Instruction: inputText,
			}
			if req.Text == "" {
				req.Text = strings.Join(projectCtx.CreativeAssetSnapshot.HistoricalNotes, "\n")
			}
			base.NormalizedPayload["adapter"] = "lofter_deai"
			if h.ProjectChatCreate != nil {
				rewritten, rules, err := h.ProjectChatCreate.DeAI(ctx, req)
				if err == nil && strings.TrimSpace(rewritten) != "" {
					base.NormalizedPayload["adapter_status"] = "live"
					if len(rules) > 0 {
						base.NormalizedPayload["applied_rules"] = rules
					}
					base.ResultTitle = "Create 去 AI 味"
					base.ResultSummary = "已通过 LOFTER 去 AI 味链路产出一版更克制的文本。"
					base.ResultItems = []string{rewritten}
					attachProjectChatQualitySummary(ctx, h, project, projectCtx, &base, "", rewritten)
					return base
				}
				base.NormalizedPayload["adapter_status"] = "fallback"
				if err != nil {
					base.NormalizedPayload["adapter_error"] = err.Error()
				} else {
					base.NormalizedPayload["adapter_error"] = "lofter deai adapter returned empty text"
				}
			}
			fallbackText, rules := buildFallbackProjectChatDeAI(req)
			if len(rules) > 0 {
				base.NormalizedPayload["applied_rules"] = rules
			}
			base.ResultTitle = "Create 去 AI 味"
			base.ResultSummary = "LOFTER 去 AI 味 adapter 当前不可用，先回退到本地规则降噪结果。"
			base.ResultItems = []string{fallbackText}
			attachProjectChatQualitySummary(ctx, h, project, projectCtx, &base, "", fallbackText)
			return base
		}
		base.ResultTitle = "Create 低风险创作预览"
		base.ResultSummary = "首版已识别为生成/改写请求，下一步会把它映射到标题、改写、去 AI 味等 LOFTER 低风险能力。"
		base.ResultItems = []string{
			"当前动作已进入 Create 路径，但还没有接上对应 adapter",
			"优先已打通的是标题候选链路，其他 Create 能力后续继续接入",
		}
	default:
		base.ResultTitle = "Ask 上下文回答"
		base.ResultSummary = projectCtx.StatusSummary
		base.ResultItems = append(base.ResultItems, projectCtx.NextRecommendedActions...)
		base.ResultItems = append(base.ResultItems, projectCtx.LatestReviewSummary...)
	}

	return base
}

func classifyProjectChatCreateMode(inputText string) string {
	lower := strings.ToLower(inputText)
	rewriteTitleKeywords := []string{"改标题", "标题改", "标题重写", "把这个标题", "rewrite title"}
	for _, keyword := range rewriteTitleKeywords {
		if strings.Contains(lower, keyword) || strings.Contains(inputText, keyword) {
			return "rewrite_title"
		}
	}
	deaiKeywords := []string{"去ai", "去 ai", "去AI", "AI味", "ai味", "降ai", "降 AI"}
	for _, keyword := range deaiKeywords {
		if strings.Contains(lower, strings.ToLower(keyword)) || strings.Contains(inputText, keyword) {
			return "deai_rewrite"
		}
	}
	titleKeywords := []string{"标题", "title"}
	for _, keyword := range titleKeywords {
		if strings.Contains(lower, keyword) || strings.Contains(inputText, keyword) {
			return "title_candidates"
		}
	}
	return "generic_create"
}

func firstProjectChatTitlePreference(snapshot ProjectCreativeAssetSnapshotResponse) string {
	if len(snapshot.TitlePreferences) == 0 {
		return ""
	}
	return strings.TrimSpace(snapshot.TitlePreferences[0])
}

func extractProjectChatQuotedText(input string) string {
	quotes := [][2]string{
		{"“", "”"},
		{"\"", "\""},
		{"《", "》"},
	}
	for _, pair := range quotes {
		start := strings.Index(input, pair[0])
		end := strings.LastIndex(input, pair[1])
		if start >= 0 && end > start {
			candidate := strings.TrimSpace(input[start+len(pair[0]) : end])
			if candidate != "" {
				return candidate
			}
		}
	}
	return ""
}

func (h *Handler) persistProjectChatAction(project db.Project, inputText string, resp ProjectChatActionResponse) {
	if h.ProjectChatRecords == nil {
		return
	}
	workspaceID := uuidToString(project.WorkspaceID)
	projectID := uuidToString(project.ID)
	artifacts := buildProjectChatArtifactsFromAction(projectID, resp)
	resultRefs := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		resultRefs = append(resultRefs, artifact.ArtifactID)
	}
	if len(artifacts) > 0 {
		_ = h.ProjectChatRecords.AppendArtifacts(workspaceID, projectID, artifacts)
	}
	record := ProjectChatActionRecord{
		ActionID:             "act_" + randomID(),
		ProjectID:            projectID,
		ActionType:           resp.ActionType,
		Target:               projectChatActionTarget(resp),
		InputText:            inputText,
		NormalizedPayload:    resp.NormalizedPayload,
		Status:               projectChatActionStatus(resp),
		ResultRefs:           resultRefs,
		RequiresConfirmation: resp.RequiresConfirmation,
		ResultTitle:          resp.ResultTitle,
		ResultSummary:        resp.ResultSummary,
		ResultItems:          resp.ResultItems,
	}
	_ = h.ProjectChatRecords.AppendAction(workspaceID, projectID, record)
}

func (h *Handler) persistProjectChatShapePatch(project db.Project, req ApplyProjectChatAssetPatchRequest) {
	if h.ProjectChatRecords == nil {
		return
	}
	record := ProjectChatActionRecord{
		ActionID:             "act_" + randomID(),
		ProjectID:            uuidToString(project.ID),
		ActionType:           "shape",
		Target:               req.AssetTarget,
		InputText:            req.Patch,
		NormalizedPayload:    map[string]any{"asset_target": req.AssetTarget, "patch_applied": true},
		Status:               "applied",
		RequiresConfirmation: false,
		ResultTitle:          "Shape 偏好已写入",
		ResultSummary:        "已把确认后的偏好写入工作台资产层，后续 Create/Shape 会读取它。",
		ResultItems:          []string{req.Patch},
	}
	_ = h.ProjectChatRecords.AppendAction(uuidToString(project.WorkspaceID), uuidToString(project.ID), record)
}

func buildProjectChatArtifactsFromAction(projectID string, resp ProjectChatActionResponse) []ProjectChatArtifactRecord {
	if resp.ActionType != "create" || len(resp.ResultItems) == 0 {
		return nil
	}
	adapter := strings.TrimSpace(fmt.Sprint(resp.NormalizedPayload["adapter"]))
	if adapter == "" {
		adapter = "project_chat"
	}
	mode := strings.TrimSpace(fmt.Sprint(resp.NormalizedPayload["mode"]))
	stage := "create"
	if mode != "" {
		stage = mode
	}
	artifactType := "generated_text"
	if mode == "title_candidates" || mode == "rewrite_title" {
		artifactType = "title_candidate"
	}
	records := make([]ProjectChatArtifactRecord, 0, len(resp.ResultItems))
	for _, item := range resp.ResultItems {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		records = append(records, ProjectChatArtifactRecord{
			ArtifactID:    "art_" + randomID(),
			ProjectID:     projectID,
			Stage:         stage,
			ArtifactType:  artifactType,
			SourceAdapter: adapter,
			Path:          "",
			Summary:       resp.ResultTitle,
			Content:       item,
		})
	}
	return records
}

func projectChatActionTarget(resp ProjectChatActionResponse) string {
	if target := strings.TrimSpace(fmt.Sprint(resp.NormalizedPayload["asset_target"])); target != "" {
		return target
	}
	if mode := strings.TrimSpace(fmt.Sprint(resp.NormalizedPayload["mode"])); mode != "" {
		return mode
	}
	if operation := strings.TrimSpace(fmt.Sprint(resp.NormalizedPayload["operation"])); operation != "" {
		return operation
	}
	return resp.ActionType
}

func projectChatActionStatus(resp ProjectChatActionResponse) string {
	if resp.RequiresConfirmation {
		return "pending_confirmation"
	}
	if resp.ActionType == "operate" {
		return "handoff_ready"
	}
	return "completed"
}

func limitProjectChatArtifacts(records []ProjectChatArtifactRecord, limit int) []ProjectChatArtifactRecord {
	if len(records) <= limit {
		return records
	}
	return records[:limit]
}

func limitProjectChatActions(records []ProjectChatActionRecord, limit int) []ProjectChatActionRecord {
	if len(records) <= limit {
		return records
	}
	return records[:limit]
}

func projectChatActionRecordsToResponse(records []ProjectChatActionRecord) []ProjectChatActionResponse {
	resp := make([]ProjectChatActionResponse, 0, len(records))
	for _, record := range records {
		resp = append(resp, ProjectChatActionResponse{
			ActionType:           record.ActionType,
			NormalizedPayload:    record.NormalizedPayload,
			RequiresConfirmation: record.RequiresConfirmation,
			ResultTitle:          record.ResultTitle,
			ResultSummary:        record.ResultSummary,
			ResultItems:          record.ResultItems,
		})
	}
	return resp
}

func attachProjectChatQualitySummary(ctx context.Context, h *Handler, project db.Project, projectCtx ProjectChatContextResponse, resp *ProjectChatActionResponse, title, content string) {
	if resp == nil || resp.ActionType != "create" {
		return
	}
	req := ProjectChatQualityRequest{
		ProjectTitle:       project.Title,
		ProjectDescription: strings.Join(projectCtx.CreativeAssetSnapshot.HistoricalNotes, "\n"),
		CP:                 deriveProjectChatCP(ProjectChatTitleRequest{ProjectTitle: project.Title}),
		Title:              strings.TrimSpace(title),
		Content:            strings.TrimSpace(content),
	}
	var summary ProjectChatQualitySummary
	if h != nil && h.ProjectChatQuality != nil {
		if live, err := h.ProjectChatQuality.Review(ctx, req); err == nil {
			summary = live
			resp.NormalizedPayload["quality_adapter_status"] = "live"
		} else {
			summary = buildFallbackProjectChatQualitySummary(req)
			resp.NormalizedPayload["quality_adapter_status"] = "fallback"
			resp.NormalizedPayload["quality_adapter_error"] = err.Error()
		}
	} else {
		summary = buildFallbackProjectChatQualitySummary(req)
		resp.NormalizedPayload["quality_adapter_status"] = "fallback"
		resp.NormalizedPayload["quality_adapter_error"] = "lofter quality adapter is not configured"
	}
	if len(summary.Items) > 0 {
		resp.NormalizedPayload["quality_summary"] = summary.Items
	}
}

func extractLatestProjectChatQualitySummary(records []ProjectChatActionRecord) []string {
	for _, record := range records {
		if items := extractStringSlice(record.NormalizedPayload["quality_summary"]); len(items) > 0 {
			return items
		}
	}
	return nil
}

func extractStringSlice(value any) []string {
	raw, ok := value.([]any)
	if ok {
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				out = append(out, strings.TrimSpace(text))
			}
		}
		return out
	}
	if typed, ok := value.([]string); ok {
		return dedupeAndTrimStrings(typed)
	}
	return nil
}

func firstString(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return strings.TrimSpace(items[0])
}

// DeleteChatSession hard-deletes a chat session owned by the caller. The
// row lock + cancel + delete run inside a single tx so a concurrent
// SendChatMessage cannot enqueue a task that would later be orphaned by
// the FK ON DELETE SET NULL on agent_task_queue.chat_session_id. Cancel
// failure aborts the delete; events fire only after commit.
func (h *Handler) DeleteChatSession(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	sessionID := chi.URLParam(r, "sessionId")

	session, ok := h.loadChatSessionForUser(w, r, userID, workspaceID, sessionID)
	if !ok {
		return
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)

	// FOR UPDATE on the chat_session row blocks any concurrent INSERT into
	// agent_task_queue that references it (the FK validation needs a
	// KEY SHARE lock). After we commit the delete, the blocked INSERT
	// fails its FK check, so it can't land an orphaned task.
	if _, err := qtx.LockChatSessionForDelete(r.Context(), session.ID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Already gone — treat as idempotent success.
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to lock chat session")
		return
	}

	cancelled, err := qtx.CancelAgentTasksByChatSession(r.Context(), session.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to cancel chat session tasks")
		return
	}

	if err := qtx.DeleteChatSession(r.Context(), session.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete chat session")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		slog.Warn("commit chat session delete failed", "session_id", sessionID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to commit chat session delete")
		return
	}

	// Post-commit broadcasts. Subscribers should never observe events for a
	// tx that didn't actually persist.
	h.TaskService.BroadcastCancelledTasks(r.Context(), cancelled)

	resolvedSessionID := uuidToString(session.ID)
	h.publishChat(protocol.EventChatSessionDeleted, workspaceID, "member", userID, resolvedSessionID, protocol.ChatSessionDeletedPayload{
		ChatSessionID: resolvedSessionID,
	})

	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Chat Messages
// ---------------------------------------------------------------------------

type SendChatMessageRequest struct {
	Content string `json:"content"`
}

type SendChatMessageResponse struct {
	MessageID string `json:"message_id"`
	TaskID    string `json:"task_id"`
	// CreatedAt anchors the chat StatusPill timer the instant the user
	// hits send. Without it the front-end falls back to its local clock
	// and the timer "snaps backwards" later when WS events deliver the
	// real created_at. Returning it here means the pill renders 0s from
	// the start with a stable anchor.
	CreatedAt string `json:"created_at"`
}

func (h *Handler) SendChatMessage(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	sessionID := chi.URLParam(r, "sessionId")

	var req SendChatMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	// Load chat session.
	session, ok := h.loadChatSessionForUser(w, r, userID, workspaceID, sessionID)
	if !ok {
		return
	}
	// New archive flow doesn't exist anymore, but legacy rows with
	// status='archived' may still be in the DB from before the feature
	// was removed. Refuse to enqueue new agent work for them — frontend
	// surfaces these as read-only.
	if session.Status != "active" {
		writeError(w, http.StatusBadRequest, "chat session is archived")
		return
	}

	// Create the user message first so the daemon can always find it.
	msg, err := h.Queries.CreateChatMessage(r.Context(), db.CreateChatMessageParams{
		ChatSessionID: session.ID,
		Role:          "user",
		Content:       req.Content,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create chat message")
		return
	}

	// Enqueue a chat task after the message exists.
	task, err := h.TaskService.EnqueueChatTask(r.Context(), session)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue chat task: "+err.Error())
		return
	}

	// Touch session updated_at.
	if err := h.Queries.TouchChatSession(r.Context(), session.ID); err != nil {
		slog.Warn("failed to touch chat session", "session_id", sessionID, "error", err)
	}

	// Broadcast the user message.
	resolvedSessionID := uuidToString(session.ID)
	h.publishChat(protocol.EventChatMessage, workspaceID, "member", userID, resolvedSessionID, protocol.ChatMessagePayload{
		ChatSessionID: resolvedSessionID,
		MessageID:     uuidToString(msg.ID),
		Role:          "user",
		Content:       req.Content,
		TaskID:        uuidToString(task.ID),
		CreatedAt:     timestampToString(msg.CreatedAt),
	})

	writeJSON(w, http.StatusCreated, SendChatMessageResponse{
		MessageID: uuidToString(msg.ID),
		TaskID:    uuidToString(task.ID),
		CreatedAt: timestampToString(task.CreatedAt),
	})
}

func (h *Handler) ListChatMessages(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	sessionID := chi.URLParam(r, "sessionId")

	session, ok := h.loadChatSessionForUser(w, r, userID, workspaceID, sessionID)
	if !ok {
		return
	}

	messages, err := h.Queries.ListChatMessages(r.Context(), session.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list chat messages")
		return
	}

	resp := make([]ChatMessageResponse, len(messages))
	for i, m := range messages {
		resp[i] = chatMessageToResponse(m)
	}
	writeJSON(w, http.StatusOK, resp)
}

// PendingChatTaskResponse is returned by GetPendingChatTask — either the
// current in-flight task's id/status, or an empty object when none is active.
// CreatedAt is the anchor the frontend uses to time the chat StatusPill
// (elapsed seconds = now - CreatedAt). It must come from the server because
// optimistic seeds don't have a real task created_at and the timer needs to
// survive refresh / reopen.
type PendingChatTaskResponse struct {
	TaskID    string `json:"task_id,omitempty"`
	Status    string `json:"status,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// MarkChatSessionRead clears the session's unread_since (→ has_unread=false)
// and broadcasts chat:session_read so other devices of the same user drop
// their badges.
func (h *Handler) MarkChatSessionRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	sessionID := chi.URLParam(r, "sessionId")

	session, ok := h.loadChatSessionForUser(w, r, userID, workspaceID, sessionID)
	if !ok {
		return
	}

	if err := h.Queries.MarkChatSessionRead(r.Context(), session.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mark session read")
		return
	}

	resolvedSessionID := uuidToString(session.ID)
	h.publishChat(protocol.EventChatSessionRead, workspaceID, "member", userID, resolvedSessionID, protocol.ChatSessionReadPayload{
		ChatSessionID: resolvedSessionID,
	})

	w.WriteHeader(http.StatusNoContent)
}

// PendingChatTasksResponse is the aggregate view consumed by the FAB.
type PendingChatTasksResponse struct {
	Tasks []PendingChatTaskItem `json:"tasks"`
}

type PendingChatTaskItem struct {
	TaskID        string `json:"task_id"`
	Status        string `json:"status"`
	ChatSessionID string `json:"chat_session_id"`
}

// ListPendingChatTasks returns every in-flight chat task owned by the current
// user in this workspace. Drives the FAB's "running" indicator when the chat
// window is closed (no per-session query is subscribed).
func (h *Handler) ListPendingChatTasks(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())

	rows, err := h.Queries.ListPendingChatTasksByCreator(r.Context(), db.ListPendingChatTasksByCreatorParams{
		WorkspaceID: parseUUID(workspaceID),
		CreatorID:   parseUUID(userID),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list pending chat tasks")
		return
	}

	items := make([]PendingChatTaskItem, len(rows))
	for i, row := range rows {
		items[i] = PendingChatTaskItem{
			TaskID:        uuidToString(row.TaskID),
			Status:        row.Status,
			ChatSessionID: uuidToString(row.ChatSessionID),
		}
	}
	writeJSON(w, http.StatusOK, PendingChatTasksResponse{Tasks: items})
}

// GetPendingChatTask returns the most recent in-flight task (queued / dispatched
// / running) for a chat session. The frontend polls this on mount / session
// switch so pending UI state survives refresh and reopen.
func (h *Handler) GetPendingChatTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	sessionID := chi.URLParam(r, "sessionId")

	session, ok := h.loadChatSessionForUser(w, r, userID, workspaceID, sessionID)
	if !ok {
		return
	}

	task, err := h.Queries.GetPendingChatTask(r.Context(), session.ID)
	if err != nil {
		// No in-flight task — return an empty object, not an error.
		writeJSON(w, http.StatusOK, PendingChatTaskResponse{})
		return
	}

	writeJSON(w, http.StatusOK, PendingChatTaskResponse{
		TaskID:    uuidToString(task.ID),
		Status:    task.Status,
		CreatedAt: timestampToString(task.CreatedAt),
	})
}

// ---------------------------------------------------------------------------
// Task cancellation (user-facing, with ownership check)
// ---------------------------------------------------------------------------

// CancelTaskByUser cancels a task after verifying the requesting user owns
// the associated chat session or issue within the current workspace.
func (h *Handler) CancelTaskByUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	taskID := chi.URLParam(r, "taskId")
	taskUUID, ok := parseUUIDOrBadRequest(w, taskID, "task id")
	if !ok {
		return
	}

	task, err := h.Queries.GetAgentTask(r.Context(), taskUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	// Verify ownership: for chat tasks, check workspace + creator;
	// for issue tasks, verify the issue belongs to the current workspace.
	if task.ChatSessionID.Valid {
		cs, err := h.Queries.GetChatSessionInWorkspace(r.Context(), db.GetChatSessionInWorkspaceParams{
			ID:          task.ChatSessionID,
			WorkspaceID: parseUUID(workspaceID),
		})
		if err != nil {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		if uuidToString(cs.CreatorID) != userID {
			writeError(w, http.StatusForbidden, "not your task")
			return
		}
	} else if task.IssueID.Valid {
		issue, err := h.Queries.GetIssue(r.Context(), task.IssueID)
		if err != nil || uuidToString(issue.WorkspaceID) != workspaceID {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
	} else {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	cancelled, err := h.TaskService.CancelTask(r.Context(), taskUUID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, taskToResponse(*cancelled))
}

// ---------------------------------------------------------------------------
// Response types & helpers
// ---------------------------------------------------------------------------

type ChatSessionResponse struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	AgentID     string `json:"agent_id"`
	CreatorID   string `json:"creator_id"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	// Only populated by list endpoints — single-session fetches return false.
	HasUnread bool   `json:"has_unread"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type ChatMessageResponse struct {
	ID            string  `json:"id"`
	ChatSessionID string  `json:"chat_session_id"`
	Role          string  `json:"role"`
	Content       string  `json:"content"`
	TaskID        *string `json:"task_id"`
	CreatedAt     string  `json:"created_at"`
	// FailureReason flags an assistant row synthesized by FailTask's chat
	// fallback. Front-end uses it to switch to the destructive bubble.
	FailureReason *string `json:"failure_reason"`
	// ElapsedMs is the wall-clock duration from task creation to terminal
	// state. Drives "Replied in 38s" / "Failed after 12s" captions.
	ElapsedMs *int64 `json:"elapsed_ms"`
}

func chatSessionToResponse(s db.ChatSession) ChatSessionResponse {
	return ChatSessionResponse{
		ID:          uuidToString(s.ID),
		WorkspaceID: uuidToString(s.WorkspaceID),
		AgentID:     uuidToString(s.AgentID),
		CreatorID:   uuidToString(s.CreatorID),
		Title:       s.Title,
		Status:      s.Status,
		CreatedAt:   timestampToString(s.CreatedAt),
		UpdatedAt:   timestampToString(s.UpdatedAt),
	}
}

func chatMessageToResponse(m db.ChatMessage) ChatMessageResponse {
	return ChatMessageResponse{
		ID:            uuidToString(m.ID),
		ChatSessionID: uuidToString(m.ChatSessionID),
		Role:          m.Role,
		Content:       m.Content,
		TaskID:        uuidToPtr(m.TaskID),
		CreatedAt:     timestampToString(m.CreatedAt),
		FailureReason: textToPtr(m.FailureReason),
		ElapsedMs:     int8ToPtr(m.ElapsedMs),
	}
}
