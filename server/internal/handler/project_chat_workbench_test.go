package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetProjectChatContext(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/projects?workspace_id="+testWorkspaceID, map[string]any{
		"title":       "LOFTER Chat Project",
		"description": "需要一条更克制的现代 CP 写作链路",
	})
	testHandler.CreateProject(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateProject: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var project ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&project); err != nil {
		t.Fatalf("decode CreateProject: %v", err)
	}
	defer func() {
		r := newRequest("DELETE", "/api/projects/"+project.ID, nil)
		r = withURLParam(r, "id", project.ID)
		testHandler.DeleteProject(httptest.NewRecorder(), r)
	}()

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/projects/"+project.ID+"/resources", map[string]any{
		"resource_type": "github_repo",
		"resource_ref":  map[string]any{"url": "https://github.com/multica-ai/lofter-chat"},
		"label":         "LOFTER source",
	})
	req = withURLParam(req, "id", project.ID)
	testHandler.CreateProjectResource(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateProjectResource: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/chat/projects/"+project.ID+"/context", nil)
	req = withURLParam(req, "projectId", project.ID)
	testHandler.GetProjectChatContext(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetProjectChatContext: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp ProjectChatContextResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode context: %v", err)
	}
	if resp.ProjectTitle != project.Title {
		t.Fatalf("ProjectTitle = %q, want %q", resp.ProjectTitle, project.Title)
	}
	if len(resp.LatestArtifacts) != 1 {
		t.Fatalf("LatestArtifacts len = %d, want 1", len(resp.LatestArtifacts))
	}
	if len(resp.NextRecommendedActions) == 0 {
		t.Fatal("expected next recommended actions")
	}
}

func TestRunProjectChatActionOperateIsolation(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/projects?workspace_id="+testWorkspaceID, map[string]any{
		"title": "Operate project",
	})
	testHandler.CreateProject(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateProject: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var project ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&project); err != nil {
		t.Fatalf("decode CreateProject: %v", err)
	}
	defer func() {
		r := newRequest("DELETE", "/api/projects/"+project.ID, nil)
		r = withURLParam(r, "id", project.ID)
		testHandler.DeleteProject(httptest.NewRecorder(), r)
	}()

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/chat/projects/"+project.ID+"/actions", map[string]any{
		"input_text": "把这篇发掉",
	})
	req = withURLParam(req, "projectId", project.ID)
	testHandler.RunProjectChatAction(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("RunProjectChatAction: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp ProjectChatActionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode action: %v", err)
	}
	if resp.ActionType != "operate" {
		t.Fatalf("ActionType = %q, want operate", resp.ActionType)
	}
	if resp.OperateHandoff == nil {
		t.Fatal("expected operate handoff payload")
	}
	if resp.OperateHandoff.Destination != "operations-console" {
		t.Fatalf("Destination = %q, want operations-console", resp.OperateHandoff.Destination)
	}
}

func TestRunProjectChatActionShapeRequiresConfirmation(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/projects?workspace_id="+testWorkspaceID, map[string]any{
		"title": "Shape project",
	})
	testHandler.CreateProject(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateProject: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var project ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&project); err != nil {
		t.Fatalf("decode CreateProject: %v", err)
	}
	defer func() {
		r := newRequest("DELETE", "/api/projects/"+project.ID, nil)
		r = withURLParam(r, "id", project.ID)
		testHandler.DeleteProject(httptest.NewRecorder(), r)
	}()

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/chat/projects/"+project.ID+"/actions", map[string]any{
		"input_text": "以后标题更克制一点",
	})
	req = withURLParam(req, "projectId", project.ID)
	testHandler.RunProjectChatAction(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("RunProjectChatAction: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp ProjectChatActionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode action: %v", err)
	}
	if resp.ActionType != "shape" {
		t.Fatalf("ActionType = %q, want shape", resp.ActionType)
	}
	if !resp.RequiresConfirmation {
		t.Fatal("expected shape action to require confirmation")
	}
	if resp.AssetPatchPreview == nil {
		t.Fatal("expected asset patch preview")
	}
}

type stubProjectChatTitleGenerator struct {
	titles []string
	err    error
}

type stubProjectChatAssetStore struct {
	snapshot ProjectCreativeAssetSnapshotResponse
}

