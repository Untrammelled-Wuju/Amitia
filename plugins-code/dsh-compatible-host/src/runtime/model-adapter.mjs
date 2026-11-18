const PROVIDER_ID = "amitia";
const MODEL_INVOKE_METHOD = "host.model.invoke";

export const name = "amitia-model-adapter";
export const inject = ["llm"];

function textOf(value) {
  return typeof value === "string" ? value : "";
}

function adapterFailure(message, code) {
  const error = new Error(message);
  error.code = code;
  error.failure = { message, code };
  return error;
}

function describeThrown(value) {
  if (value instanceof Error) return `${value.name}: ${value.message}`;
  try {
    return JSON.stringify(value);
  } catch (error) {
    return String(value);
  }
}

function assertActiveSignal(signal) {
  if (!signal || typeof signal !== "object" || !signal.aborted) return;
  const reason = signal.reason;
  const detail = reason === undefined ? "" : `: ${describeThrown(reason)}`;
  throw adapterFailure(`模型调用已取消${detail}`, "ABORTED");
}

function blockError(position, role, blockType) {
  return adapterFailure(
    `第 ${position} 条 ${role} 消息包含 ${JSON.stringify(blockType)} 内容块，Amitia 模型网关只支持文本与工具调用内容`,
    "UNSUPPORTED_CONTENT",
  );
}

function contentText(content, position, role) {
  if (!Array.isArray(content)) {
    throw adapterFailure(`第 ${position} 条 ${role} 消息的 content 不是内容块数组: ${describeThrown(content)}`, "INVALID_REQUEST");
  }
  const parts = [];
  for (const block of content) {
    const type = textOf(block?.type);
    if (type === "text") parts.push(textOf(block.text));
    else if (type === "reasoning") continue;
    else throw blockError(position, role, type || block);
  }
  return parts.join("");
}

function assistantContent(content, position) {
  if (!Array.isArray(content)) {
    throw adapterFailure(`第 ${position} 条 assistant 消息的 content 不是内容块数组: ${describeThrown(content)}`, "INVALID_REQUEST");
  }
  const parts = [];
  const toolCalls = [];
  for (const block of content) {
    const type = textOf(block?.type);
    if (type === "text") parts.push(textOf(block.text));
    else if (type === "reasoning") continue;
    else if (type === "tool-call") {
      const id = textOf(block.id);
      const callName = textOf(block.name);
      if (!id || !callName) {
        throw adapterFailure(
          `第 ${position} 条 assistant 消息的 tool-call 块缺少 id 或 name: ${describeThrown(block)}`,
          "INVALID_REQUEST",
        );
      }
      toolCalls.push({ id, name: callName, arguments: textOf(block.arguments) });
    } else throw blockError(position, "assistant", type || block);
  }
  return { text: parts.join(""), toolCalls };
}

function translateMessages(messages) {
  if (!Array.isArray(messages)) {
    throw adapterFailure(`GenerateOptions.messages 不是数组: ${describeThrown(messages)}`, "INVALID_REQUEST");
  }
  const translated = [];
  for (let position = 0; position < messages.length; position += 1) {
    const message = messages[position];
    if (!message || typeof message !== "object") {
      throw adapterFailure(`第 ${position} 条消息不是对象: ${describeThrown(message)}`, "INVALID_REQUEST");
    }
    const role = textOf(message.role);
    if (role === "system" || role === "developer") {
      translated.push({ role, content: contentText(message.content, position, role) });
      continue;
    }
    if (role === "user") {
      translated.push({ role: "user", content: contentText(message.content, position, role) });
      continue;
    }
    if (role === "assistant") {
      const assembled = assistantContent(message.content, position);
      const entry = { role: "assistant", content: assembled.text };
      if (assembled.toolCalls.length > 0) entry.toolCalls = assembled.toolCalls;
      translated.push(entry);
      continue;
    }
    if (role === "tool") {
      const callId = textOf(message.toolCallId) || textOf(message.source?.callId);
      if (!callId) {
        throw adapterFailure(
          `第 ${position} 条 tool 消息缺少 toolCallId: ${describeThrown(message.source)}`,
          "INVALID_REQUEST",
        );
      }
      translated.push({
        role: "tool",
        content: contentText(message.content, position, role),
        toolCallId: callId,
      });
      continue;
    }
    throw adapterFailure(`第 ${position} 条消息的角色 ${JSON.stringify(message.role)} 不受支持`, "INVALID_REQUEST");
  }
  return translated;
}

