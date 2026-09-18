package kernel

import (
	"sync/atomic"
)

type ToolFacadeCounters struct {
	prepareAgentSkillPrompt atomic.Int64
	endAgentSkillRound      atomic.Int64
	beforePrompt            atomic.Int64
	modelTools              atomic.Int64
	executeModelTool        atomic.Int64
	afterReply              atomic.Int64

	pipelineExecutions atomic.Int64
	pipelineFailures   atomic.Int64

	mcpToolSync          atomic.Int64
	mcpDuplicateDetected atomic.Int64
}

func NewToolFacadeCounters() *ToolFacadeCounters {
	return &ToolFacadeCounters{}
}

func (c *ToolFacadeCounters) IncPrepareAgentSkillPrompt() {
	c.prepareAgentSkillPrompt.Add(1)
}

func (c *ToolFacadeCounters) IncEndAgentSkillRound() {
	c.endAgentSkillRound.Add(1)
}

func (c *ToolFacadeCounters) IncBeforePrompt() {
	c.beforePrompt.Add(1)
}

func (c *ToolFacadeCounters) IncModelTools() {
	c.modelTools.Add(1)
}

func (c *ToolFacadeCounters) IncExecuteModelTool() {
	c.executeModelTool.Add(1)
}

func (c *ToolFacadeCounters) IncAfterReply() {
	c.afterReply.Add(1)
}

func (c *ToolFacadeCounters) IncPipelineExecution() {
	c.pipelineExecutions.Add(1)
}

func (c *ToolFacadeCounters) IncPipelineFailure(stage string) {
	c.pipelineFailures.Add(1)
	_ = stage
}

func (c *ToolFacadeCounters) IncMCPToolSync() {
	c.mcpToolSync.Add(1)
}

func (c *ToolFacadeCounters) IncMCPDuplicateDetected() {
	c.mcpDuplicateDetected.Add(1)
}

func (c *ToolFacadeCounters) Snapshot() map[string]int64 {
	return map[string]int64{
		"prepare_agent_skill_prompt": c.prepareAgentSkillPrompt.Load(),
		"end_agent_skill_round":      c.endAgentSkillRound.Load(),
		"before_prompt":              c.beforePrompt.Load(),
		"model_tools":                c.modelTools.Load(),
		"execute_model_tool":         c.executeModelTool.Load(),
		"after_reply":                c.afterReply.Load(),
		"pipeline_executions":        c.pipelineExecutions.Load(),
		"pipeline_failures":          c.pipelineFailures.Load(),
		"mcp_tool_sync":              c.mcpToolSync.Load(),
		"mcp_duplicate_detected":     c.mcpDuplicateDetected.Load(),
	}
}
