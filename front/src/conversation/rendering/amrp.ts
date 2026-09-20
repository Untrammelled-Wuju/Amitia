import type {
  AgentTaskBlock,
  AIMessageData,
  AudioBlock,
  CitationSource,
  FileBlock,
  ImageBlock,
  MessageState,
  RichBlock,
  ToolBlock,
  VideoBlock,
} from "./types";

function firstString(source: Record<string, any>, keys: string[]): string {
  for (const key of keys) {
    const value = source[key];
    if (value !== undefined && value !== null && String(value).trim()) {
      return String(value).trim();
    }
  }
  return "";
}

function normalizeRole(role: unknown): AIMessageData["role"] {
  const value = String(role ?? "").toLowerCase();
  if (value === "user") return "user";
  if (value === "system") return "system";
  return "assistant";
}

export function normalizeMessageState(message: Record<string, any>): MessageState {
  const raw = String(message.state ?? message.status ?? "").toLowerCase();
  if (["queued", "pending"].includes(raw)) return "queued";
  if (["streaming", "sending", "generating", "collecting", "processing"].includes(raw)) {
    return "streaming";
  }
  if (["interrupted", "paused"].includes(raw)) return "interrupted";
  if (["failed", "error"].includes(raw)) return "failed";
  if (["cancelled", "canceled", "stopped"].includes(raw)) return "cancelled";
  if (message.typingStart && message.typingDone !== true) return "streaming";
  return "completed";
}

function normalizeStatus(value: unknown): ToolBlock["status"] {
  const raw = String(value ?? "").toLowerCase();
  if (["queued", "pending"].includes(raw)) return "queued";
  if (["running", "streaming", "sending"].includes(raw)) return "running";
  if (["failed", "error"].includes(raw)) return "failed";
  if (["cancelled", "canceled", "stopped"].includes(raw)) return "cancelled";
  return "success";
}

function parseJsonValue(value: unknown): unknown {
  if (typeof value !== "string") return value;
  const trimmed = value.trim();
  if (!trimmed || (!trimmed.startsWith("{") && !trimmed.startsWith("["))) return value;
  try {
    return JSON.parse(trimmed);
  } catch {
    return value;
  }
}

