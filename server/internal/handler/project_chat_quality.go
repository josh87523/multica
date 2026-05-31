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

type ProjectChatQualitySummary struct {
	Items []string `json:"items"`
}

type ProjectChatQualityRequest struct {
	ProjectTitle       string
	ProjectDescription string
	CP                 string
	Title              string
	Content            string
}

type projectChatQualityAdapter interface {
	Review(ctx context.Context, req ProjectChatQualityRequest) (ProjectChatQualitySummary, error)
}

type pythonProjectChatQualityAdapter struct {
	botRoot string
	runCmd  projectChatCommandRunner
}

func newProjectChatQualityAdapter() projectChatQualityAdapter {
	return &pythonProjectChatQualityAdapter{
		botRoot: resolveProjectChatBotRoot(),
		runCmd:  runProjectChatCommand,
	}
}

func (a *pythonProjectChatQualityAdapter) Review(ctx context.Context, req ProjectChatQualityRequest) (ProjectChatQualitySummary, error) {
	if a == nil || strings.TrimSpace(a.botRoot) == "" {
		return ProjectChatQualitySummary{}, errors.New("lofter quality adapter is not configured")
	}
	for _, required := range []string{"publish_gate.py", "human_benchmark.py", "deai_processor.py"} {
		if _, err := os.Stat(filepath.Join(a.botRoot, required)); err != nil {
			return ProjectChatQualitySummary{}, fmt.Errorf("lofter quality adapter unavailable: %w", err)
		}
	}

	payload := map[string]string{
		"project_title":       strings.TrimSpace(req.ProjectTitle),
		"project_description": strings.TrimSpace(req.ProjectDescription),
		"cp":                  strings.TrimSpace(req.CP),
		"title":               strings.TrimSpace(req.Title),
		"content":             strings.TrimSpace(req.Content),
	}
	stdin, err := json.Marshal(payload)
	if err != nil {
		return ProjectChatQualitySummary{}, err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := a.runCmd(
		timeoutCtx,
		"python3",
		[]string{"-c", projectChatQualityPythonEntrypoint},
		append(os.Environ(), "PYTHONPATH="+a.botRoot),
		a.botRoot,
		stdin,
	)
	if err != nil {
		return ProjectChatQualitySummary{}, err
	}

	var resp ProjectChatQualitySummary
	if err := decodeProjectChatTitleResponse(out, &resp); err != nil {
		return ProjectChatQualitySummary{}, err
	}
	resp.Items = dedupeAndTrimStrings(resp.Items)
	if len(resp.Items) == 0 {
		return ProjectChatQualitySummary{}, errors.New("lofter quality adapter returned empty summary")
	}
	return resp, nil
}

func buildFallbackProjectChatQualitySummary(req ProjectChatQualityRequest) ProjectChatQualitySummary {
	items := []string{}
	title := strings.TrimSpace(req.Title)
	if title != "" {
		if len([]rune(title)) > 26 {
			items = append(items, "标题质量: 偏长，建议再收短一点")
		} else {
			items = append(items, "标题质量: 当前长度可用，先看情绪钩子是否足够集中")
		}
	}
	content := strings.TrimSpace(req.Content)
	switch {
	case strings.Contains(content, "空气仿佛凝固") || strings.Contains(content, "喉咙发紧") || strings.Contains(content, "眼神动了动"):
		items = append(items, "AI 味风险: 偏高，已有高频规则词命中")
	case content != "":
		items = append(items, "AI 味风险: 暂未发现明显高频规则词，但仍建议人工过一眼")
	}
	if desc := strings.TrimSpace(req.ProjectDescription); desc != "" {
		items = append(items, "CP/角色一致性: 当前按项目描述对齐，未见明显跑题信号")
	}
	items = append(items, "Benchmark 摘要: 继续参考 LOFTER 活人感对标，不要把标题和正文写成统一模板句")
	return ProjectChatQualitySummary{Items: dedupeAndTrimStrings(items)}
}

const projectChatQualityPythonEntrypoint = `
import json
import sys

from deai_processor import DeAIProcessor
from human_benchmark import select_life_benchmark_source
from publish_gate import validate_title_content_sync, is_title_validation_indeterminate

payload = json.load(sys.stdin)
title = (payload.get("title") or "").strip()
content = (payload.get("content") or "").strip()
cp = (payload.get("cp") or payload.get("project_title") or "LOFTER").strip()
items = []

if title and content:
    ok, message = validate_title_content_sync(title, content, cp)
    if ok:
        items.append("标题质量: 标题和正文主事件基本一致")
    elif is_title_validation_indeterminate(message):
        items.append("标题质量: 校验结果暂不确定，建议人工快速过一眼")
    else:
        items.append(f"标题质量: {message}")

if content:
    processor = DeAIProcessor()
    result = processor.process(content, add_disclaimer=False)
    rule_count = len(result.applied_rules or [])
    reduction_ratio = result.reduction_ratio
    if rule_count >= 3 or reduction_ratio >= 0.18:
        items.append(f"AI 味风险: 偏高，规则命中 {rule_count} 项")
    elif rule_count > 0 or reduction_ratio >= 0.08:
        items.append(f"AI 味风险: 中等，规则命中 {rule_count} 项")
    else:
        items.append("AI 味风险: 低，暂未命中明显规则")

source = select_life_benchmark_source(
    {"name": payload.get("project_title") or cp},
    {"tone": "克制", "anchors": []},
    seed=f"{cp}:{title or payload.get('project_title') or 'quality'}",
)
items.append(f"Benchmark 摘要: 对标标题《{source.get('title') or '活人感结构对标'}》")
items.append("CP/角色一致性: 首版默认按当前项目描述和生成结果做弱校验，未做重链路人物审稿")
sys.stdout.write(json.dumps({"items": items}, ensure_ascii=False))
`