type stubProjectChatCreateAdapter struct {
	rewrittenTitle string
	deaiText       string
	appliedRules   []string
	rewriteErr     error
	deaiErr        error
}

type stubProjectChatRecordStore struct {
	actions   []ProjectChatActionRecord
	artifacts []ProjectChatArtifactRecord
}

type stubProjectChatQualityAdapter struct {
	summary ProjectChatQualitySummary
	err     error
}

func (s *stubProjectChatAssetStore) Load(_ string, _ string) (ProjectCreativeAssetSnapshotResponse, error) {
	return normalizeProjectCreativeAssetSnapshot(s.snapshot), nil
}

func (s *stubProjectChatAssetStore) ApplyPatch(_ string, _ string, target, patch string) (ProjectCreativeAssetSnapshotResponse, error) {
	target = strings.TrimSpace(target)
	patch = strings.TrimSpace(patch)
	switch target {
	case "title_preferences":
		s.snapshot.TitlePreferences = appendUniqueString(s.snapshot.TitlePreferences, patch)
	case "style_examples":
		s.snapshot.StyleExamples = appendUniqueString(s.snapshot.StyleExamples, patch)
	default:
		s.snapshot.ShapePreferences = appendUniqueString(s.snapshot.ShapePreferences, patch)
	}
	s.snapshot = normalizeProjectCreativeAssetSnapshot(s.snapshot)
	return s.snapshot, nil
}

func (s stubProjectChatTitleGenerator) GenerateTitles(_ context.Context, _ ProjectChatTitleRequest) ([]string, error) {
	return s.titles, s.err
}

func (s stubProjectChatCreateAdapter) RewriteTitle(_ context.Context, _ ProjectChatRewriteTitleRequest) (string, error) {
	return s.rewrittenTitle, s.rewriteErr
}

func (s stubProjectChatCreateAdapter) DeAI(_ context.Context, _ ProjectChatDeAIRequest) (string, []string, error) {
	return s.deaiText, s.appliedRules, s.deaiErr
}

func (s *stubProjectChatRecordStore) ListActions(_ string, _ string) ([]ProjectChatActionRecord, error) {
	out := make([]ProjectChatActionRecord, len(s.actions))
	copy(out, s.actions)
	return out, nil
}

func (s *stubProjectChatRecordStore) AppendAction(_ string, _ string, record ProjectChatActionRecord) error {
	s.actions = append(s.actions, normalizeProjectChatActionRecord(record))
	return nil
}

func (s *stubProjectChatRecordStore) ListArtifacts(_ string, _ string) ([]ProjectChatArtifactRecord, error) {
	out := make([]ProjectChatArtifactRecord, len(s.artifacts))
	copy(out, s.artifacts)
	return out, nil
}

func (s *stubProjectChatRecordStore) AppendArtifacts(_ string, _ string, records []ProjectChatArtifactRecord) error {
	for _, record := range records {
		s.artifacts = append(s.artifacts, normalizeProjectChatArtifactRecord(record))
	}
	return nil
}

func (s stubProjectChatQualityAdapter) Review(_ context.Context, _ ProjectChatQualityRequest) (ProjectChatQualitySummary, error) {
	return s.summary, s.err
}

