package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ProjectChatRewriteTitleRequest struct {
	CP             string
	OriginalTitle  string
	Instruction    string
	ReferenceTitle string
}

type ProjectChatDeAIRequest struct {
	Text        string
	Instruction string
}

type projectChatCreateAdapter interface {
	RewriteTitle(ctx context.Context, req ProjectChatRewriteTitleRequest) (string, error)
	DeAI(ctx context.Context, req ProjectChatDeAIRequest) (string, []string, error)
}

type pythonProjectChatCreateAdapter struct {
	botRoot string
	runCmd  projectChatCommandRunner
}

func newProjectChatCreateAdapter() projectChatCreateAdapter {
	return &pythonProjectChatCreateAdapter{
		botRoot: resolveProjectChatBotRoot(),
		runCmd:  runProjectChatCommand,
	}
}

func (a *pythonProjectChatCreateAdapter) RewriteTitle(ctx context.Context, req ProjectChatRewriteTitleRequest) (string, error) {
	if a == nil || strings.TrimSpace(a.botRoot) == "" {
		return "", errors.New("lofter create adapter is not configured")
	}
	if _, err := os.Stat(filepath.Join(a.botRoot, "generation_core.py")); err != nil {
		return "", fmt.Errorf("lofter create adapter unavailable: %w", err)
	}

	payload := map[string]string{
		"cp":              strings.TrimSpace(req.CP),
		"original_title":  strings.TrimSpace(req.OriginalTitle),
		"instruction":     strings.TrimSpace(req.Instruction),
		"reference_title": strings.TrimSpace(req.ReferenceTitle),
	}
	stdin, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := a.runCmd(
		timeoutCtx,
		"python3",
		[]string{"-c", projectChatRewriteTitlePythonEntrypoint},
		append(os.Environ(), "PYTHONPATH="+a.botRoot),
		a.botRoot,
		stdin,
	)
	if err != nil {
		return "", err
	}

	var resp struct {
		Title string `json:"title"`
	}
	if err := decodeProjectChatTitleResponse(out, &resp); err != nil {
		return "", err
	}
	title := strings.TrimSpace(resp.Title)
	if title == "" {
		return "", errors.New("lofter rewrite adapter returned empty title")
	}
	return title, nil
}

func (a *pythonProjectChatCreateAdapter) DeAI(ctx context.Context, req ProjectChatDeAIRequest) (string, []string, error) {
	if a == nil || strings.TrimSpace(a.botRoot) == "" {
		return "", nil, errors.New("lofter create adapter is not configured")
	}
	if _, err := os.Stat(filepath.Join(a.botRoot, "deai_processor.py")); err != nil {
		return "", nil, fmt.Errorf("lofter deai adapter unavailable: %w", err)
	}

	payload := map[string]string{
		"text":        strings.TrimSpace(req.Text),
		"instruction": strings.TrimSpace(req.Instruction),
	}
	stdin, err := json.Marshal(payload)
	if err != nil {
		return "", nil, err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := a.runCmd(
		timeoutCtx,
		"python3",
		[]string{"-c", projectChatDeAIPythonEntrypoint},
		append(os.Environ(), "PYTHONPATH="+a.botRoot),
		a.botRoot,
		stdin,
	)
	if err != nil {
		return "", nil, err
	}

	var resp struct {
		Text         string   `json:"text"`
		AppliedRules []string `json:"applied_rules"`
	}
	if err := decodeProjectChatTitleResponse(out, &resp); err != nil {
		return "", nil, err
	}
	text := strings.TrimSpace(resp.Text)
	if text == "" {
		return "", nil, errors.New("lofter deai adapter returned empty text")
	}
	return text, dedupeAndTrimStrings(resp.AppliedRules), nil
}

func buildFallbackProjectChatRewriteTitle(req ProjectChatRewriteTitleRequest) string {
	cp := strings.TrimSpace(req.CP)
	if cp == "" {
		cp = "LOFTER"
	}
	original := strings.TrimSpace(req.OriginalTitle)
	original = strings.TrimPrefix(original, "【"+cp+"】")
	original = strings.TrimSpace(strings.Trim(original, "[]【】"))
	if original == "" {
		original = "把失控收住一点"
	}
	original = strings.ReplaceAll(original, "太狗血", "更克制")
	original = strings.ReplaceAll(original, "慌了", "收住了")
	return fmt.Sprintf("【%s】%s", cp, original)
}

func buildFallbackProjectChatDeAI(req ProjectChatDeAIRequest) (string, []string) {
	text := strings.TrimSpace(req.Text)
	rules := []string{}
	replacements := [][2]string{
		{"空气仿佛凝固", ""},
		{"喉咙发紧", ""},
		{"眼神动了动", ""},
		{"故事才刚刚开始", ""},
		{"似乎", ""},
	}
	for _, pair := range replacements {
		if strings.Contains(text, pair[0]) {
			rules = append(rules, "fallback:"+pair[0])
			text = strings.ReplaceAll(text, pair[0], pair[1])
		}
	}
	text = strings.TrimSpace(strings.Join(strings.Fields(text), " "))
	if text == "" {
		text = strings.TrimSpace(req.Text)
	}
	return text, rules
}

const projectChatRewriteTitlePythonEntrypoint = `
import asyncio
import json
import sys

from generation_core import generate_title_from_content

payload = json.load(sys.stdin)
content = "\n".join([
    f"原标题: {payload.get('original_title', '')}".strip(),
    f"改写要求: {payload.get('instruction', '')}".strip(),
]).strip()
title = asyncio.run(
    generate_title_from_content(
        payload["cp"],
        content,
        reference_title=payload.get("reference_title") or payload.get("original_title", ""),
    )
)
sys.stdout.write(json.dumps({"title": title}, ensure_ascii=False))
`

const projectChatDeAIPythonEntrypoint = `
import json
import sys

from deai_processor import DeAIProcessor

payload = json.load(sys.stdin)
processor = DeAIProcessor()
result = processor.process(payload["text"], add_disclaimer=False)
sys.stdout.write(json.dumps({
    "text": result.text,
    "applied_rules": result.applied_rules,
}, ensure_ascii=False))
`
