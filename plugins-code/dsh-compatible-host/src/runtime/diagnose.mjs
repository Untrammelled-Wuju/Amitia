const PROVIDER_ID = "amitia";

import { createAmitiaApprovalAnswerer } from "./approval-bridge.mjs";

function textOf(value) {
  return typeof value === "string" ? value : "";
}

function messageText(message) {
  if (typeof message === "string") return message;
  if (!message || typeof message !== "object") return "";
  if (typeof message.text === "string") return message.text;
  const content = message.content ?? message.parts;
  if (typeof content === "string") return content;
  if (!Array.isArray(content)) return "";
  return content
    .map((block) => {
      if (typeof block === "string") return block;
      if (block && typeof block === "object") {
        if (typeof block.text === "string") return block.text;
        if (typeof block.content === "string") return block.content;
      }
      return "";
    })
    .join("");
}

function assistantEvents(events) {
  return events.filter((event) => textOf(event?.type) === "assistant/message");
}

function assistantTextFromEvents(events) {
  const matched = assistantEvents(events);
  for (let index = matched.length - 1; index >= 0; index -= 1) {
    const message = matched[index]?.data?.message;
    const text = messageText(message);
    if (text) return text;
  }
  return "";
}

function registerProbeTool(agentCtx, name) {
  if (!agentCtx || !agentCtx.tools || typeof agentCtx.tools.register !== "function") {
    throw new Error("官方 agent setup 上下文缺少 tools.register，无法注册探针工具");
  }
  agentCtx.tools.register({
    name,
    description: "Return the fixed probe payload { ok: true }.",
    parameters: {
      type: "object",
      properties: {},
      additionalProperties: false,
    },
    output: {
      schema: {
        type: "object",
        properties: { ok: { type: "boolean" } },
        required: ["ok"],
        additionalProperties: false,
      },
      render(args, value) {
        return [{ type: "text", text: JSON.stringify(value) }];
      },
    },
    async execute() {
      return { ok: true };
    },
  });
}

function registerApprovalProbe(agentCtx, agent, name) {
  if (!agentCtx || !agentCtx.tools || typeof agentCtx.tools.register !== "function") {
    throw new Error("官方 agent setup 上下文缺少 tools.register，无法注册审批探针");
  }
  const approval = typeof agentCtx.get === "function" ? agentCtx.get("approval") : undefined;
  if (!approval || typeof approval.request !== "function") {
    throw new Error("官方 agent 上下文缺少 approval.request，官方 @deepseek-ai/dsh-user-approval 未就绪");
  }
  agentCtx.tools.register({
    name,
    description: "Ask the conversation owner to approve one probe action and return the approval outcome.",
    parameters: {
      type: "object",
      properties: {},
      additionalProperties: false,
    },
    output: {
      schema: {
        type: "object",
        properties: { outcome: { type: "string" } },
        required: ["outcome"],
        additionalProperties: false,
      },
      render(args, value) {
        return [{ type: "text", text: JSON.stringify(value) }];
      },
    },
    async execute(_args, exec) {
      const outcome = await approval.request({
        agent,
        toolName: name,
        reason: "DSH 审核探针",
        signal: exec?.signal,
      });
      return { outcome };
    },
  });
}

function eventTypeCounts(events) {
  const counts = {};
  for (const event of events) {
    const type = textOf(event?.type);
    if (!type) continue;
    counts[type] = (counts[type] ?? 0) + 1;
  }
  return counts;
}

