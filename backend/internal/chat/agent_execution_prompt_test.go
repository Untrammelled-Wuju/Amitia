package chat

import (
	"strings"
	"testing"

	coreexec "github.com/u-ai/backend/internal/execution"
)

func TestWorkspaceAgentUsesExecutionContractInsteadOfShortChatRule(t *testing.T) {
	if workspaceAgentBound(nil) || workspaceAgentBound(&coreexec.ExecutionContext{}) {
		t.Fatal("only a bound workspace may bypass conversation delivery directives")
	}
	if !workspaceAgentBound(&coreexec.ExecutionContext{WorkspaceID: "project"}) {
		t.Fatal("a bound workspace must be recognized as an execution context")
	}
	bound := agentBaseIdentity(&coreexec.ExecutionContext{WorkspaceID: "project"})
	if !strings.Contains(bound, "工作区 Agent 执行契约") {
		t.Fatal("bound workspace must receive an executable task contract")
	}
	if strings.Contains(bound, "每次回复1-5句短话") {
		t.Fatal("ordinary chat length limit must not override workspace task execution")
	}
	normal := agentBaseIdentity(nil)
	if strings.Contains(normal, "工作区 Agent 执行契约") || !strings.Contains(normal, "每次回复1-5句短话") {
		t.Fatal("ordinary companion conversations must preserve their original style")
	}
}
