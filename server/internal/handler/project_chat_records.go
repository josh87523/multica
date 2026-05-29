package handler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ProjectChatArtifactRecord struct {
	ArtifactID    string `json:"artifact_id"`
	ProjectID     string `json:"project_id"`
	Stage         string `json:"stage"`
	ArtifactType  string `json:"artifact_type"`
	SourceAdapter string `json:"source_adapter"`
	Path          string `json:"path"`
	Summary       string `json:"summary"`
	Content       string `json:"content"`
	CreatedAt     string `json:"created_at"`
}

type ProjectChatActionRecord struct {
	ActionID             string         `json:"action_id"`
	ProjectID            string         `json:"project_id"`
	ActionType           string         `json:"action_type"`
	Target               string         `json:"target"`
	InputText            string         `json:"input_text"`
	NormalizedPayload    map[string]any `json:"normalized_payload"`
	Status               string         `json:"status"`
	ResultRefs           []string       `json:"result_refs"`
	RequiresConfirmation bool           `json:"requires_confirmation"`
	ResultTitle          string         `json:"result_title"`
	ResultSummary        string         `json:"result_summary"`
	ResultItems          []string       `json:"result_items"`
	CreatedAt            string         `json:"created_at"`
}

type projectChatRecordStore interface {
	ListActions(workspaceID, projectID string) ([]ProjectChatActionRecord, error)
	AppendAction(workspaceID, projectID string, record ProjectChatActionRecord) error
	ListArtifacts(workspaceID, projectID string) ([]ProjectChatArtifactRecord, error)
	AppendArtifacts(workspaceID, projectID string, records []ProjectChatArtifactRecord) error
}

type fileProjectChatRecordStore struct {
	rootDir string
}

func newProjectChatRecordStore() projectChatRecordStore {
	return &fileProjectChatRecordStore{
		rootDir: resolveProjectChatAssetStoreRoot(),
	}
}

func (s *fileProjectChatRecordStore) ListActions(workspaceID, projectID string) ([]ProjectChatActionRecord, error) {
	path := s.actionsPath(workspaceID, projectID)
	var records []ProjectChatActionRecord
	if err := loadProjectChatJSON(path, &records); err != nil {
		return nil, err
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt > records[j].CreatedAt
	})
	return records, nil
}

func (s *fileProjectChatRecordStore) AppendAction(workspaceID, projectID string, record ProjectChatActionRecord) error {
	path := s.actionsPath(workspaceID, projectID)
	var records []ProjectChatActionRecord
	if err := loadProjectChatJSON(path, &records); err != nil {
		return err
	}
	records = append(records, normalizeProjectChatActionRecord(record))
	return writeProjectChatJSON(path, records)
}

func (s *fileProjectChatRecordStore) ListArtifacts(workspaceID, projectID string) ([]ProjectChatArtifactRecord, error) {
	path := s.artifactsPath(workspaceID, projectID)
	var records []ProjectChatArtifactRecord
	if err := loadProjectChatJSON(path, &records); err != nil {
		return nil, err
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt > records[j].CreatedAt
	})
	return records, nil
}

func (s *fileProjectChatRecordStore) AppendArtifacts(workspaceID, projectID string, newRecords []ProjectChatArtifactRecord) error {
	path := s.artifactsPath(workspaceID, projectID)
	var records []ProjectChatArtifactRecord
	if err := loadProjectChatJSON(path, &records); err != nil {
		return err
	}
	for _, record := range newRecords {
		records = append(records, normalizeProjectChatArtifactRecord(record))
	}
	return writeProjectChatJSON(path, records)
}

func (s *fileProjectChatRecordStore) actionsPath(workspaceID, projectID string) string {
	return filepath.Join(s.rootDir, "actions", workspaceID, fmt.Sprintf("%s.json", projectID))
}

func (s *fileProjectChatRecordStore) artifactsPath(workspaceID, projectID string) string {
	return filepath.Join(s.rootDir, "artifacts", workspaceID, fmt.Sprintf("%s.json", projectID))
}

func loadProjectChatJSON(path string, dst any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dst)
}

func writeProjectChatJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func normalizeProjectChatArtifactRecord(record ProjectChatArtifactRecord) ProjectChatArtifactRecord {
	record.ArtifactID = strings.TrimSpace(record.ArtifactID)
	record.ProjectID = strings.TrimSpace(record.ProjectID)
	record.Stage = strings.TrimSpace(record.Stage)
	record.ArtifactType = strings.TrimSpace(record.ArtifactType)
	record.SourceAdapter = strings.TrimSpace(record.SourceAdapter)
	record.Path = strings.TrimSpace(record.Path)
	record.Summary = strings.TrimSpace(record.Summary)
	record.Content = strings.TrimSpace(record.Content)
	record.CreatedAt = ensureProjectChatTimestamp(record.CreatedAt)
	return record
}

func normalizeProjectChatActionRecord(record ProjectChatActionRecord) ProjectChatActionRecord {
	record.ActionID = strings.TrimSpace(record.ActionID)
	record.ProjectID = strings.TrimSpace(record.ProjectID)
	record.ActionType = strings.TrimSpace(record.ActionType)
	record.Target = strings.TrimSpace(record.Target)
	record.InputText = strings.TrimSpace(record.InputText)
	record.Status = strings.TrimSpace(record.Status)
	record.ResultRefs = dedupeAndTrimStrings(record.ResultRefs)
	record.ResultTitle = strings.TrimSpace(record.ResultTitle)
	record.ResultSummary = strings.TrimSpace(record.ResultSummary)
	record.ResultItems = dedupeAndTrimStrings(record.ResultItems)
	record.CreatedAt = ensureProjectChatTimestamp(record.CreatedAt)
	if record.NormalizedPayload == nil {
		record.NormalizedPayload = map[string]any{}
	}
	return record
}

func ensureProjectChatTimestamp(value string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return time.Now().UTC().Format(time.RFC3339)
}
