package chat

import (
	"encoding/json"
	"fmt"
	"math"
)

type modelScenario struct {
	ID          string
	Label       string
	Description string
}

var modelScenarios = []modelScenario{
	{"chat", "聊天对话", "日常聊天、角色预览和日程消息；会话显式选择的模型优先"},
	{"summary", "会话摘要", "生成会话摘要、记忆摘要和会话标题"},
	{"memory_extract", "记忆提取", "从对话提取记忆、导入候选和整理候选"},
}

func supportedModelScenario(id string) bool {
	for _, scenario := range modelScenarios {
		if scenario.ID == id {
			return true
		}
	}
	return false
}

func routeModelID(value any) (int, error) {
	if value == nil {
		return 0, nil
	}
	var number float64
	switch v := value.(type) {
	case int:
		number = float64(v)
	case int64:
		number = float64(v)
	case float64:
		number = v
	case json.Number:
		parsed, err := v.Float64()
		if err != nil {
			return 0, fmt.Errorf("模型编号无效")
		}
		number = parsed
	default:
		return 0, fmt.Errorf("模型编号必须为非负整数")
	}
	if math.IsNaN(number) || math.IsInf(number, 0) || number < 0 || number > 2147483647 || math.Trunc(number) != number {
		return 0, fmt.Errorf("模型编号必须为非负整数")
	}
	return int(number), nil
}

func (s *service) modelForScenario(scenario string, explicitID int) (*ModelConfig, error) {
	if !supportedModelScenario(scenario) {
		return nil, fmt.Errorf("不支持的模型用途: %s", scenario)
	}
	if explicitID > 0 {
		return s.repo.GetModelByID(explicitID)
	}
	routes, err := s.repo.GetModelRoutes()
	if err != nil {
		return nil, err
	}
	for _, route := range routes {
		if route["scenario"] != scenario {
			continue
		}
		id, err := routeModelID(route["modelConfigId"])
		if err != nil {
			return nil, err
		}
		if id > 0 {
			return s.repo.GetModelByID(id)
		}
	}
	return s.repo.GetActiveModel()
}