function translateTools(tools) {
  if (tools === undefined || tools === null) return [];
  if (!Array.isArray(tools)) {
    throw adapterFailure(`GenerateOptions.tools 不是数组: ${describeThrown(tools)}`, "INVALID_REQUEST");
  }
  const translated = [];
  for (const tool of tools) {
    const toolName = textOf(tool?.name);
    if (!toolName) {
      throw adapterFailure(`工具声明缺少 name: ${describeThrown(tool)}`, "INVALID_REQUEST");
    }
    const parameters = tool.parameters === undefined || tool.parameters === null ? { type: "object", properties: {} } : tool.parameters;
    if (typeof parameters !== "object") {
      throw adapterFailure(`工具 ${toolName} 的 parameters 不是 JSON Schema 对象: ${describeThrown(tool.parameters)}`, "INVALID_REQUEST");
    }
    translated.push({ name: toolName, description: textOf(tool.description), parameters });
  }
  return translated;
}

function buildInvokeRequest(options) {
  if (Array.isArray(options.stop) && options.stop.length > 0) {
    throw adapterFailure(
      `Amitia 模型网关不支持 stop 序列，请求携带了 ${JSON.stringify(options.stop)}`,
      "UNSUPPORTED_OPTION",
    );
  }
  const request = {
    provider: PROVIDER_ID,
    model: textOf(options.model),
    system: textOf(options.system),
    messages: translateMessages(options.messages),
    tools: translateTools(options.tools),
    maxOutputTokens: Number.isFinite(Number(options.maxTokens)) && Number(options.maxTokens) > 0
      ? Math.trunc(Number(options.maxTokens))
      : 0,
    reasoningEffort: textOf(options.reasoningEffort),
    sessionId: textOf(options.sessionId),
    purpose: textOf(options.purpose),
  };
  if (typeof options.temperature === "number" && Number.isFinite(options.temperature)) {
    request.temperature = options.temperature;
  }
  return request;
}

function toCount(value) {
  const numeric = Number(value);
  return Number.isFinite(numeric) && numeric > 0 ? Math.trunc(numeric) : 0;
}

function mapUsage(usage) {
  if (!usage || typeof usage !== "object" || usage.reported !== true) return null;
  const promptTotal = toCount(usage.inputTokens);
  const cached = toCount(usage.cachedInputTokens);
  const cacheRead = usage.cacheUsageReported === true ? Math.min(cached, promptTotal) : 0;
  const mapped = {
    inputTokens: Math.max(0, promptTotal - cacheRead),
    outputTokens: toCount(usage.outputTokens),
  };
  if (cacheRead > 0) mapped.cacheReadTokens = cacheRead;
  const total = toCount(usage.totalTokens);
  if (total > 0) mapped.totalTokens = total;
  const reasoningTokens = toCount(usage.reasoningTokens);
  if (reasoningTokens > 0) mapped.reasoningTokens = reasoningTokens;
  return mapped;
}

function mapFinishReason(raw, toolCallCount, logger) {
  const value = textOf(raw).trim().toLowerCase();
  if (value === "tool_calls" || value === "tool-calls") return { kind: "tool-calls" };
  if (value === "length" || value === "max_tokens" || value === "max-tokens") return { kind: "max-tokens" };
  if (value === "aborted" || value === "cancelled" || value === "canceled") {
    return { kind: "aborted", failure: { message: "模型调用已取消", code: "ABORTED" } };
  }
  if (value === "stop" || value === "end_turn" || value === "end-turn") return { kind: "stop" };
  if (value === "") return toolCallCount > 0 ? { kind: "tool-calls" } : { kind: "stop" };
  if (logger && typeof logger.warn === "function") {
    logger.warn(`[amitia-model-adapter] 模型返回了未映射的 finishReason ${JSON.stringify(raw)}，按 ${toolCallCount > 0 ? "tool-calls" : "stop"} 处理`);
  }
  return toolCallCount > 0 ? { kind: "tool-calls" } : { kind: "stop" };
}