function normalizeBlock(raw: Record<string, any>, index: number): RichBlock {
  const type = String(raw.kind ?? raw.type ?? raw.blockType ?? "").toLowerCase();
  const id = firstString(raw, ["id", "blockId"]) || `block-${index}`;
  if (type.includes("tool")) {
    return {
      kind: "tool",
      id,
      name: firstString(raw, ["name", "toolName", "title"]) || "工具调用",
      arguments: raw.arguments ?? raw.args ?? raw.input,
      result: raw.result ?? raw.output ?? raw.content,
      duration: Number(raw.duration ?? raw.durationMs ?? 0) || undefined,
      error: firstString(raw, ["error", "errorCode", "errorMessage"]) || undefined,
      status: normalizeStatus(raw.status ?? raw.state),
    };
  }
  if (type.includes("file") || type.includes("attachment")) {
    return {
      kind: "file",
      id,
      name: firstString(raw, ["name", "fileName", "title"]) || "未命名文件",
      mimeType: firstString(raw, ["mimeType", "mime"]) || undefined,
      size: Number(raw.size ?? raw.sizeBytes ?? raw.fileSize ?? 0) || undefined,
      url: firstString(raw, ["url", "resourceUri", "href"]) || undefined,
      status: String(raw.status ?? "ready").toLowerCase() === "failed"
        ? "failed"
        : String(raw.status ?? "ready").toLowerCase() === "loading"
          ? "loading"
          : "ready",
      error: firstString(raw, ["error", "errorMessage"]) || undefined,
    };
  }
  if (type.includes("image")) {
    return {
      kind: "image",
      id,
      url: firstString(raw, ["url", "imageUrl", "src"]),
      alt: firstString(raw, ["alt", "altText", "title"]) || undefined,
      mimeType: firstString(raw, ["mimeType", "mime"]) || undefined,
      animated: Boolean(raw.animated ?? raw.isAnimated),
      width: Number(raw.width ?? 0) || undefined,
      height: Number(raw.height ?? 0) || undefined,
      status: "loading",
    };
  }
  if (type.includes("audio") || type.includes("voice")) {
    return {
      kind: "audio",
      id,
      url: firstString(raw, ["url", "audioUrl", "src"]),
      title: firstString(raw, ["title", "fileName", "name"]) || "语音",
      duration: Number(raw.duration ?? raw.durationMs ?? 0) || undefined,
      status: "loading",
    };
  }
  if (type.includes("video")) {
    return {
      kind: "video",
      id,
      url: firstString(raw, ["url", "videoUrl", "src"]),
      title: firstString(raw, ["title", "fileName", "name"]) || "视频",
      poster: firstString(raw, ["poster", "posterUrl"]) || undefined,
      duration: Number(raw.duration ?? raw.durationMs ?? 0) || undefined,
      status: "loading",
    };
  }
  if (type.includes("artifact")) {
    return {
      kind: "artifact",
      id,
      artifactKind: firstString(raw, ["artifactKind", "artifactType", "kindName"]) || "document",
      title: firstString(raw, ["title", "name"]) || "Artifact",
      mimeType: firstString(raw, ["mimeType", "mime"]) || undefined,
      content: typeof raw.content === "string" ? raw.content : undefined,
      fileId: firstString(raw, ["fileId"]) || undefined,
      url: firstString(raw, ["url", "href"]) || undefined,
      size: Number(raw.size ?? raw.sizeBytes ?? 0) || undefined,
    };
  }
  if (type.includes("agent") || type.includes("task") || type.includes("workflow")) {
    const steps = Array.isArray(raw.steps)
      ? raw.steps.map((step: any, stepIndex: number) => ({
          id: String(step?.id ?? `step-${stepIndex}`),
          title: firstString(step ?? {}, ["title", "name", "label"]) || `Step ${stepIndex + 1}`,
          status: normalizeStatus(step?.status ?? step?.state) === "success"
            ? "done" as const
            : normalizeStatus(step?.status ?? step?.state) === "failed"
              ? "failed" as const
              : normalizeStatus(step?.status ?? step?.state) === "running"
                ? "running" as const
                : "pending" as const,
          meta: firstString(step ?? {}, ["meta", "statusLabel", "duration"]) || undefined,
        }))
      : [];
    const status = normalizeStatus(raw.status ?? raw.state);
    return {
      kind: "agent-task",
      id,
      title: firstString(raw, ["title", "name", "taskTitle"]) || "Agent Task",
      status: status === "success" ? "done" : status === "failed" ? "failed" : status === "running" ? "running" : "pending",
      steps,
      progress: Number(raw.progress ?? raw.percentage ?? 0) || undefined,
      elapsed: firstString(raw, ["elapsed", "elapsedTime"]) || undefined,
      error: firstString(raw, ["error", "errorMessage"]) || undefined,
    } satisfies AgentTaskBlock;
  }
  if (type.includes("extension") || firstString(raw, ["rendererId"])) {
    return {
      kind: "extension",
      id,
      rendererId: firstString(raw, ["rendererId"]) || "unknown.renderer",
      version: Number(raw.version ?? 1),
      payload: raw.payload ?? raw.data,
    };
  }
  return {
    kind: "unknown",
    id,
    type: type || "unknown",
    payload: raw,
  };
}

