import { Client, MessageOption } from './client';
import { channelPublish } from './channel';
import { Envelope } from './protocol';

export const METHOD_BINARY_CREATE = 'binary.create';
export const METHOD_BINARY_WRITE = 'binary.write';
export const METHOD_BINARY_SEAL = 'binary.seal';
export const METHOD_BINARY_READ = 'binary.read';
export const METHOD_BINARY_STAT = 'binary.stat';
export const METHOD_BINARY_RELEASE = 'binary.release';
export const METHOD_BINARY_ABORT = 'binary.abort';

export const DEFAULT_BINARY_CHUNK_SIZE = 512 * 1024;
export const MAX_BINARY_CHUNK_SIZE = 2 * 1024 * 1024;

export type BinaryStorageKind = 'file';
export type BinaryLifetime = 'message' | 'runtime';

export interface BinaryChecksum {
  algorithm: 'sha256' | string;
  value: string;
}

export interface BinaryReference {
  id: string;
  kind: BinaryStorageKind;
  size: number;
  mediaType?: string;
  checksum?: BinaryChecksum;
  lifetime: BinaryLifetime;
  metadata?: Record<string, unknown>;
}

export interface BinaryCreateInput {
  channelId: string;
  expectedSize: number;
  mediaType?: string;
  lifetime?: BinaryLifetime;
  metadata?: Record<string, unknown>;
}

export interface BinaryCreateResult {
  id: string;
  kind: BinaryStorageKind;
  chunkSize: number;
  maxChunkSize: number;
  maxObjectSize: number;
}

export interface BinaryWriteResult {
  id: string;
  written: number;
  nextOffset: number;
}

export interface BinaryReadResult {
  id: string;
  offset: number;
  nextOffset: number;
  data: string;
  eof: boolean;
  size: number;
}

export interface BinaryStatResult {
  id: string;
  kind: BinaryStorageKind;
  size: number;
  mediaType?: string;
  lifetime: BinaryLifetime;
  checksum?: BinaryChecksum;
  metadata?: Record<string, unknown>;
  state: string;
}

function isBinaryObjectId(id: unknown): id is string {
  return typeof id === 'string' && id.length >= 5 && id.length <= 512 && /^bin_[0-9a-f-]+$/.test(id);
}

function requireBinaryReference(ref: BinaryReference): void {
  if (!ref || !isBinaryObjectId(ref.id)) {
    throw new Error('invalid binary reference id');
  }
  if (ref.kind !== 'file') {
    throw new Error(`unsupported binary storage kind '${String(ref.kind)}'`);
  }
  if (!Number.isSafeInteger(ref.size) || ref.size < 0) {
    throw new Error('invalid binary reference size');
  }
  if (ref.lifetime !== 'message' && ref.lifetime !== 'runtime') {
    throw new Error(`invalid binary lifetime '${String(ref.lifetime)}'`);
  }
}

function payloadOf<T>(envelope: Envelope, method: string): T {
  if (envelope.payload === undefined || envelope.payload === null) {
    throw new Error(`${method} returned an empty payload`);
  }
  return envelope.payload as T;
}

export async function binaryCreate(
  client: Client,
  input: BinaryCreateInput,
  opts: MessageOption[] = []
): Promise<BinaryCreateResult> {
  if (!input.channelId || input.channelId.trim() === '') {
    throw new Error('binary.create requires channelId');
  }
  const expectedSize = input.expectedSize;
  if (!Number.isSafeInteger(expectedSize) || expectedSize < 0) {
    throw new Error('binary.create expectedSize must be a non-negative safe integer');
  }
  const envelope = await client.sendReservedRequest(METHOD_BINARY_CREATE, {
    ...input,
    expectedSize,
    lifetime: input.lifetime ?? 'message',
  }, ...opts);
  return payloadOf<BinaryCreateResult>(envelope, METHOD_BINARY_CREATE);
}

export async function binaryWrite(
  client: Client,
  id: string,
  offset: number,
  data: Uint8Array,
  opts: MessageOption[] = []
): Promise<BinaryWriteResult> {
  if (!isBinaryObjectId(id)) throw new Error('binary.write requires a valid binary object id');
  if (!Number.isSafeInteger(offset) || offset < 0) throw new Error('binary.write offset must be a non-negative safe integer');
  if (!(data instanceof Uint8Array) || data.byteLength === 0 || data.byteLength > MAX_BINARY_CHUNK_SIZE) {
    throw new Error(`binary.write chunk must contain 1..${MAX_BINARY_CHUNK_SIZE} bytes`);
  }
  const envelope = client.supportsBinaryFrames()
    ? await client.sendReservedBinaryRequest(METHOD_BINARY_WRITE, id, offset, data, ...opts)
    : await client.sendReservedRequest(METHOD_BINARY_WRITE, {
        id,
        offset,
        data: Buffer.from(data).toString('base64'),
      }, ...opts);
  return payloadOf<BinaryWriteResult>(envelope, METHOD_BINARY_WRITE);
}

