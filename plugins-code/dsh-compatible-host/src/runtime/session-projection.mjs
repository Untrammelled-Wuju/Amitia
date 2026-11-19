export const SESSION_PROJECTION_VERSION = 1;

const MAX_REASONING_IN_MESSAGE = 200000;

function text(value) {
  return value == null ? "" : String(value).trim();
}

function fail(message, details) {
  const error = new Error(message);
  error.name = "DshSessionProjectionError";
  error.code = details.code;
  error.details = details;
  return error;
}

function jsonArguments(value) {
  if (typeof value === "string") return value;
  if (value === undefined || value === null) return "{}";
  try {
    return JSON.stringify(value);
  } catch (error) {
    throw new Error(`DSH 工具调用参数无法序列化：${String(value)}`, { cause: error });
  }
}

function resultText(payload) {
  if (payload === undefined || payload === null) return "";
  if (typeof payload === "string") return payload;
  for (const key of ["result", "output", "summary", "error", "message", "reason"]) {
    const value = payload[key];
    if (typeof value === "string" && value !== "") return value;
  }
  try {
    return JSON.stringify(payload);
  } catch (error) {
    throw new Error(`DSH 工具结果无法序列化：${String(payload)}`, { cause: error });
  }
}

export function attachmentBlock(attachment) {
  const attachmentId = text(attachment?.attachmentId) || text(attachment?.artifactId) || text(attachment?.sha256);
  const name = text(attachment?.name) || attachmentId;
  if (attachmentId === "" || name === "") {
    throw fail(`DSH 附件缺少 attachmentId 或 name：${JSON.stringify(attachment ?? null)}`, {
      code: "DSH_SESSION_ATTACHMENT_INVALID",
      attachment: attachment ?? null,
    });
  }
  const bytes = Number(attachment?.bytes);
  const mediaType = text(attachment?.mediaType) || text(attachment?.mimeType);
  const uri = text(attachment?.path) || text(attachment?.uri);
  return Object.freeze({
    type: "file",
    attachment: Object.freeze({
      attachmentId,
      name,
      bytes: Number.isSafeInteger(bytes) && bytes >= 0 ? bytes : 0,
      ...(mediaType === "" ? {} : { mediaType }),
      ...(uri === "" ? {} : { uri }),
    }),
  });
}

function userContent(record) {
  const blocks = [];
  const content = typeof record?.contentSummary === "string" ? record.contentSummary : text(record?.content);
  if (content !== "") blocks.push(Object.freeze({ type: "text", text: content }));
  for (const attachment of Array.isArray(record?.attachments) ? record.attachments : []) {
    blocks.push(attachmentBlock(attachment));
  }
  return Object.freeze(blocks);
}

function turnBlocks(turn) {
  const blocks = [];
  for (const block of Array.isArray(turn?.blocks) ? turn.blocks : []) {
    const type = text(block?.type);
    const content = typeof block?.content === "string" ? block.content : "";
    if (type === "text" && content !== "") {
      blocks.push(Object.freeze({ type: "text", text: content }));
      continue;
    }
    if (type === "reasoning" && content !== "") {
      blocks.push(Object.freeze({ type: "reasoning", text: content.slice(0, MAX_REASONING_IN_MESSAGE) }));
      continue;
    }
    if (type === "tool_call") {
      const callId = text(block?.callId);
      if (callId === "") continue;
      blocks.push(Object.freeze({
        type: "tool-call",
        id: callId,
        name: text(block?.payload?.toolName) || text(block?.payload?.name) || "tool",
        arguments: jsonArguments(block?.payload?.arguments),
      }));
    }
  }
  return Object.freeze(blocks);
}

function assistantContent(record, turn) {
  const blocks = turnBlocks(turn);
  if (blocks.length > 0) return blocks;
  const content = typeof record?.contentSummary === "string" ? record.contentSummary : text(record?.content);
  if (content === "") return Object.freeze([]);
  return Object.freeze([Object.freeze({ type: "text", text: content })]);
}

function isToolFailure(block) {
  if (text(block?.status).toLowerCase() === "failed") return true;
  const payload = block?.payload;
  if (!payload || typeof payload !== "object") return false;
  if (payload.error !== undefined && payload.error !== null && text(payload.error) !== "") return true;
  const status = text(payload.status).toUpperCase();
  return ["FAILED", "FAILURE", "ERROR", "DENIED", "REJECTED", "CANCELLED", "CANCELED", "TIMED_OUT", "TIMEOUT", "UNAVAILABLE", "UNKNOWN", "UNCERTAIN"].includes(status);
}

function completedToolCalls(turn) {
  const results = [];
  for (const block of Array.isArray(turn?.blocks) ? turn.blocks : []) {
    if (text(block?.type) !== "tool_call") continue;
    if (text(block?.callId) === "") continue;
    if (block?.payload?.result === undefined || block?.payload?.result === null) continue;
    results.push(block);
  }
  return results;
}