func TestApplyProjectChatAssetPatchPersistsAndReadsBackContext(t *testing.T) {
	original := testHandler.ProjectChatAssets
	store := &stubProjectChatAssetStore{}
	testHandler.ProjectChatAssets = store
	defer func() { testHandler.ProjectChatAssets = original }()

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/projects?workspace_id="+testWorkspaceID, map[string]any{
		"title":       "Shape project",
		"description": "现代都市背景，想保留拉扯感。",
	})
	testHandler.CreateProject(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateProject: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var project ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&project); err != nil {
		t.Fatalf("decode CreateProject: %v", err)
	}
	defer func() {
		r := newRequest("DELETE", "/api/projects/"+project.ID, nil)
		r = withURLParam(r, "id", project.ID)
		testHandler.DeleteProject(httptest.NewRecorder(), r)
	}()

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/chat/projects/"+project.ID+"/assets/patch", map[string]any{
		"asset_target": "title_preferences",
		"patch":        "标题更克制一点，不要太狗血",
	})
	req = withURLParam(req, "projectId", project.ID)
	testHandler.ApplyProjectChatAssetPatch(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ApplyProjectChatAssetPatch: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var patchResp ApplyProjectChatAssetPatchResponse
	if err := json.NewDecoder(w.Body).Decode(&patchResp); err != nil {
		t.Fatalf("decode patch response: %v", err)
	}
	if len(patchResp.UpdatedAssetSnapshot.TitlePreferences) != 1 {
		t.Fatalf("TitlePreferences len = %d, want 1", len(patchResp.UpdatedAssetSnapshot.TitlePreferences))
	}

	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/chat/projects/"+project.ID+"/context", nil)
	req = withURLParam(req, "projectId", project.ID)
	testHandler.GetProjectChatContext(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetProjectChatContext: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var contextResp ProjectChatContextResponse
	if err := json.NewDecoder(w.Body).Decode(&contextResp); err != nil {
		t.Fatalf("decode context: %v", err)
	}
	if len(contextResp.CreativeAssetSnapshot.TitlePreferences) != 1 {
		t.Fatalf("context title_preferences len = %d, want 1", len(contextResp.CreativeAssetSnapshot.TitlePreferences))
	}
	if contextResp.CreativeAssetSnapshot.TitlePreferences[0] != "标题更克制一点，不要太狗血" {
		t.Fatalf("title_preferences[0] = %q", contextResp.CreativeAssetSnapshot.TitlePreferences[0])
	}
	if len(contextResp.CreativeAssetSnapshot.HistoricalNotes) != 1 {
		t.Fatalf("historical_notes len = %d, want 1", len(contextResp.CreativeAssetSnapshot.HistoricalNotes))
	}
}

func TestRunProjectChatActionCreateUsesLiveTitleAdapter(t *testing.T) {
	original := testHandler.ProjectChatTitles
	testHandler.ProjectChatTitles = stubProjectChatTitleGenerator{
		titles: []string{
			"【朝俞】他都收手了，贺朝还在装不在意",
			"【朝俞】明明说好克制，结果先失控的是他",
		},
	}
	defer func() { testHandler.ProjectChatTitles = original }()

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/projects?workspace_id="+testWorkspaceID, map[string]any{
		"title":       "朝俞 LOFTER",
		"description": "现代都市校园向，想要更克制但仍有拉扯感。",
	})
	testHandler.CreateProject(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateProject: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var project ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&project); err != nil {
		t.Fatalf("decode CreateProject: %v", err)
	}
	defer func() {
		r := newRequest("DELETE", "/api/projects/"+project.ID, nil)
		r = withURLParam(r, "id", project.ID)
		testHandler.DeleteProject(httptest.NewRecorder(), r)
	}()

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/chat/projects/"+project.ID+"/actions", map[string]any{
		"input_text": "给我 3 个更克制一点的标题方向",
	})
	req = withURLParam(req, "projectId", project.ID)
	testHandler.RunProjectChatAction(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("RunProjectChatAction: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp ProjectChatActionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode action: %v", err)
	}
	if resp.ActionType != "create" {
		t.Fatalf("ActionType = %q, want create", resp.ActionType)
	}
	if got := resp.NormalizedPayload["adapter_status"]; got != "live" {
		t.Fatalf("adapter_status = %v, want live", got)
	}
	if len(resp.ResultItems) != 2 {
		t.Fatalf("ResultItems len = %d, want 2", len(resp.ResultItems))
	}
}

func TestRunProjectChatActionCreateFallsBackWhenTitleAdapterFails(t *testing.T) {
	original := testHandler.ProjectChatTitles
	testHandler.ProjectChatTitles = stubProjectChatTitleGenerator{
		err: errors.New("boom"),
	}
	defer func() { testHandler.ProjectChatTitles = original }()

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/projects?workspace_id="+testWorkspaceID, map[string]any{
		"title":       "朝俞 LOFTER",
		"description": "现代都市校园向，想要更克制但仍有拉扯感。",
	})
	testHandler.CreateProject(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateProject: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var project ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&project); err != nil {
		t.Fatalf("decode CreateProject: %v", err)
	}
	defer func() {
		r := newRequest("DELETE", "/api/projects/"+project.ID, nil)
		r = withURLParam(r, "id", project.ID)
		testHandler.DeleteProject(httptest.NewRecorder(), r)
	}()

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/chat/projects/"+project.ID+"/actions", map[string]any{
		"input_text": "给我 3 个更克制一点的标题方向",
	})
	req = withURLParam(req, "projectId", project.ID)
	testHandler.RunProjectChatAction(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("RunProjectChatAction: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp ProjectChatActionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode action: %v", err)
	}
	if got := resp.NormalizedPayload["adapter_status"]; got != "fallback" {
		t.Fatalf("adapter_status = %v, want fallback", got)
	}
	if len(resp.ResultItems) == 0 {
		t.Fatal("expected fallback titles")
	}
}