export async function binarySeal(
  client: Client,
  id: string,
  opts: MessageOption[] = []
): Promise<BinaryReference> {
  const envelope = await client.sendReservedRequest(METHOD_BINARY_SEAL, { id }, ...opts);
  const result = payloadOf<{ reference: BinaryReference }>(envelope, METHOD_BINARY_SEAL);
  requireBinaryReference(result.reference);
  return result.reference;
}

export async function binaryAbort(client: Client, id: string, opts: MessageOption[] = []): Promise<void> {
  await client.sendReservedRequest(METHOD_BINARY_ABORT, { id }, ...opts);
}

export async function binaryRelease(client: Client, id: string, opts: MessageOption[] = []): Promise<void> {
  await client.sendReservedRequest(METHOD_BINARY_RELEASE, { id }, ...opts);
}

export async function binaryStat(client: Client, id: string, opts: MessageOption[] = []): Promise<BinaryStatResult> {
  const envelope = await client.sendReservedRequest(METHOD_BINARY_STAT, { id }, ...opts);
  return payloadOf<BinaryStatResult>(envelope, METHOD_BINARY_STAT);
}

export async function binaryRead(
  client: Client,
  ref: BinaryReference,
  offset: number,
  maxBytes: number = DEFAULT_BINARY_CHUNK_SIZE,
  opts: MessageOption[] = []
): Promise<{ data: Uint8Array; result: BinaryReadResult }> {
  requireBinaryReference(ref);
  if (!Number.isSafeInteger(offset) || offset < 0) throw new Error('binary.read offset must be a non-negative safe integer');
  if (!Number.isSafeInteger(maxBytes) || maxBytes <= 0 || maxBytes > MAX_BINARY_CHUNK_SIZE) {
    throw new Error(`binary.read maxBytes must be in 1..${MAX_BINARY_CHUNK_SIZE}`);
  }
  const envelope = await client.sendReservedRequest(METHOD_BINARY_READ, {
    reference: ref,
    offset,
    maxBytes,
  }, ...opts);
  const result = payloadOf<BinaryReadResult>(envelope, METHOD_BINARY_READ);
  return { data: Uint8Array.from(Buffer.from(result.data, 'base64')), result };
}

export async function binaryUpload(
  client: Client,
  input: BinaryCreateInput,
  data: Uint8Array,
  opts: MessageOption[] = []
): Promise<BinaryReference> {
  const created = await binaryCreate(client, { ...input, expectedSize: data.byteLength }, opts);
  let sealed = false;
  try {
    let chunkSize = created.chunkSize;
    if (!Number.isSafeInteger(chunkSize) || chunkSize <= 0 || chunkSize > created.maxChunkSize) {
      chunkSize = Math.min(DEFAULT_BINARY_CHUNK_SIZE, created.maxChunkSize || MAX_BINARY_CHUNK_SIZE);
    }
    chunkSize = Math.min(chunkSize, MAX_BINARY_CHUNK_SIZE);
    let offset = 0;
    while (offset < data.byteLength) {
      const end = Math.min(offset + chunkSize, data.byteLength);
      const result = await binaryWrite(client, created.id, offset, data.subarray(offset, end), opts);
      if (result.nextOffset <= offset) throw new Error(`binary.write made no progress at offset ${offset}`);
      offset = result.nextOffset;
    }
    const ref = await binarySeal(client, created.id, opts);
    sealed = true;
    return ref;
  } finally {
    if (!sealed) {
      try { await binaryAbort(client, created.id, opts); } catch { /* best-effort cleanup */ }
    }
  }
}

export async function binaryReadAll(
  client: Client,
  ref: BinaryReference,
  opts: MessageOption[] = []
): Promise<Uint8Array> {
  requireBinaryReference(ref);
  const chunks: Uint8Array[] = [];
  let total = 0;
  let offset = 0;
  while (offset < ref.size) {
    const { data, result } = await binaryRead(client, ref, offset, DEFAULT_BINARY_CHUNK_SIZE, opts);
    chunks.push(data);
    total += data.byteLength;
    if (result.nextOffset <= offset && !result.eof) throw new Error(`binary.read made no progress at offset ${offset}`);
    offset = result.nextOffset;
    if (result.eof) break;
  }
  if (total !== ref.size) throw new Error(`binary.read size mismatch: got ${total}, want ${ref.size}`);
  const out = new Uint8Array(total);
  let cursor = 0;
  for (const chunk of chunks) {
    out.set(chunk, cursor);
    cursor += chunk.byteLength;
  }
  return out;
}

export async function channelPublishBinary(
  client: Client,
  channelId: string,
  ref: BinaryReference,
  metadata?: Record<string, unknown>,
  opts: MessageOption[] = []
): Promise<Envelope> {
  requireBinaryReference(ref);
  return channelPublish(client, { channelId, payload: ref, metadata }, opts);
}
