import { normalizeMessageState } from "./amrp";

const runtimeErrorFields = [
  "provider",
  "errorType",
  "errorCode",
  "stage",
  "toolName",
  "userMessage",
  "internalMessage",
  "message",
  "error",
  "detail",
  "cause",
  "rawError",
  "stackTrace",
  "stack",
];

function toErrorText(value: unknown): string {
  if (value === undefined || value === null) return "";
  if (typeof value === "string") return value;
  try {
    return JSON.stringify(value, null, 2);
  } catch (error) {
    console.error("[original-error]", error);
    return `${String(error)}\n${String(value)}`;
  }
}

export function errorNoticeText(detail: string): string {
  return `-${detail}-`;
}

export function formatTurnErrorDetail(turn: any, fallback = ""): string {
  const item = [...(turn?.items || [])]
    .reverse()
    .find((candidate) => candidate?.type === "error");
  if (!item) return fallback.trim() || "任务执行失败，未返回具体原因";

  let payload: Record<string, unknown>;
  try {
    const parsed = JSON.parse(String(item.resultJson || "{}"));
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return item.content || item.errorCode || fallback.trim()
        || "任务执行失败，未返回具体原因";
    }
    payload = parsed as Record<string, unknown>;
  } catch (error) {
    console.error("[original-error]", error);
    return [item.content, item.resultJson, String(error)].filter(Boolean).join("\n\n");
  }

  const values: string[] = [];
  for (const field of runtimeErrorFields) {
    const text = toErrorText(payload[field]);
    if (text.trim()) values.push(`${field}: ${text}`);
  }
  if (!values.length) {
    const value = item.content || item.errorCode || fallback.trim();
    if (value) values.push(String(value));
  }
  if (Object.keys(payload).length) {
    try {
      values.push(`originalPayload:\n${JSON.stringify(payload, null, 2)}`);
    } catch (error) {
      console.error("[original-error]", error);
      values.push(`originalPayload:\n${String(error)}\n${String(payload)}`);
    }
  }
  return values.join("\n\n");
}

export function hasRenderableMessageContent(message: Record<string, any> | undefined): boolean {
  if (!message) return false;
  if (String(message.content || message.markdown || "").trim()) return true;
  return Array.isArray(message.blocks) && message.blocks.length > 0;
}

export function shouldShowTurnErrorNotice(
  message: Record<string, any> | undefined,
  modelError = "",
): boolean {
  if (!message) return false;
  const state = normalizeMessageState(message);
  if (message.role === "user") return state === "failed";
  if (message.role !== "assistant") return false;
  if (state === "failed") return true;
  return (
    state === "interrupted" &&
    !hasRenderableMessageContent(message) &&
    Boolean(String(modelError || "").trim())
  );
}