export async function runModelDiagnose(options = {}) {
  const model = textOf(options.model).trim();
  const prompt = textOf(options.prompt).trim();
  const sessionId = textOf(options.sessionId).trim();
  const cwd = textOf(options.cwd).trim();
  if (!model) throw new Error("DSH 模型自检需要 model（Amitia 当前激活模型名）");
  if (!prompt) throw new Error("DSH 模型自检需要 prompt");
  if (!sessionId) throw new Error("DSH 模型自检需要 sessionId");
  if (!cwd || !cwd.startsWith("/")) throw new Error(`DSH 模型自检需要绝对路径 cwd，收到 ${JSON.stringify(cwd)}`);

  const llm = options.llm;
  if (!llm || typeof llm.stream !== "function") {
    throw new Error("容器 llm 服务未提供 stream，官方 LlmRuntime 或 adapter 未就绪");
  }

  const probe = {
    provider: PROVIDER_ID,
    model,
    chunkTypes: {},
    text: "",
    reasoning: "",
    finishReason: null,
    usage: null,
  };
  for await (const chunk of llm.stream({
    provider: PROVIDER_ID,
    model,
    messages: [{ role: "user", content: [{ type: "text", text: prompt }] }],
    maxTokens: options.maxTokens,
    sessionId,
    purpose: "dsh-diagnose",
  })) {
    const type = textOf(chunk?.type);
    if (!type) {
      throw new Error(`llm.stream 返回了缺少 type 的 chunk: ${JSON.stringify(chunk)}`);
    }
    probe.chunkTypes[type] = (probe.chunkTypes[type] ?? 0) + 1;
    if (type === "text-delta") probe.text += String(chunk.text ?? "");
    if (type === "reasoning-delta") probe.reasoning += String(chunk.text ?? "");
    if (type === "usage") probe.usage = chunk.usage ?? null;
    if (type === "finish") probe.finishReason = chunk.reason ?? null;
  }
  if (!probe.text) {
    throw new Error(`llm.stream 未产出文本: ${JSON.stringify({ chunkTypes: probe.chunkTypes, finishReason: probe.finishReason })}`);
  }

  const registry = options.agents;
  if (!registry || typeof registry.create !== "function") {
    throw new Error("容器 agents 服务未提供官方 create，官方 @deepseek-ai/dsh-agent 未就绪");
  }
  const toolProbe = options.toolProbe === true;
  const approvalProbe = options.approvalProbe === true;
  const normalizedSession = sessionId.replace(/[^a-zA-Z0-9_]/g, "_");
  const probeToolName = `amitia_probe_${normalizedSession}`.slice(0, 64);
  const approvalToolName = `amitia_approval_${normalizedSession}`.slice(0, 64);
  const answererDisposers = [];
  if (typeof options.installApprovalAnswerer === "function") {
    if (approvalProbe && !options.approvalScopeSnapshotId) {
      throw new Error("approvalProbe 需要 approvalScopeSnapshotId");
    }
    const answerer = createAmitiaApprovalAnswerer({
      host: options.host,
      timeoutMs: options.approvalTimeoutMs,
      scopeSnapshotId: options.approvalScopeSnapshotId,
      conversationId: options.approvalConversationId,
    });
    answererDisposers.push(options.installApprovalAnswerer(answerer));
  } else if (approvalProbe) {
    throw new Error("approvalProbe 需要 installApprovalAnswerer 注入审批监听器");
  }
  const commandContext = { agentCtx: null, agent: null };
  const setup = (agentCtx, agent) => {
        commandContext.agentCtx = agentCtx;
        commandContext.agent = agent;
        if (toolProbe) registerProbeTool(agentCtx, probeToolName);
        if (approvalProbe) {
          registerApprovalProbe(agentCtx, agent, approvalToolName);
        }
      };
  const handle = await registry.create({
    sessionId,
    meta: { cwd },
    agentOptions: {
      provider: PROVIDER_ID,
      model,
      ...(Number(options.maxTokens) > 0 ? { maxTokens: Number(options.maxTokens) } : {}),
    },
    setup,
  });
  const agent = handle?.agent;
  if (!agent || typeof agent.followup !== "function" || typeof agent.whenIdle !== "function") {
    throw new Error(`官方 agents.create 未返回可驱动的 agent: ${JSON.stringify(Object.keys(handle ?? {}))}`);
  }

  agent.followup({ role: "user", id: `${sessionId}-user`, source: { kind: "user" }, content: [{ type: "text", text: prompt }] });
  await agent.whenIdle();

  const session = agent.session;
  const events = typeof session?.snapshotEvents === "function" ? session.snapshotEvents() : [];
  if (events.length === 0) {
    throw new Error(`官方 agent session ${sessionId} 未记录任何事件，无法证明真实回合执行`);
  }
  const messages = typeof session?.deriveMessages === "function" ? session.deriveMessages() : [];
  const types = eventTypeCounts(events);
  const assistantText = assistantTextFromEvents(events) || messageText(messages.filter((message) => textOf(message?.role) === "assistant").at(-1));
  const turn = {
    sessionId,
    cwd,
    eventCount: events.length,
    messageCount: messages.length,
    eventTypes: types,
    turnStart: types["turn/start"] ?? 0,
    turnEnd: types["turn/end"] ?? 0,
    assistantMessages: types["assistant/message"] ?? 0,
    toolCalls: types["tool/call"] ?? 0,
    toolResults: types["tool/result"] ?? 0,
    probeTool: toolProbe ? probeToolName : "",
    approvalProbeTool: approvalProbe ? approvalToolName : "",
    approvalConversationId: textOf(options.approvalConversationId),
    approvalRequested: types["approval/requested"] ?? 0,
    approvalDecided: types["approval/decided"] ?? 0,
    assistantText,
  };

  let commandReport = null;
  if (textOf(options.commandLine).trim()) {
    const commands = typeof commandContext.agentCtx?.get === "function" ? commandContext.agentCtx.get("commands") : undefined;
    if (!commands || typeof commands.execute !== "function") {
      throw new Error("官方 commands 服务未就绪，无法执行斜杠命令");
    }
    const execution = await commands.execute(commandContext.agent, textOf(options.commandLine).trim(), [], new AbortController().signal);
    commandReport = execution === undefined
      ? { line: textOf(options.commandLine).trim(), execution: null }
      : { line: textOf(options.commandLine).trim(), execution: JSON.parse(JSON.stringify(execution)) };
  }

  const spillStore = options.spillStore;
  let spill = null;
  if (spillStore && typeof spillStore.saveText === "function") {
    const content = assistantText || probe.text;
    const saved = await spillStore.saveText({
      owner: { sessionId },
      suggestedName: "diagnose.txt",
      content,
    });
    spill = { locator: textOf(saved?.locator), bytes: Number(saved?.bytes) || 0, source: assistantText ? "turn" : "probe" };
  }

  if (approvalProbe && turn.approvalDecided === 0) {
    throw new Error(`DSH approval 探针未产生审批记录: ${JSON.stringify({ sessionId, eventTypes: types, tool: approvalToolName })}`);
  }
  if (!assistantText || turn.turnEnd === 0) {
    const failure = {
      sessionId,
      eventTypes: types,
      turnMessages: assistantEvents(events).length,
      lastEvents: events.slice(-6).map((event) => ({ type: event?.type, seq: event?.seq, data: event?.data })),
      llm: { chunkTypes: probe.chunkTypes, finishReason: probe.finishReason, text: probe.text.slice(0, 200) },
    };
    throw new Error(`DSH agent turn 未完成真实模型回合: ${JSON.stringify(failure)}`);
  }

  const dispose = options.dispose === true;
  if (dispose && typeof handle.dispose === "function") await handle.dispose();

  return {
    provider: PROVIDER_ID,
    model,
    llm: probe,
    turn,
    spill,
    ...(commandReport ? { command: commandReport } : {}),
    disposed: dispose,
  };
}
