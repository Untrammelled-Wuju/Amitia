import { normalizeCitationSnapshot, normalizeCitationURL } from "@/conversation/runtime/citationSnapshot";
import type {
  AgentTaskBlock,
  AIMessageData,
  CitationSource,
  MessageState,
  RichBlock,
  ToolBlock,
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
  if (raw === "streaming") return "streaming";
  if (raw === "interrupted") return "interrupted";
  if (["failed", "error"].includes(raw)) return "failed";
  return "completed";
}

function normalizeStatus(value: unknown): ToolBlock["status"] {
  const raw = String(value ?? "").toLowerCase();
  if (raw === "queued") return "queued";
  if (raw === "running") return "running";
  if (["failed", "error"].includes(raw)) return "failed";
  if (raw === "interrupted") return "interrupted";
  return "success";
}

function normalizeBlock(raw: Record<string, any>, index: number): RichBlock {
  const type = String(raw.kind ?? "").toLowerCase();
  const id = firstString(raw, ["id"]) || `block-${index}`;
  if (type.includes("tool")) {
    return {
      kind: "tool",
      id,
      name: firstString(raw, ["name"]) || "工具调用",
      arguments: raw.arguments,
      result: raw.result,
      duration: Number(raw.duration ?? 0) || undefined,
      error: firstString(raw, ["error"]) || undefined,
      status: normalizeStatus(raw.status),
    };
  }
  if (type.includes("file")) {
    return {
      kind: "file",
      id,
      name: firstString(raw, ["name"]) || "未命名文件",
      mimeType: firstString(raw, ["mimeType"]) || undefined,
      size: Number(raw.size ?? 0) || undefined,
      url: firstString(raw, ["url"]) || undefined,
      downloadUrl: firstString(raw, ["downloadUrl"]) || undefined,
      status: String(raw.status ?? "ready").toLowerCase() === "failed"
        ? "failed"
        : String(raw.status ?? "ready").toLowerCase() === "loading"
          ? "loading"
          : "ready",
      error: firstString(raw, ["error"]) || undefined,
    };
  }
  if (type.includes("image")) {
    return {
      kind: "image",
      id,
      url: firstString(raw, ["url"]),
      downloadUrl: firstString(raw, ["downloadUrl"]) || undefined,
      alt: firstString(raw, ["alt"]) || undefined,
      mimeType: firstString(raw, ["mimeType"]) || undefined,
      animated: Boolean(raw.animated),
      width: Number(raw.width ?? 0) || undefined,
      height: Number(raw.height ?? 0) || undefined,
      status: "loading",
    };
  }
  if (type.includes("audio") || type.includes("voice")) {
    return {
      kind: "audio",
      id,
      url: firstString(raw, ["url"]),
      downloadUrl: firstString(raw, ["downloadUrl"]) || undefined,
      title: firstString(raw, ["title"]) || "语音",
      duration: Number(raw.duration ?? 0) || undefined,
      status: "loading",
    };
  }
  if (type.includes("video")) {
    return {
      kind: "video",
      id,
      url: firstString(raw, ["url"]),
      downloadUrl: firstString(raw, ["downloadUrl"]) || undefined,
      title: firstString(raw, ["title"]) || "视频",
      poster: firstString(raw, ["poster"]) || undefined,
      duration: Number(raw.duration ?? 0) || undefined,
      status: "loading",
    };
  }
  if (type.includes("artifact")) {
    return {
      kind: "artifact",
      id,
      artifactKind: firstString(raw, ["artifactKind"]) || "document",
      title: firstString(raw, ["title"]) || "Artifact",
      mimeType: firstString(raw, ["mimeType"]) || undefined,
      content: typeof raw.content === "string" ? raw.content : undefined,
      fileId: firstString(raw, ["fileId"]) || undefined,
      url: firstString(raw, ["url"]) || undefined,
      downloadUrl: firstString(raw, ["downloadUrl"]) || undefined,
      size: Number(raw.size ?? 0) || undefined,
    };
  }
  if (type.includes("agent") || type.includes("task") || type.includes("workflow")) {
    const steps = Array.isArray(raw.steps)
      ? raw.steps.map((step: any, stepIndex: number) => ({
          id: String(step?.id ?? `step-${stepIndex}`),
          title: firstString(step ?? {}, ["title"]) || `Step ${stepIndex + 1}`,
          status: normalizeStatus(step?.status) === "success"
            ? "done" as const
            : normalizeStatus(step?.status) === "failed"
              ? "failed" as const
              : normalizeStatus(step?.status) === "running"
                ? "running" as const
                : "pending" as const,
          meta: firstString(step ?? {}, ["meta"]) || undefined,
        }))
      : [];
    const status = normalizeStatus(raw.status);
    return {
      kind: "agent-task",
      id,
      title: firstString(raw, ["title"]) || "Agent Task",
      status: status === "success" ? "done" : status === "failed" ? "failed" : status === "running" ? "running" : "pending",
      steps,
      progress: Number(raw.progress ?? 0) || undefined,
      elapsed: firstString(raw, ["elapsed"]) || undefined,
      error: firstString(raw, ["error"]) || undefined,
    } satisfies AgentTaskBlock;
  }
  if (type.includes("extension")) {
    return {
      kind: "extension",
      id,
      rendererId: firstString(raw, ["rendererId"]) || "unknown.renderer",
      version: Number(raw.version ?? 1),
      payload: raw.payload,
    };
  }
  return {
    kind: "unknown",
    id,
    type: type || "unknown",
    payload: raw,
  };
}

