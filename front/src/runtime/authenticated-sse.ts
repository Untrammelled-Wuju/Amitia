// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { resolveApiUrl } from "./runtime-adapter";
import { createAuthenticatedFetchInit } from "./request-auth";
import { consumeSSEBody } from "./sse-stream";

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
  await consumeSSEBody(response.body, options.signal, options.onEvent);
}
