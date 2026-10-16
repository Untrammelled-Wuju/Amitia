package kernel

import (
	"github.com/u-ai/backend/internal/conversationstream"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
)

func approvalRuntimeTarget(request execution.ApprovalRequest) (string, *conversationstream.Manager) {
	if request.RuntimeKey != "" {
		return request.RuntimeKey, business.RuntimeManager()
	}
	return request.ConversationID, conversationstream.DefaultManager()
}
