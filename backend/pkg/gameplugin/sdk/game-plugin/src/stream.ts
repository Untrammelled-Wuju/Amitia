import { Client, MessageOption } from './client';

export const METHOD_STREAM_OPEN = 'stream.open';
export const METHOD_STREAM_WRITE = 'stream.write';
export const METHOD_STREAM_READ = 'stream.read';
export const METHOD_STREAM_CLOSE = 'stream.close';
export const METHOD_STREAM_CURSOR = 'stream.cursor';

export interface StreamCursor {
  runtimeId: string;
  serviceId: string;
  channelId: string;
  generation: string;
  sequence: number;
}

export interface StreamOpenInput {
  channelId: string;
  cursor?: StreamCursor;
}

export interface StreamOpenOutput {
  streamId: string;
  generation: string;
  cursor: StreamCursor;
}

export interface StreamWriteInput {
  streamId: string;
  data: unknown;
}

export interface StreamWriteOutput {
  sequence: number;
}

export interface StreamReadInput {
  streamId: string;
  cursor: StreamCursor;
  limit?: number;
}

export interface StreamFrame {
  sequence: number;
  data: unknown;
}

export interface StreamReadOutput {
  items: StreamFrame[];
  cursor: StreamCursor;
  hasMore: boolean;
}

export interface StreamCloseInput {
  streamId: string;
}

export interface StreamCursorInput {
  streamId: string;
}

export interface StreamCursorOutput {
  cursor: StreamCursor;
}

export async function streamOpen(
  client: Client,
  input: StreamOpenInput,
  opts: MessageOption[] = []
): Promise<StreamOpenOutput> {
  const payload: Record<string, unknown> = {
    channelId: input.channelId,
  };
  if (input.cursor) {
    payload.cursor = input.cursor;
  }
  const envelope = await client.sendRequest(METHOD_STREAM_OPEN, payload, ...opts);
  return envelope.payload as StreamOpenOutput;
}

export async function streamWrite(
  client: Client,
  input: StreamWriteInput,
  opts: MessageOption[] = []
): Promise<StreamWriteOutput> {
  const envelope = await client.sendRequest(METHOD_STREAM_WRITE, input, ...opts);
  return envelope.payload as StreamWriteOutput;
}

export async function streamRead(
  client: Client,
  input: StreamReadInput,
  opts: MessageOption[] = []
): Promise<StreamReadOutput> {
  const payload: Record<string, unknown> = {
    streamId: input.streamId,
    cursor: input.cursor,
  };
  if (input.limit) {
    payload.limit = input.limit;
  }
  const envelope = await client.sendRequest(METHOD_STREAM_READ, payload, ...opts);
  return envelope.payload as StreamReadOutput;
}

export async function streamClose(
  client: Client,
  input: StreamCloseInput,
  opts: MessageOption[] = []
): Promise<void> {
  await client.sendRequest(METHOD_STREAM_CLOSE, input, ...opts);
}

export async function streamCursor(
  client: Client,
  input: StreamCursorInput,
  opts: MessageOption[] = []
): Promise<StreamCursorOutput> {
  const envelope = await client.sendRequest(METHOD_STREAM_CURSOR, input, ...opts);
  return envelope.payload as StreamCursorOutput;
}
