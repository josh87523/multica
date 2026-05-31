package handler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type projectChatAssetStore interface {
	Load(workspaceID, projectID string) (ProjectCreativeAssetSnapshotResponse, error)
	ApplyPatch(workspaceID, projectID, target, patch string) (ProjectCreativeAssetSnapshotResponse, error)
}

type fileProjectChatAssetStore struct {
	rootDir string
}

func newProjectChatAssetStore() projectChatAssetStore {
	return &fileProjectChatAssetStore{
		rootDir: resolveProjectChatAssetStoreRoot(),
	}
}

func resolveProjectChatAssetStoreRoot() string {
	if v := strings.TrimSpace(os.Getenv("MULTICA_PROJECT_CHAT_DATA_DIR")); v != "" {
		return v
	}
	return filepath.Join("data", "workbench", "project-chat")
}

func (s *fileProjectChatAssetStore) Load(workspaceID, projectID string) (ProjectCreativeAssetSnapshotResponse, error) {
	path := s.filePath(workspaceID, projectID)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyProjectCreativeAssetSnapshot(), nil
		}
		return ProjectCreativeAssetSnapshotResponse{}, err
	}
	var snapshot ProjectCreativeAssetSnapshotResponse
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return ProjectCreativeAssetSnapshotResponse{}, err
	}
	return normalizeProjectCreativeAssetSnapshot(snapshot), nil
}

func (s *fileProjectChatAssetStore) ApplyPatch(workspaceID, projectID, target, patch string) (ProjectCreativeAssetSnapshotResponse, error) {
	snapshot, err := s.Load(workspaceID, projectID)
	if err != nil {
		return ProjectCreativeAssetSnapshotResponse{}, err
	}
	patch = strings.TrimSpace(patch)
	target = strings.TrimSpace(target)
	if patch == "" {
		return snapshot, nil
	}
	switch target {
	case "title_preferences":
		snapshot.TitlePreferences = appendUniqueString(snapshot.TitlePreferences, patch)
	case "style_examples":
		snapshot.StyleExamples = appendUniqueString(snapshot.StyleExamples, patch)
	default:
		snapshot.ShapePreferences = appendUniqueString(snapshot.ShapePreferences, patch)
	}
	snapshot = normalizeProjectCreativeAssetSnapshot(snapshot)
	if err := os.MkdirAll(filepath.Dir(s.filePath(workspaceID, projectID)), 0o755); err != nil {
		return ProjectCreativeAssetSnapshotResponse{}, err
	}
	raw, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return ProjectCreativeAssetSnapshotResponse{}, err
	}
	if err := os.WriteFile(s.filePath(workspaceID, projectID), raw, 0o644); err != nil {
		return ProjectCreativeAssetSnapshotResponse{}, err
	}
	return snapshot, nil
}

func (s *fileProjectChatAssetStore) filePath(workspaceID, projectID string) string {
	return filepath.Join(s.rootDir, workspaceID, fmt.Sprintf("%s.json", projectID))
}

func emptyProjectCreativeAssetSnapshot() ProjectCreativeAssetSnapshotResponse {
	return ProjectCreativeAssetSnapshotResponse{
		StyleExamples:    []string{},
		TitlePreferences: []string{},
		ShapePreferences: []string{},
		HistoricalNotes:  []string{},
	}
}

func normalizeProjectCreativeAssetSnapshot(snapshot ProjectCreativeAssetSnapshotResponse) ProjectCreativeAssetSnapshotResponse {
	snapshot.StyleExamples = dedupeAndTrimStrings(snapshot.StyleExamples)
	snapshot.TitlePreferences = dedupeAndTrimStrings(snapshot.TitlePreferences)
	snapshot.ShapePreferences = dedupeAndTrimStrings(snapshot.ShapePreferences)
	snapshot.HistoricalNotes = dedupeAndTrimStrings(snapshot.HistoricalNotes)
	return snapshot
}

func appendUniqueString(items []string, value string) []string {
	items = append(items, value)
	return dedupeAndTrimStrings(items)
}

func appendUniqueStrings(items []string, values ...string) []string {
	items = append(items, values...)
	return dedupeAndTrimStrings(items)
}

func dedupeAndTrimStrings(items []string) []string {
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