function messageBlocks(message: Record<string, any>): RichBlock[] {
  const blocks: RichBlock[] = [];
  if (Array.isArray(message.blocks)) {
    blocks.push(...message.blocks
      .filter((item: unknown): item is Record<string, any> => !!item && typeof item === "object")
      .map(normalizeBlock));
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
  const raw = Array.isArray(message.citationSnapshot)
    ? normalizeCitationSnapshot(message.citationSnapshot)
    : [];
  if (!Array.isArray(raw)) return [];
  return raw
    .filter((item: unknown): item is Record<string, any> => !!item && typeof item === "object")
    .map((item, index) => ({
      id: firstString(item, ["id", "marker"]).replace(/^\[|\]$/g, "") || (item.kind === "knowledge" ? `K${item.index}` : firstString(item, ["index"])) || String(index + 1),
      title: firstString(item, ["title", "name", "url", "fileId"]) || `Source ${index + 1}`,
      url: normalizeCitationURL(firstString(item, ["url", "href"])) || undefined,
      fileId: firstString(item, ["fileId"]) || undefined,
      snippet: firstString(item, ["excerpt", "snippet", "text", "description"]) || undefined,
    }));
}

export function normalizeAIMessage(
  input: Record<string, any>,
  character?: { id?: string; name?: string; avatar?: string },
): AIMessageData {
  const message = input ?? {};
  const role = normalizeRole(message.role);
  const markdown = typeof message.content === "string" ? message.content : "";
  const thinkingContent = firstString(message, ["reasoningContent"]);
  const reasoningDurationMs = Number(message.reasoningDurationMs ?? 0);
  const thinkingDurationSeconds =
    Number.isFinite(reasoningDurationMs) && reasoningDurationMs > 0 ? reasoningDurationMs / 1000 : undefined;
  const state = normalizeMessageState(message);
  return {
    id: String(message.id ?? `message-${Date.now()}`),
    conversationId: firstString(message, ["conversationId"]) || undefined,
    role,
    character: character
      ? {
          id: String(character.id ?? ""),
          name: String(character.name ?? "Amitia"),
          avatar: character.avatar,
        }
      : undefined,
    markdown,
    thinking: thinkingContent
      ? {
          content: thinkingContent,
          state: message.reasoningComplete === true || (message.reasoningComplete !== false && markdown.trim()) ? "completed" : state,
          duration: thinkingDurationSeconds,
        }
      : undefined,
    blocks: messageBlocks(message),
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
  } catch (error) {
  console.error("[original-error]", error);
return String(value);
  }
}

export function aimMessagePlainText(message: AIMessageData): string {
  return [message.markdown, message.thinking?.content, ...message.blocks.map(richBlockText)]
    .filter((value) => String(value ?? "").trim())
    .join("\n\n");
}
