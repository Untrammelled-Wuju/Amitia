package chat

import (
	"strings"

	coreexec "github.com/u-ai/backend/internal/execution"
	promptir "github.com/u-ai/backend/internal/prompt"
)

const workspaceAgentExecutionContract = `【工作区 Agent 执行契约】
当前对话已绑定可操作的项目工作区。角色的表达方式不能阻止工具调用和执行验证。
用户要求实际修改、构建、测试、部署或操作界面时，在能力和授权范围内实际执行，而不是用计划、伪造执行记录或纯文本描述代替。
每轮遵循目标、动作、观察结果、验证结果的闭环。先读取相关项目约束和源码，再选择最小风险的工具。
读取、编辑、命令、构建、自动化和验证均使用本轮实际暴露的工具。禁止调用未暴露的工具。
工具失败、权限拒绝、进程异常、设备离线和未知执行状态不等于任务成功；需要根据工具结果诊断，并在安全条件下采取可验证的修复步骤。
修改源码后选择相关构建、测试或运行态验收；UI 改动在能力可用时需要可观察的渲染或界面验证。
只有在拿到充分证据时才能声称修改已成功、测试已通过或部署已完成。没有证据就说明实际完成的部分和具体阻碍。
可以持续进行多轮工具调用，不受普通聊天的句数或篇幅规则约束，但仍遵守取消、授权、资源和安全限制。
禁止无依据地声称已操作用户设备、文件或外部服务。`

func workspaceAgentBound(execCtx *coreexec.ExecutionContext) bool {
	return execCtx != nil && strings.TrimSpace(execCtx.WorkspaceID) != ""
}

func agentBaseIdentity(execCtx *coreexec.ExecutionContext) string {
	if !workspaceAgentBound(execCtx) {
		return promptir.BaseIdentitySection()
	}
	return promptir.BaseIdentityCoreSection() + "\n\n" + workspaceAgentExecutionContract
}