func TestRunProjectChatActionCreateRewriteTitleUsesLiveAdapter(t *testing.T) {
	original := testHandler.ProjectChatCreate
	originalQuality := testHandler.ProjectChatQuality
	testHandler.ProjectChatCreate = stubProjectChatCreateAdapter{
		rewrittenTitle: "【朝俞】嘴上说收住了，贺朝却先乱了分寸",
	}
	testHandler.ProjectChatQuality = stubProjectChatQualityAdapter{
		summary: ProjectChatQualitySummary{Items: []string{"标题质量: 标题和正文主事件基本一致"}},
	}
	defer func() { testHandler.ProjectChatCreate = original }()
	defer func() { testHandler.ProjectChatQuality = originalQuality }()

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/projects?workspace_id="+testWorkspaceID, map[string]any{
		"title":       "朝俞 LOFTER",
		"description": "想把标题收得更克制。",
	})
	testHandler.CreateProject(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateProject: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var project ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&project); err != nil {
		t.Fatalf("decode CreateProject: %v", err)
	}
	defer func() {
		r := newRequest("DELETE", "/api/projects/"+project.ID, nil)
		r = withURLParam(r, "id", project.ID)
		testHandler.DeleteProject(httptest.NewRecorder(), r)
	}()

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/chat/projects/"+project.ID+"/actions", map[string]any{
		"input_text": "把这个标题“【朝俞】他都收手了，贺朝还在装不在意”改得更克制一点",
	})
	req = withURLParam(req, "projectId", project.ID)
	testHandler.RunProjectChatAction(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("RunProjectChatAction: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp ProjectChatActionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode action: %v", err)
	}
	if resp.ActionType != "create" {
		t.Fatalf("ActionType = %q, want create", resp.ActionType)
	}
	if got := resp.NormalizedPayload["mode"]; got != "rewrite_title" {
		t.Fatalf("mode = %v, want rewrite_title", got)
	}
	if got := resp.NormalizedPayload["adapter_status"]; got != "live" {
		t.Fatalf("adapter_status = %v, want live", got)
	}
	if len(resp.ResultItems) != 1 || resp.ResultItems[0] != "【朝俞】嘴上说收住了，贺朝却先乱了分寸" {
		t.Fatalf("unexpected result items: %#v", resp.ResultItems)
	}
	if got := extractStringSlice(resp.NormalizedPayload["quality_summary"]); len(got) == 0 {
		t.Fatal("expected quality_summary on create response")
	}
}

func TestRunProjectChatActionCreateDeAIUsesLiveAdapter(t *testing.T) {
	original := testHandler.ProjectChatCreate
	testHandler.ProjectChatCreate = stubProjectChatCreateAdapter{
		deaiText:     "贺朝顿了顿，最后还是把那句重话收了回去。",
		appliedRules: []string{"删除词语: 1处", "黑名单整句删除: 1句"},
	}
	defer func() { testHandler.ProjectChatCreate = original }()

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/projects?workspace_id="+testWorkspaceID, map[string]any{
		"title":       "朝俞 LOFTER",
		"description": "想把段落收得更自然。",
	})
	testHandler.CreateProject(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateProject: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var project ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&project); err != nil {
		t.Fatalf("decode CreateProject: %v", err)
	}
	defer func() {
		r := newRequest("DELETE", "/api/projects/"+project.ID, nil)
		r = withURLParam(r, "id", project.ID)
		testHandler.DeleteProject(httptest.NewRecorder(), r)
	}()

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/chat/projects/"+project.ID+"/actions", map[string]any{
		"input_text": "把这段去 AI 味：“空气仿佛凝固，贺朝喉咙发紧，眼神动了动。”",
	})
	req = withURLParam(req, "projectId", project.ID)
	testHandler.RunProjectChatAction(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("RunProjectChatAction: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp ProjectChatActionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode action: %v", err)
	}
	if resp.ActionType != "create" {
		t.Fatalf("ActionType = %q, want create", resp.ActionType)
	}
	if got := resp.NormalizedPayload["mode"]; got != "deai_rewrite" {
		t.Fatalf("mode = %v, want deai_rewrite", got)
	}
	if got := resp.NormalizedPayload["adapter_status"]; got != "live" {
		t.Fatalf("adapter_status = %v, want live", got)
	}
	if len(resp.ResultItems) != 1 || resp.ResultItems[0] != "贺朝顿了顿，最后还是把那句重话收了回去。" {
		t.Fatalf("unexpected result items: %#v", resp.ResultItems)
	}
}