function legacyBlocks(message: Record<string, any>): RichBlock[] {
  if (Array.isArray(message.blocks)) {
    return message.blocks
      .filter((item: unknown): item is Record<string, any> => !!item && typeof item === "object")
      .map(normalizeBlock);
  }
  const blocks: RichBlock[] = [];
  const type = String(message.msgType ?? message.messageType ?? message.type ?? "text").toLowerCase();
  const commonId = String(message.id ?? `legacy-${Date.now()}`);
  if (type === "tool_call" || type === "toolcall" || type === "tool") {
    blocks.push({
      kind: "tool",
      id: `${commonId}-tool`,
      name: firstString(message, ["toolName", "name", "title"]) || "工具调用",
      arguments: parseJsonValue(message.toolArguments ?? message.arguments),
      result: parseJsonValue(message.toolResult ?? message.result ?? message.content),
      duration: Number(message.duration ?? message.durationMs ?? 0) || undefined,
      error: firstString(message, ["error", "errorMessage"]) || undefined,
      status: normalizeStatus(message.status),
    });
  }
  if (type === "agent_task" || type === "agenttask" || type === "agent-task") {
    blocks.push(normalizeBlock({ ...message, kind: "agent-task" }, 0));
  }
  if (type === "file") {
    blocks.push({
      kind: "file",
      id: `${commonId}-file`,
      name: firstString(message, ["fileName", "name"]) || "未命名文件",
      mimeType: firstString(message, ["mimeType", "mime"]) || undefined,
      size: Number(message.fileSizeBytes ?? message.sizeBytes ?? message.fileSize ?? 0) || undefined,
      url: firstString(message, ["resourceUri", "url", "href"]) || undefined,
      status: String(message.status ?? "").toLowerCase() === "failed" ? "failed" : "ready",
      error: firstString(message, ["error", "errorMessage"]) || undefined,
    } satisfies FileBlock);
  }
  if (type === "image" || firstString(message, ["imageUrl"])) {
    const imageUrl = firstString(message, ["imageUrl", "image_url"]);
    if (imageUrl) {
      blocks.push({
        kind: "image",
        id: `${commonId}-image`,
        url: imageUrl,
        alt: firstString(message, ["altText", "alt"]) || undefined,
        mimeType: firstString(message, ["mimeType", "mime"]) || undefined,
        animated: Boolean(message.isAnimated ?? message.is_animated),
        width: Number(message.width ?? message.media_width ?? 0) || undefined,
        height: Number(message.height ?? message.media_height ?? 0) || undefined,
        status: "loading",
      } satisfies ImageBlock);
    }
  }
  if (type === "audio" || type === "voice" || firstString(message, ["audioUrl"])) {
    const audioUrl = firstString(message, ["audioUrl", "audio_url"]);
    if (audioUrl) {
      blocks.push({
        kind: "audio",
        id: `${commonId}-audio`,
        url: audioUrl,
        title: firstString(message, ["fileName", "name"]) || "语音",
        duration: Number(message.audioDuration ?? 0) || undefined,
        status: "loading",
      } satisfies AudioBlock);
    }
  }
  if (type === "video" || firstString(message, ["videoUrl"])) {
    const videoUrl = firstString(message, ["videoUrl", "video_url"]);
    if (videoUrl) {
      blocks.push({
        kind: "video",
        id: `${commonId}-video`,
        url: videoUrl,
        title: firstString(message, ["fileName", "name"]) || "视频",
        poster: firstString(message, ["poster", "posterUrl"]) || undefined,
        duration: Number(message.videoDuration ?? 0) || undefined,
        status: "loading",
      } satisfies VideoBlock);
    }
  }
  if (Array.isArray(message.attachments)) {
    for (const attachment of message.attachments) {
      if (attachment && typeof attachment === "object") {
        blocks.push(normalizeBlock(attachment, blocks.length));
      }
    }
  }
  return blocks;
}