function orderMessages(records) {
  const stamped = records.map((record, index) => {
    const at = Date.parse(typeof record?.timestamp === "string" ? record.timestamp : (typeof record?.createdAt === "string" ? record.createdAt : ""));
    return { record, index, at: Number.isFinite(at) ? at : null };
  });
  if (stamped.some((entry) => entry.at === null)) return records;
  return stamped
    .sort((left, right) => left.at - right.at || left.index - right.index)
    .map((entry) => entry.record);
}

export function buildSessionProjection(input = {}) {
  const conversationId = text(input.conversationId);
  if (conversationId === "") {
    throw fail("DSH 会话投影需要 conversationId", { code: "DSH_SESSION_PROJECTION_CONVERSATION_REQUIRED" });
  }
  const messages = orderMessages(Array.isArray(input.messages) ? input.messages : []);
  const turns = Array.isArray(input.turns) ? input.turns : [];
  const turnsByRequest = new Map();
  for (const turn of turns) {
    const requestId = text(turn?.requestId);
    if (requestId !== "") turnsByRequest.set(requestId, turn);
  }
  const consumedRequests = new Set();
  const events = [];
  let seq = 0;
  let turnNumber = 0;
  let openTurn = null;
  let titleSeq = null;
  const emit = (type, data, surfaceOp) => {
    const event = surfaceOp === undefined
      ? Object.freeze({ type, seq, time: Date.now(), data })
      : Object.freeze({ type, seq, time: Date.now(), data, surfaceOp });
    events.push(event);
    seq += 1;
    return event;
  };
  const closeTurn = (reason) => {
    if (openTurn === null) return;
    emit("step/end", { turn: openTurn, step: 0 });
    emit("turn/end", { turn: openTurn, reason });
    openTurn = null;
  };
  const emitTurnAssistant = (turn, reference) => {
    const content = turnBlocks(turn);
    if (content.length === 0) return false;
    const assistantId = text(turn?.turnId) || reference || `assistant/${turnNumber}`;
    emit("assistant/message", Object.freeze({
      turn: openTurn,
      step: 0,
      message: Object.freeze({
        id: assistantId,
        role: "assistant",
        content,
        source: Object.freeze({ kind: "model", provider: "amitia", model: "amitia-conversation" }),
      }),
      stream: Object.freeze([]),
    }), "append");
    for (const block of completedToolCalls(turn)) {
      const callId = text(block.callId);
      emit("tool/result", Object.freeze({
        turn: openTurn,
        step: 0,
        message: Object.freeze({
          id: `${assistantId}/tool/${callId}`,
          role: "tool",
          toolCallId: callId,
          ...(isToolFailure(block) ? { isError: true } : {}),
          content: Object.freeze([Object.freeze({ type: "text", text: resultText(block.payload) })]),
          source: Object.freeze({ kind: "tool", callId }),
        }),
      }), "append");
    }
    return true;
  };
  for (let index = 0; index < messages.length; index += 1) {
    const record = messages[index];
    if (!record || typeof record !== "object") continue;
    const role = text(record.role).toLowerCase() === "user" ? "user" : "assistant";
    const recordId = text(record.id) || `${role}/${index}`;
    if (role === "user") {
      closeTurn({ kind: "completed" });
      turnNumber += 1;
      openTurn = turnNumber;
      emit("turn/start", { turn: openTurn });
      emit("step/start", { turn: openTurn, step: 0 });
      const userEvent = emit("user/message", Object.freeze({
        id: recordId,
        role: "user",
        content: userContent(record),
        source: Object.freeze({ kind: "user" }),
      }), "append");
      if (titleSeq === null) {
        const title = messagePlainText(userEvent.data).split("\n")[0]?.trim() ?? "";
        if (title !== "") {
          titleSeq = userEvent.seq;
          emit("session/title", { title: title.slice(0, 120), messageSeqs: [userEvent.seq], source: "user" });
        }
      }
      continue;
    }
    if (openTurn === null) {
      turnNumber += 1;
      openTurn = turnNumber;
      emit("turn/start", { turn: openTurn });
      emit("step/start", { turn: openTurn, step: 0 });
    }
    const requestId = text(record.requestId);
    const turn = requestId === "" ? null : (turnsByRequest.get(requestId) ?? null);
    if (turn !== null && requestId !== "") consumedRequests.add(requestId);
    const content = assistantContent(record, turn);
    if (content.length > 0) {
      const assistantId = recordId;
      const modelSource = Object.freeze({
        kind: "model",
        provider: text(record.provider) || "amitia",
        model: text(record.model) || "amitia-conversation",
      });
      emit("assistant/message", Object.freeze({
        turn: openTurn,
        step: 0,
        message: Object.freeze({ id: assistantId, role: "assistant", content, source: modelSource }),
        stream: Object.freeze([]),
        ...(text(record.status).toLowerCase() === "interrupted" ? { interrupted: true } : {}),
      }), "append");
      for (const block of completedToolCalls(turn)) {
        const callId = text(block.callId);
        emit("tool/result", Object.freeze({
          turn: openTurn,
          step: 0,
          message: Object.freeze({
            id: `${assistantId}/tool/${callId}`,
            role: "tool",
            toolCallId: callId,
            ...(isToolFailure(block) ? { isError: true } : {}),
            content: Object.freeze([Object.freeze({ type: "text", text: resultText(block.payload) })]),
            source: Object.freeze({ kind: "tool", callId }),
          }),
        }), "append");
      }
    }
    closeTurn({ kind: "completed" });
  }
  closeTurn({ kind: "completed" });
  for (const turn of turns) {
    const requestId = text(turn?.requestId);
    if (requestId !== "" && consumedRequests.has(requestId)) continue;
    const content = turnBlocks(turn);
    if (content.length === 0) continue;
    turnNumber += 1;
    openTurn = turnNumber;
    emit("turn/start", { turn: openTurn });
    emit("step/start", { turn: openTurn, step: 0 });
    if (requestId !== "") consumedRequests.add(requestId);
    emitTurnAssistant(turn, text(turn?.turnId));
    closeTurn({ kind: "completed" });
  }
  return Object.freeze({
    conversationId,
    header: Object.freeze({ id: conversationId, ...(text(input.cwd) === "" ? {} : { cwd: text(input.cwd) }) }),
    events: Object.freeze(events),
  });
}

