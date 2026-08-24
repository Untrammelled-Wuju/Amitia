import { Client, MessageOption } from './client';

export const METHOD_PERMISSION_CHECK = 'permission.check';
export const METHOD_PERMISSION_SNAPSHOT = 'permission.snapshot';
export const METHOD_PERMISSION_REQUEST = 'permission.request';

export const PERM_GAMEHOST_CONTROL = 'gamehost.control';
export const PERM_GAMEHOST_CHANNEL_USE = 'gamehost.channel.use';
export const PERM_GAMEHOST_HOST_API_INVOKE = 'gamehost.host_api.invoke';
export const PERM_GAMEHOST_ARTIFACT_DEPLOY = 'gamehost.artifact.deploy';

export const DECISION_ALLOWED = 'allowed';
export const DECISION_DENIED = 'denied';
export const DECISION_APPROVAL_REQUIRED = 'approval_required';

export interface PermissionCheckInput {
  permissionId: string;
  serviceId?: string;
  runtimeId?: string;
}

export interface PermissionCheckResult {
  permissionId: string;
  decision: 'allowed' | 'denied' | 'approval_required';
  reason?: string;
  detail?: string;
}

export interface PermissionSnapshotInput {
  runtimeId?: string;
  serviceId?: string;
}

export interface PermissionSnapshotResult {
  snapshotId: string;
  revision: string;
  grantedPerms: string[];
  grantedScopes: string[];
  expiresAt?: number;
  isValid: boolean;
}

export interface PermissionRequestInput {
  permissionId: string;
  serviceId?: string;
  reason?: string;
}

export interface PermissionRequestResult {
  permissionId: string;
  decision: 'allowed' | 'denied' | 'approval_required';
  reason?: string;
}

export async function checkPermission(
  client: Client,
  input: PermissionCheckInput,
  opts: MessageOption[] = []
): Promise<PermissionCheckResult> {
  const envelope = await client.sendRequest(METHOD_PERMISSION_CHECK, input, ...opts);
  return envelope.payload as PermissionCheckResult;
}

export async function getPermissionSnapshot(
  client: Client,
  input: PermissionSnapshotInput,
  opts: MessageOption[] = []
): Promise<PermissionSnapshotResult> {
  const envelope = await client.sendRequest(METHOD_PERMISSION_SNAPSHOT, input, ...opts);
  return envelope.payload as PermissionSnapshotResult;
}

export async function requestPermission(
  client: Client,
  input: PermissionRequestInput,
  opts: MessageOption[] = []
): Promise<PermissionRequestResult> {
  const envelope = await client.sendRequest(METHOD_PERMISSION_REQUEST, input, ...opts);
  return envelope.payload as PermissionRequestResult;
}
