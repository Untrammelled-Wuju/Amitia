import { Client, MessageOption } from './client';

export const METHOD_SECRET_ACQUIRE = 'secret.acquire';
export const METHOD_SECRET_RELEASE = 'secret.release';
export const METHOD_SECRET_QUERY = 'secret.query';

export const SECRET_PURPOSE_STARTUP = 'startup';
export const SECRET_PURPOSE_RUNTIME = 'runtime';

export type SecretRef = string;

export interface SecretAcquireInput {
  ref: SecretRef;
  purpose: 'startup' | 'runtime';
  required?: boolean;
}

export interface SecretAcquireResult {
  leaseId: string;
  ref: SecretRef;
  purpose: 'startup' | 'runtime';
  granted: boolean;
  expiresAt?: number;
  reason?: string;
}

export interface SecretReleaseInput {
  leaseId: string;
  reason?: string;
}

export interface SecretReleaseResult {
  released: boolean;
  reason?: string;
}

export interface SecretQueryInput {
  leaseId: string;
}

export interface SecretQueryResult {
  leaseId: string;
  ref?: SecretRef;
  granted: boolean;
  expiresAt?: number;
  valid: boolean;
}

export async function acquireSecret(
  client: Client,
  input: SecretAcquireInput,
  opts: MessageOption[] = []
): Promise<SecretAcquireResult> {
  const envelope = await client.sendReservedRequest(METHOD_SECRET_ACQUIRE, input, ...opts);
  return envelope.payload as SecretAcquireResult;
}

export async function releaseSecret(
  client: Client,
  input: SecretReleaseInput,
  opts: MessageOption[] = []
): Promise<SecretReleaseResult> {
  const envelope = await client.sendReservedRequest(METHOD_SECRET_RELEASE, input, ...opts);
  return envelope.payload as SecretReleaseResult;
}

export async function querySecretLease(
  client: Client,
  input: SecretQueryInput,
  opts: MessageOption[] = []
): Promise<SecretQueryResult> {
  const envelope = await client.sendReservedRequest(METHOD_SECRET_QUERY, input, ...opts);
  return envelope.payload as SecretQueryResult;
}
