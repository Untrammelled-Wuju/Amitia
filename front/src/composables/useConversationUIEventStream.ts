import { resolveApiUrl } from "@/runtime/runtime-adapter";
import { createAuthenticatedFetchInit } from "@/runtime/request-auth";
import type { ConversationUIEventRecord } from "@/api/extension";

export interface ConversationUIEventStreamOptions {
  conversationId: string;
  afterSequence?: number;
  onEvent(record: ConversationUIEventRecord): void | Promise<void>;
  onError?(error: unknown): void;
  reconnectDelayMs?: number;
}

export function createConversationUIEventStream(options: ConversationUIEventStreamOptions): () => void {
  const conversationId = options.conversationId.trim();
  if (!conversationId) return () => undefined;
  let cursor = normalizeSequence(options.afterSequence);
  let stopped = false;
  let controller: AbortController | null = null;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  const schedule = () => {
    if (stopped || reconnectTimer) return;
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null;
      void connect();
    }, Math.max(250, options.reconnectDelayMs ?? 1000));
  };

  const connect = async () => {
    if (stopped) return;
    controller?.abort();
    controller = new AbortController();
    try {
      const path = `/api/extensions/events/conversation-ui/${encodeURIComponent(conversationId)}/stream?afterSequence=${cursor}`;
      const url = await resolveApiUrl(path);
      const init = await createAuthenticatedFetchInit(path, {
        headers: { Accept: "text/event-stream" },
        signal: controller.signal,
      });
      const response = await fetch(url, init);
      if (!response.ok || !response.body || !response.headers.get("content-type")?.includes("text/event-stream")) {
        throw new Error(`conversation event stream unavailable (${response.status})`);
      }
      await consumeSSE(response, controller.signal, async (eventName, data) => {
        if (eventName !== "conversation_ui_event" || !data) return;
        const parsed = JSON.parse(data) as ConversationUIEventRecord;
        const sequence = normalizeSequence(parsed.sequence);
        if (sequence <= cursor) return;
        await options.onEvent(parsed);
        cursor = sequence;
      });
      if (!stopped && !controller.signal.aborted) schedule();
    } catch (error) {
      if (stopped || controller?.signal.aborted) return;
      options.onError?.(error);
      schedule();
    }
  };

  void connect();
  return () => {
    stopped = true;
    if (reconnectTimer) clearTimeout(reconnectTimer);
    reconnectTimer = null;
    controller?.abort();
    controller = null;
  };
}

async function consumeSSE(
  response: Response,
  signal: AbortSignal,
  onEvent: (eventName: string, data: string) => void | Promise<void>,
): Promise<void> {
  const reader = response.body!.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  try {
    while (!signal.aborted) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true }).replace(/\r/g, "");
      let boundary = buffer.indexOf("\n\n");
      while (boundary >= 0) {
        const block = buffer.slice(0, boundary);
        buffer = buffer.slice(boundary + 2);
        let eventName = "message";
        const data: string[] = [];
        for (const line of block.split("\n")) {
          if (line.startsWith("event:")) eventName = line.slice(6).trim();
          else if (line.startsWith("data:")) data.push(line.slice(5).trimStart());
        }
        if (data.length > 0) await onEvent(eventName, data.join("\n"));
        boundary = buffer.indexOf("\n\n");
      }
    }
  } finally {
    reader.releaseLock();
  }
}

function normalizeSequence(value: unknown): number {
  const number = Number(value);
  return Number.isSafeInteger(number) && number > 0 ? number : 0;
}