function buildChunks(result, logger) {
  if (!result || typeof result !== "object") {
    throw adapterFailure(`host.model.invoke 返回了非法结果: ${describeThrown(result)}`, "INVALID_RESPONSE");
  }
  const chunks = [];
  let index = 0;
  const reasoning = textOf(result.reasoning);
  if (reasoning.length > 0) {
    chunks.push({ type: "block-start", index, blockType: "reasoning" });
    chunks.push({ type: "reasoning-delta", index, text: reasoning });
    chunks.push({ type: "block-end", index, block: { type: "reasoning", text: reasoning } });
    index += 1;
  }
  const visible = textOf(result.text).length > 0 ? textOf(result.text) : textOf(result.refusal);
  if (visible.length > 0) {
    chunks.push({ type: "block-start", index, blockType: "text" });
    chunks.push({ type: "text-delta", index, text: visible });
    chunks.push({ type: "block-end", index, block: { type: "text", text: visible } });
    index += 1;
  }
  const toolCalls = Array.isArray(result.toolCalls) ? result.toolCalls : [];
  for (const call of toolCalls) {
    const id = textOf(call?.id);
    const callName = textOf(call?.name);
    if (!id || !callName) {
      throw adapterFailure(`host.model.invoke 返回的 tool call 缺少 id 或 name: ${describeThrown(call)}`, "INVALID_RESPONSE");
    }
    const args = textOf(call.arguments);
    chunks.push({ type: "block-start", index, blockType: "tool-call" });
    chunks.push({ type: "tool-call-delta", index, id, name: callName, argumentsDelta: args });
    chunks.push({ type: "block-end", index, block: { type: "tool-call", id, name: callName, arguments: args } });
    index += 1;
  }
  const usage = mapUsage(result.usage);
  if (usage) chunks.push({ type: "usage", usage });
  chunks.push({ type: "finish", reason: mapFinishReason(result.finishReason, toolCalls.length, logger) });
  return chunks;
}

export function createAmitiaLlmAdapter({ host, logger } = {}) {
  if (!host || typeof host.call !== "function") {
    throw adapterFailure("amitia model adapter 需要 Amitia 宿主 host.call", "INVALID_CONFIGURATION");
  }
  const invokeHost = (payload) => host.call(MODEL_INVOKE_METHOD, payload);
  const contextWindows = new Map();
  const contextKey = (provider, model) => `${provider}\u0000${model}`;
  const rememberContextWindow = (provider, model, result) => {
    const contextWindow = Number(result?.contextWindow);
    if (!Number.isFinite(contextWindow) || contextWindow <= 0) return;
    contextWindows.set(contextKey(provider, model), contextWindow);
  };

  async function* streamCall(options) {
    if (!options || typeof options !== "object") {
      throw adapterFailure(`GenerateOptions 不是对象: ${describeThrown(options)}`, "INVALID_REQUEST");
    }
    const provider = textOf(options.provider);
    if (provider !== PROVIDER_ID) {
      throw adapterFailure(`amitia adapter 只处理 provider=${PROVIDER_ID} 的请求，收到 ${JSON.stringify(options.provider)}`, "INVALID_REQUEST");
    }
    assertActiveSignal(options.signal);
    const request = buildInvokeRequest(options);
    const result = await invokeHost(request);
    assertActiveSignal(options.signal);
    rememberContextWindow(provider, textOf(request.model), result);
    for (const chunk of buildChunks(result, logger)) yield chunk;
  }

  return {
    providerInfo(provider) {
      return { id: provider, name: "Amitia 模型网关" };
    },
    providerRetryPolicy() {
      return undefined;
    },
    imageRequestPricing() {
      return undefined;
    },
    imageRequestTarget(provider, model, image) {
      return { width: image.width, height: image.height, maxBytes: image.bytes };
    },
    async listModels() {
      return [];
    },
    async resolveModel(provider, model) {
      const contextWindow = contextWindows.get(contextKey(provider, model));
      return {
        provider,
        id: model,
        name: model,
        inputModalities: ["text"],
        ...(contextWindow ? { context: { contextWindow } } : {}),
      };
    },
    async prepareCall(provider, model, signal) {
      if (textOf(provider) !== PROVIDER_ID) {
        throw adapterFailure(`amitia adapter 只处理 provider=${PROVIDER_ID} 的请求，收到 ${JSON.stringify(provider)}`, "INVALID_REQUEST");
      }
      assertActiveSignal(signal);
      return {
        model: await this.resolveModel(provider, model, signal),
        stream: (options) => streamCall(options),
      };
    },
    stream(options) {
      return streamCall(options);
    },
  };
}

export function apply(ctx, config = {}) {
  const adapter = createAmitiaLlmAdapter({ host: config?.host, logger: config?.logger });
  ctx.llm.registerAdapter([PROVIDER_ID], adapter);
  return adapter;
}

export const plugin = { name, inject, apply };
