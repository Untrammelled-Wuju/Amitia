function objectMessage(value: Record<string, unknown>): string {
  const messages: string[] = [];
  const visited = new Set<unknown>();
  const append = (candidate: unknown) => {
    if (candidate == null || visited.has(candidate)) return;
    if (typeof candidate === "object") {
      visited.add(candidate);
      const nested = objectMessage(candidate as Record<string, unknown>);
      if (nested && !messages.includes(nested)) messages.push(nested);
      return;
    }
    if (typeof candidate === "string") {
      const text = candidate.trim();
      if (text && !messages.includes(text)) messages.push(text);
    }
  };
  const response = value.response;
  if (response && typeof response === "object") {
    append((response as Record<string, unknown>).data);
  }
  for (const key of ["message", "msg", "error", "title", "detail", "internalMessage", "reason", "cause"]) {
    if (messages.length > 0 && ["message", "msg", "title"].includes(key)) continue;
    append(value[key]);
  }
  return messages.join("\n");
}

export function originalErrorMessage(error: unknown, fallback = "Unknown error"): string {
  if (typeof error === "string") {
    return error || fallback;
  }
  if (error && typeof error === "object") {
    const message = objectMessage(error as Record<string, unknown>);
    if (message) return message;
    if (error instanceof Error && error.message) return error.message;
    try {
      const encoded = JSON.stringify(error);
      if (encoded && encoded !== "{}") return encoded;
    } catch (error) {
  console.error("[original-error]", error);
}
  }
  if (error instanceof Error && error.message) return error.message;
  const text = error == null ? "" : String(error);
  return text || fallback;
}

export function originalErrorDetail(error: unknown): string | undefined {
  if (!error || typeof error !== "object") return undefined;
  const record = error as Record<string, unknown>;
  const response = record.response;
  const data = response && typeof response === "object"
    ? (response as Record<string, unknown>).data
    : undefined;
  const source = data && typeof data === "object" ? data as Record<string, unknown> : record;
  for (const key of ["detail", "internalMessage", "errorCode", "code", "reason"]) {
    const value = source[key];
    if (typeof value === "string" && value.trim() && value !== originalErrorMessage(error)) return value;
  }
  return undefined;
}