function normalizeSources(message: Record<string, any>): CitationSource[] {
  const raw = message.sources ?? message.citations;
  if (!Array.isArray(raw)) return [];
  return raw
    .filter((item: unknown): item is Record<string, any> => !!item && typeof item === "object")
    .map((item, index) => ({
      id: firstString(item, ["id", "index"]) || String(index + 1),
      title: firstString(item, ["title", "name", "url", "fileId"]) || `Source ${index + 1}`,
      url: firstString(item, ["url", "href"]) || undefined,
      fileId: firstString(item, ["fileId"]) || undefined,
      snippet: firstString(item, ["snippet", "text", "description"]) || undefined,
    }));
}

export function normalizeAIMessage(
  input: Record<string, any>,
  character?: { id?: string; name?: string; avatar?: string },
): AIMessageData {
  const message = input ?? {};
  const role = normalizeRole(message.role);
  const type = String(message.msgType ?? message.messageType ?? message.type ?? "text").toLowerCase();
  let markdown = typeof message.content === "string" ? message.content : String(message.text ?? "");
  if ((type === "image" && markdown === "[图片]") || (type === "video" && markdown === "[视频]")) {
    markdown = "";
  }
  if (["tool_call", "toolcall", "tool", "agent_task", "agenttask", "agent-task"].includes(type)) {
    markdown = "";
  }
  const reasoningContent = firstString(message, ["reasoningContent", "reasoning_content"]);
  const rawThinking = message.thinking;
  const thinkingContent =
    rawThinking && typeof rawThinking === "object"
      ? firstString(rawThinking, ["content", "text"])
      : reasoningContent;
  const thinkingDuration = Number(rawThinking?.duration ?? message.reasoningDuration ?? 0);
  const reasoningDurationMs = Number(message.reasoningDurationMs ?? 0);
  const thinkingDurationSeconds =
    Number.isFinite(thinkingDuration) && thinkingDuration > 0
      ? thinkingDuration
      : Number.isFinite(reasoningDurationMs) && reasoningDurationMs > 0
        ? reasoningDurationMs / 1000
        : undefined;
  const state = normalizeMessageState(message);
  return {
    id: String(message.id ?? `message-${Date.now()}`),
    conversationId: firstString(message, ["conversationId", "conversation_id"]) || undefined,
    role,
    character: character
      ? {
          id: String(character.id ?? ""),
          name: String(character.name ?? "Amitia"),
          avatar: character.avatar,
        }
      : undefined,
    markdown,
    thinking:
      thinkingContent || message.generationPending === true
        ? {
            content: thinkingContent,
            state: message.generationPending === true ? "streaming" : state,
            duration: thinkingDurationSeconds,
          }
        : undefined,
    blocks: legacyBlocks(message),
    sources: normalizeSources(message),
    state,
    createdAt: Date.parse(String(message.createdAt ?? "")) || Number(message.createdAt) || Date.now(),
    raw: message,
  };
}

export function richBlockText(block: RichBlock): string {
  switch (block.kind) {
    case "tool":
      return [block.name, formatUnknown(block.arguments), formatUnknown(block.result), block.error]
        .filter(Boolean)
        .join("\n");
    case "file":
      return [block.name, block.mimeType, block.url].filter(Boolean).join("\n");
    case "image":
      return block.alt || block.url;
    case "audio":
    case "video":
      return [block.title, block.url].filter(Boolean).join("\n");
    case "artifact":
      return [block.title, block.artifactKind, block.content].filter(Boolean).join("\n");
    case "agent-task":
      return [block.title, ...block.steps.map((step) => `${step.title} ${step.meta ?? ""}`)].join("\n");
    case "extension":
      return `${block.rendererId}\n${formatUnknown(block.payload)}`;
    case "unknown":
      return formatUnknown(block.payload);
  }
}

export function formatUnknown(value: unknown): string {
  if (value === undefined || value === null) return "";
  if (typeof value === "string") return value;
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

export function aimMessagePlainText(message: AIMessageData): string {
  return [message.markdown, message.thinking?.content, ...message.blocks.map(richBlockText)]
    .filter((value) => String(value ?? "").trim())
    .join("\n\n");
}
