// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { resolveApiUrl } from "./runtime-adapter";
import { createAuthenticatedFetchInit } from "./request-auth";

export interface AuthenticatedSSEEvent {
  event: string;
  data: string;
  id?: string;
}

export interface ConsumeAuthenticatedSSEOptions {
  signal: AbortSignal;
  onOpen?: () => void;
  onEvent: (event: AuthenticatedSSEEvent) => void | Promise<void>;
}

export async function consumeAuthenticatedSSE(
  path: string,
  options: ConsumeAuthenticatedSSEOptions,
): Promise<void> {
  const url = await resolveApiUrl(path);
  const init = await createAuthenticatedFetchInit(path, {
    method: "GET",
    headers: { Accept: "text/event-stream" },
    cache: "no-store",
    signal: options.signal,
  });
  const response = await fetch(url, init);
  if (!response.ok) {
    throw new Error(`SSE HTTP ${response.status}`);
  }
  const contentType = response.headers.get("content-type") ?? "";
  if (!contentType.toLowerCase().includes("text/event-stream") || !response.body) {
    throw new Error("SSE response is not text/event-stream");
  }

  options.onOpen?.();
  await consumeSSEBody(response.body, options);
}

async function consumeSSEBody(
  body: ReadableStream<Uint8Array>,
  options: ConsumeAuthenticatedSSEOptions,
): Promise<void> {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  try {
    while (!options.signal.aborted) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true }).replace(/\r/g, "");

      let boundary = buffer.indexOf("\n\n");
      while (boundary >= 0) {
        const block = buffer.slice(0, boundary);
        buffer = buffer.slice(boundary + 2);
        const parsed = parseBlock(block);
        if (parsed) await options.onEvent(parsed);
        boundary = buffer.indexOf("\n\n");
      }
    }
  } finally {
    reader.releaseLock();
  }
}

function parseBlock(block: string): AuthenticatedSSEEvent | null {
  let event = "message";
  let id: string | undefined;
  const data: string[] = [];

  for (const line of block.split("\n")) {
    if (!line || line.startsWith(":")) continue;
    const separator = line.indexOf(":");
    const field = separator >= 0 ? line.slice(0, separator) : line;
    let value = separator >= 0 ? line.slice(separator + 1) : "";
    if (value.startsWith(" ")) value = value.slice(1);

    switch (field) {
      case "event":
        event = value || "message";
        break;
      case "data":
        data.push(value);
        break;
      case "id":
        id = value;
        break;
      default:
        break;
    }
  }

  if (data.length === 0) return null;
  return { event, data: data.join("\n"), id };
}