func TestRunProjectChatActionCreatePersistsReadModelIntoContext(t *testing.T) {
	originalCreate := testHandler.ProjectChatCreate
	originalRecords := testHandler.ProjectChatRecords
	originalQuality := testHandler.ProjectChatQuality
	testHandler.ProjectChatCreate = stubProjectChatCreateAdapter{
		rewrittenTitle: "【朝俞】嘴上说收住了，贺朝却先乱了分寸",
	}
	testHandler.ProjectChatQuality = stubProjectChatQualityAdapter{
		summary: ProjectChatQualitySummary{Items: []string{
			"标题质量: 标题和正文主事件基本一致",
			"AI 味风险: 低，暂未命中明显规则",
		}},
	}
	recordStore := &stubProjectChatRecordStore{}
	testHandler.ProjectChatRecords = recordStore
	defer func() {
		testHandler.ProjectChatCreate = originalCreate
		testHandler.ProjectChatRecords = originalRecords
		testHandler.ProjectChatQuality = originalQuality
	}()

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/projects?workspace_id="+testWorkspaceID, map[string]any{
		"title":       "朝俞 LOFTER",
		"description": "想把标题收得更克制。",
	})
	testHandler.CreateProject(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateProject: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var project ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&project); err != nil {
		t.Fatalf("decode CreateProject: %v", err)
	}
	defer func() {
		r := newRequest("DELETE", "/api/projects/"+project.ID, nil)
		r = withURLParam(r, "id", project.ID)
		testHandler.DeleteProject(httptest.NewRecorder(), r)
	}()

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/chat/projects/"+project.ID+"/actions", map[string]any{
		"input_text": "把这个标题“【朝俞】他都收手了，贺朝还在装不在意”改得更克制一点",
	})
	req = withURLParam(req, "projectId", project.ID)
	testHandler.RunProjectChatAction(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("RunProjectChatAction: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if len(recordStore.actions) != 1 {
		t.Fatalf("actions len = %d, want 1", len(recordStore.actions))
	}
	if len(recordStore.artifacts) != 1 {
		t.Fatalf("artifacts len = %d, want 1", len(recordStore.artifacts))
	}

	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/chat/projects/"+project.ID+"/context", nil)
	req = withURLParam(req, "projectId", project.ID)
	testHandler.GetProjectChatContext(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetProjectChatContext: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp ProjectChatContextResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode context: %v", err)
	}
	if len(resp.RecentActions) != 1 {
		t.Fatalf("recent_actions len = %d, want 1", len(resp.RecentActions))
	}
	if len(resp.LatestReviewSummary) == 0 {
		t.Fatal("expected latest_review_summary")
	}
	if resp.LatestReviewSummary[0] != "标题质量: 标题和正文主事件基本一致" {
		t.Fatalf("latest_review_summary[0] = %q", resp.LatestReviewSummary[0])
	}
	if resp.RecentActions[0].ResultTitle != "Create 标题改写" {
		t.Fatalf("recent action title = %q", resp.RecentActions[0].ResultTitle)
	}
	if len(resp.LatestArtifacts) == 0 {
		t.Fatal("expected latest_artifacts to include persisted generated artifact")
	}
	if resp.LatestArtifacts[0].Summary != "Create 标题改写" {
		t.Fatalf("latest artifact summary = %q", resp.LatestArtifacts[0].Summary)
	}
}