function projectable(event) {
  const data = event?.data;
  if (!data || typeof data !== "object") return null;
  if (event.type === "user/message") return data;
  if (event.type === "assistant/message") return data.message ?? null;
  if (event.type === "tool/result") return data.message ?? null;
  if (event.type === "system/message") return data.message ?? null;
  if (event.type === "developer/message") return data.message ?? null;
  return null;
}

export function foldSessionSurface(events, projections = []) {
  const list = Array.isArray(events) ? events : [];
  const nodes = [];
  const messages = new Map();
  const replacements = [];
  for (let index = 0; index < list.length; index += 1) {
    const event = list[index];
    if (!event || typeof event !== "object") continue;
    const message = projectable(event);
    if (message !== null && Array.isArray(message.content) && message.content.length > 0) {
      const op = event.surfaceOp;
      if (op && typeof op === "object" && op.op === "replace") {
        const startIdx = nodes.indexOf(Number(op.startSeq));
        const endIdx = nodes.indexOf(Number(op.endSeq));
        if (startIdx < 0 || endIdx < startIdx) {
          throw fail(`DSH 会话事件 ${event.seq} 的替换范围 ${op.startSeq}..${op.endSeq} 不在当前表面上`, {
            code: "DSH_SESSION_SURFACE_RANGE_INVALID",
            seq: event.seq,
            surfaceOp: op,
          });
        }
        for (const removed of nodes.slice(startIdx, endIdx + 1)) messages.delete(removed);
        nodes.splice(startIdx, endIdx - startIdx + 1);
        replacements.push(Object.freeze({ seq: event.seq, startSeq: Number(op.startSeq), endSeq: Number(op.endSeq) }));
      }
      nodes.push(Number(event.seq));
      messages.set(Number(event.seq), message);
    }
    for (const projection of projections) {
      if (!projection || projection.type !== event.type || typeof projection.project !== "function") continue;
      const updated = projection.project(event, Object.freeze({
        nodes: Object.freeze(nodes.slice()),
        events: Object.freeze(list.slice(0, index + 1)),
        baseSeq: 0,
        messages: new Map(messages),
      }));
      if (!(updated instanceof Map)) {
        throw fail(`DSH 消息投影 ${event.type} 必须返回 Map<seq, Message>`, {
          code: "DSH_SESSION_PROJECTION_RESULT_INVALID",
          seq: event.seq,
          type: event.type,
        });
      }
      for (const [key, value] of updated) {
        if (!messages.has(Number(key))) {
          throw fail(`DSH 消息投影 ${event.type} 改写了不在当前表面上的消息 ${String(key)}`, {
            code: "DSH_SESSION_PROJECTION_TARGET_INVALID",
            seq: event.seq,
            type: event.type,
            target: Number(key),
          });
        }
        messages.set(Number(key), value);
      }
    }
  }
  return Object.freeze({ nodes: Object.freeze(nodes), messages, replacements: Object.freeze(replacements) });
}

export function messagePlainText(message) {
  const parts = [];
  for (const block of Array.isArray(message?.content) ? message.content : []) {
    if (block?.type === "text" || block?.type === "reasoning") parts.push(String(block.text ?? ""));
    else if (block?.type === "tool-call") parts.push(`[tool ${block.name} ${block.id}]`);
    else if (block?.type === "file") parts.push(`[file ${block.attachment?.name ?? ""}]`);
  }
  return parts.join("\n").trim();
}

export function foldSessionTitle(events) {
  for (const event of Array.isArray(events) ? events : []) {
    if (event?.type !== "user/message") continue;
    const title = (messagePlainText(event.data).split("\n")[0] ?? "").trim();
    if (title !== "") return title.slice(0, 120);
  }
  return "";
}
