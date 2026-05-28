package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	issueCommentWebhookDispatchAttempts = 3
	issueCommentWebhookRetryBaseDelay   = 50 * time.Millisecond
)

type issueCommentWebhookPayload struct {
	SchemaVersion string                   `json:"schema_version"`
	Event         string                   `json:"event"`
	Issue         issueCommentWebhookIssue `json:"issue"`
	Comment       issueCommentWebhookItem  `json:"comment"`
	Command       string                   `json:"command"`
}

type issueCommentWebhookIssue struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	Identifier  string  `json:"identifier"`
	Title       string  `json:"title"`
	Status      string  `json:"status"`
	Description *string `json:"description,omitempty"`
}

type issueCommentWebhookItem struct {
	ID         string  `json:"id"`
	Content    string  `json:"content"`
	Type       string  `json:"type"`
	AuthorType string  `json:"author_type"`
	AuthorID   string  `json:"author_id"`
	ParentID   *string `json:"parent_id,omitempty"`
	CreatedAt  string  `json:"created_at"`
}

func parseIssueOrchestrationCommand(content string) (string, bool) {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "/orchestrate") {
		return "", false
	}
	fields := strings.Fields(trimmed)
	if len(fields) < 2 {
		return "", false
	}
	return strings.TrimSpace(fields[1]), true
}

func (h *Handler) dispatchIssueCommentWebhook(ctx context.Context, issue db.Issue, comment db.Comment, authorType string, authorID string, parentID *string) {
	webhookURL := strings.TrimSpace(os.Getenv("MULTICA_ISSUE_COMMENT_WEBHOOK_URL"))
	if webhookURL == "" {
		return
	}
	command, ok := parseIssueOrchestrationCommand(comment.Content)
	if !ok {
		return
	}
	payloadBytes, _ := json.Marshal(issueCommentWebhookPayload{
		SchemaVersion: "multica_issue_comment_webhook_v1",
		Event:         "issue_comment_command",
		Command:       command,
		Issue: issueCommentWebhookIssue{
			ID:          uuidToString(issue.ID),
			WorkspaceID: uuidToString(issue.WorkspaceID),
			Identifier:  h.getIssuePrefix(ctx, issue.WorkspaceID) + "-" + strconv.Itoa(int(issue.Number)),
			Title:       issue.Title,
			Status:      issue.Status,
			Description: textToPtr(issue.Description),
		},
		Comment: issueCommentWebhookItem{
			ID:         uuidToString(comment.ID),
			Content:    comment.Content,
			Type:       comment.Type,
			AuthorType: authorType,
			AuthorID:   authorID,
			ParentID:   parentID,
			CreatedAt:  timestampToString(comment.CreatedAt),
		},
	})
	go func() {
		var lastErr string
		for attempt := 1; attempt <= issueCommentWebhookDispatchAttempts; attempt++ {
			reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, webhookURL, bytes.NewReader(payloadBytes))
			if err != nil {
				cancel()
				lastErr = err.Error()
			} else {
				req.Header.Set("Content-Type", "application/json")
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					lastErr = err.Error()
				} else {
					resp.Body.Close()
					if resp.StatusCode >= 200 && resp.StatusCode < 300 {
						cancel()
						return
					}
					lastErr = resp.Status
				}
				cancel()
			}
			if attempt < issueCommentWebhookDispatchAttempts {
				time.Sleep(time.Duration(attempt) * issueCommentWebhookRetryBaseDelay)
			}
		}
		slog.Warn("issue comment webhook dispatch failed",
			"issue_id", uuidToString(issue.ID),
			"comment_id", uuidToString(comment.ID),
			"command", command,
			"error", lastErr,
		)
	}()
}
