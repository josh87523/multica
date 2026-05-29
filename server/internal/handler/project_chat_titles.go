package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type ProjectChatTitleRequest struct {
	CP                 string
	ProjectTitle       string
	ProjectDescription string
	InputText          string
	ReferenceTitle     string
}

type projectChatTitleGenerator interface {
	GenerateTitles(ctx context.Context, req ProjectChatTitleRequest) ([]string, error)
}

type projectChatCommandRunner func(ctx context.Context, name string, args []string, env []string, dir string, stdin []byte) ([]byte, error)

type pythonProjectChatTitleGenerator struct {
	botRoot string
	runCmd  projectChatCommandRunner
}

func newProjectChatTitleGenerator() projectChatTitleGenerator {
	return &pythonProjectChatTitleGenerator{
		botRoot: resolveProjectChatBotRoot(),
		runCmd:  runProjectChatCommand,
	}
}

func resolveProjectChatBotRoot() string {
	if v := strings.TrimSpace(os.Getenv("MULTICA_LOFTER_BOT_ROOT")); v != "" {
		return v
	}
	candidates := []string{
		"/Users/obayotian/Workspace/HanxuKeji/lofter_bot",
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(candidate, "generation_core.py")); err == nil {
			return candidate
		}
	}
	return ""
}

func (g *pythonProjectChatTitleGenerator) GenerateTitles(ctx context.Context, req ProjectChatTitleRequest) ([]string, error) {
	if g == nil || strings.TrimSpace(g.botRoot) == "" {
		return nil, errors.New("lofter title adapter is not configured")
	}
	if _, err := os.Stat(filepath.Join(g.botRoot, "generation_core.py")); err != nil {
		return nil, fmt.Errorf("lofter title adapter unavailable: %w", err)
	}

	payload := map[string]string{
		"cp":              deriveProjectChatCP(req),
		"content":         buildProjectChatTitleContent(req),
		"reference_title": strings.TrimSpace(req.ReferenceTitle),
	}
	stdin, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := g.runCmd(
		timeoutCtx,
		"python3",
		[]string{"-c", projectChatTitlePythonEntrypoint},
		append(os.Environ(), "PYTHONPATH="+g.botRoot),
		g.botRoot,
		stdin,
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Titles []string `json:"titles"`
	}
	if err := decodeProjectChatTitleResponse(out, &resp); err != nil {
		return nil, err
	}
	cleaned := make([]string, 0, len(resp.Titles))
	for _, title := range resp.Titles {
		title = strings.TrimSpace(title)
		if title != "" {
			cleaned = append(cleaned, title)
		}
	}
	if len(cleaned) == 0 {
		return nil, errors.New("lofter title adapter returned no titles")
	}
	return cleaned, nil
}

func runProjectChatCommand(ctx context.Context, name string, args []string, env []string, dir string, stdin []byte) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = bytes.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s %v failed: %w: %s", name, args, err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func decodeProjectChatTitleResponse(raw []byte, dst any) error {
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if err := json.Unmarshal([]byte(line), dst); err == nil {
			return nil
		}
	}
	return fmt.Errorf("failed to decode title adapter response: %s", strings.TrimSpace(string(raw)))
}

func deriveProjectChatCP(req ProjectChatTitleRequest) string {
	candidate := strings.TrimSpace(req.CP)
	if candidate == "" {
		candidate = strings.TrimSpace(req.ProjectTitle)
	}
	replacements := []string{
		"LOFTER", "lofter", "Lofter",
		"纯聊天", "写作", "workbench", "chat",
	}
	for _, item := range replacements {
		candidate = strings.ReplaceAll(candidate, item, "")
	}
	candidate = strings.Trim(candidate, " -_:|/[]【】")
	if parts := strings.Fields(candidate); len(parts) > 0 {
		candidate = parts[0]
	}
	if candidate == "" {
		candidate = "LOFTER"
	}
	return candidate
}

func buildProjectChatTitleContent(req ProjectChatTitleRequest) string {
	parts := []string{
		strings.TrimSpace(req.ProjectDescription),
		strings.TrimSpace(req.InputText),
	}
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	if len(filtered) == 0 {
		return strings.TrimSpace(req.ProjectTitle)
	}
	return strings.Join(filtered, "\n\n")
}

func buildFallbackProjectChatTitles(req ProjectChatTitleRequest) []string {
	cp := deriveProjectChatCP(req)
	focus := strings.TrimSpace(req.InputText)
	cleanup := []string{
		"给我", "生成", "标题", "候选", "版本", "三个", "3个", "再来", "一些",
	}
	for _, item := range cleanup {
		focus = strings.ReplaceAll(focus, item, "")
	}
	focus = strings.TrimSpace(strings.Trim(focus, "，。,:：；!?！？"))
	if focus == "" {
		focus = strings.TrimSpace(req.ProjectDescription)
	}
	if focus == "" {
		focus = "新的情绪钩子"
	}
	body := []string{
		focus,
		"这次轮到他先失控",
		"明明收住了，却还是越线",
	}
	seen := map[string]struct{}{}
	titles := make([]string, 0, len(body))
	for _, item := range body {
		title := fmt.Sprintf("【%s】%s", cp, strings.TrimSpace(item))
		if _, ok := seen[title]; ok {
			continue
		}
		seen[title] = struct{}{}
		titles = append(titles, title)
	}
	return titles
}

const projectChatTitlePythonEntrypoint = `
import asyncio
import json
import sys

from generation_core import generate_title_from_content

payload = json.load(sys.stdin)
title = asyncio.run(
    generate_title_from_content(
        payload["cp"],
        payload["content"],
        reference_title=payload.get("reference_title", ""),
    )
)
sys.stdout.write(json.dumps({"titles": [title]}, ensure_ascii=False))
`
