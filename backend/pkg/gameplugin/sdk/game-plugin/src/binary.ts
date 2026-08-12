import { Client, MessageOption } from './client';

export const METHOD_BINARY_REGISTER = 'binary.register';
export const METHOD_BINARY_RELEASE = 'binary.release';

export interface BinaryChecksum {
  algorithm: string;
  value: string;
}

export interface BinaryRegisterInput {
  binaryId: string;
  kind: string;
  size: number;
  mediaType?: string;
  checksum?: BinaryChecksum;
  lifetime: string;
  metadata?: Record<string, unknown>;
}

export interface BinaryRegisterOutput {
  binaryId: string;
  token: string;
}

export interface BinaryReleaseInput {
  binaryId: string;
}

export async function registerBinary(
  client: Client,
  input: BinaryRegisterInput,
  opts: MessageOption[] = []
): Promise<BinaryRegisterOutput> {
  const payload: Record<string, unknown> = {
    binaryId: input.binaryId,
    kind: input.kind,
    size: input.size,
    lifetime: input.lifetime,
  };
  if (input.mediaType) {
    payload.mediaType = input.mediaType;
  }
  if (input.checksum) {
    payload.checksum = input.checksum;
  }
  if (input.metadata) {
    payload.metadata = input.metadata;
  }
  const envelope = await client.sendRequest(METHOD_BINARY_REGISTER, payload, ...opts);
  return envelope.payload as BinaryRegisterOutput;
}

export async function releaseBinary(
  client: Client,
  input: BinaryReleaseInput,
  opts: MessageOption[] = []
): Promise<void> {
  await client.sendRequest(METHOD_BINARY_RELEASE, input, ...opts);
}
