package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/capability/acquisition"
)

type workshopGeneratorPortAdapter struct {
	model WorkshopModelGenerator
}

func NewWorkshopGeneratorPortAdapter(model WorkshopModelGenerator) acquisition.WorkshopGeneratePort {
	return &workshopGeneratorPortAdapter{model: model}
}

func (a *workshopGeneratorPortAdapter) GenerateInstruction(ctx context.Context, requirement string) (acquisition.WorkshopInstructionDraft, error) {
	if a.model == nil {
		return acquisition.WorkshopInstructionDraft{}, fmt.Errorf("workshop: model not configured")
	}
	if requirement == "" {
		return acquisition.WorkshopInstructionDraft{}, fmt.Errorf("workshop: empty requirement")
	}
	prompt := "你是 Amitia Agent Skill 工坊。只返回 JSON，字段必须且只能是 name、description、body、references、assets、displayName、shortDescription。name 必须是小写短横线格式，description 必须说明功能和适用触发场景，body 是清晰的 Markdown 命令式流程。references 和 assets 是相对文件名到纯文本内容的对象，只能放知识材料和模板。禁止 scripts、源码、Shell、Python、Node、PowerShell、MCP、allowed-tools、Secret、安装说明、README 和 CHANGELOG。"
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		raw, _, _, err := a.model.GenerateWorkshopJSON(ctx, prompt, requirement)
		if err != nil {
			last = err
			continue
		}
		var draft acquisition.WorkshopInstructionDraft
		decoder := json.NewDecoder(strings.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&draft); err != nil {
			last = err
			continue
		}
		if draft.Name == "" || strings.TrimSpace(draft.Body) == "" {
			last = fmt.Errorf("生成内容不符合 Agent Skill 结构")
			continue
		}
		return draft, nil
	}
	return acquisition.WorkshopInstructionDraft{}, fmt.Errorf("workshop: 生成失败: %v", last)
}
