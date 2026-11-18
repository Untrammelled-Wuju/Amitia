const APPROVAL_METHOD = "host.approval.request";

function textOf(value) {
  return typeof value === "string" ? value : "";
}

export function approvalPromptText(request) {
  const toolName = textOf(request?.toolName).trim();
  const reason = textOf(request?.reason).trim();
  const lines = [];
  lines.push(toolName ? `插件请求执行工具：${toolName}` : "插件请求执行受限操作");
  if (reason) lines.push(`原因：${reason}`);
  return lines.join("\n");
}

export function createAmitiaApprovalAnswerer({ host, timeoutMs, scopeSnapshotId, conversationId } = {}) {
  if (!host || typeof host.call !== "function") {
    throw new Error("DSH approval 应答器需要 Amitia 宿主 host.call");
  }
  const budget = Number(timeoutMs);
  const snapshotId = textOf(scopeSnapshotId).trim();
  if (snapshotId && typeof host.callTargeted !== "function") {
    throw new Error("宿主未提供 host.callTargeted，无法按会话作用域快照发起审批");
  }
  const callOptions = Number.isFinite(budget) && budget > 0 ? { timeoutMs: Math.trunc(budget) + 30000 } : undefined;
  const invoke = (payload) => (snapshotId
    ? host.callTargeted(APPROVAL_METHOD, payload, snapshotId, callOptions)
    : host.call(APPROVAL_METHOD, payload, callOptions));
  return async function answerApproval(request) {
    console.error(`[dsh-approval] request tool=${textOf(request?.toolName)} call=${textOf(request?.callId)} snapshot=${snapshotId ? "yes" : "no"} conversation=${textOf(conversationId) ? "yes" : "no"}`);
    const result = await invoke({
      toolName: textOf(request?.toolName) || "unknown-tool",
      arguments: approvalPromptText(request),
      toolCallId: textOf(request?.callId),
      requestId: textOf(request?.id),
      ...(textOf(conversationId).trim() ? { conversationId: textOf(conversationId).trim() } : {}),
      ...(Number.isFinite(budget) && budget > 0 ? { timeoutMs: Math.trunc(budget) } : {}),
    });
    if (result?.approved === true) return "allowed-once";
    if (result?.approved === false) return "rejected";
    throw new Error(`${APPROVAL_METHOD} 返回了非法结果 ${JSON.stringify(result)}`);
  };
}

export function installApprovalAnswerer(context, answerer) {
  if (!context || typeof context.on !== "function") {
    throw new Error("DSH approval 应答器需要 cordis context.on");
  }
  if (typeof answerer !== "function") {
    throw new Error("DSH approval 应答器必须是函数");
  }
  return context.on("approval/request", answerer);
}
