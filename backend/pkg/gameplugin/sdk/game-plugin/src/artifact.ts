import { Client, MessageOption } from './client';
import type { PluginArtifact } from './game';

export const METHOD_ARTIFACT_LIST = 'artifact.list';
export const METHOD_ARTIFACT_DEPLOY_REQUIRED = 'artifact.deploy_required';
export const METHOD_ARTIFACT_DEPLOY = 'artifact.deploy';
export const METHOD_ARTIFACT_VERIFY = 'artifact.verify';
export const METHOD_ARTIFACT_REMOVE = 'artifact.remove';

export interface ArtifactRequest {
  artifactId?: string;
  targetRoot: string;
  compatibilityVersion?: string;
}

export interface ArtifactStatus {
  artifact: PluginArtifact;
  installed: boolean;
  healthy: boolean;
  targetPath?: string;
  installedHash?: string;
}

export interface ArtifactListResult {
  items: ArtifactStatus[];
}

export interface ArtifactRemoveResult {
  removed: boolean;
}

export async function listArtifacts(client: Client, input: ArtifactRequest, opts: MessageOption[] = []): Promise<ArtifactListResult> {
  const envelope = await client.sendReservedRequest(METHOD_ARTIFACT_LIST, input, ...opts);
  return envelope.payload as ArtifactListResult;
}

export async function deployRequiredArtifacts(client: Client, input: ArtifactRequest, opts: MessageOption[] = []): Promise<ArtifactListResult> {
  const envelope = await client.sendReservedRequest(METHOD_ARTIFACT_DEPLOY_REQUIRED, input, ...opts);
  return envelope.payload as ArtifactListResult;
}

export async function deployArtifact(client: Client, input: ArtifactRequest, opts: MessageOption[] = []): Promise<ArtifactStatus> {
  const envelope = await client.sendReservedRequest(METHOD_ARTIFACT_DEPLOY, input, ...opts);
  return envelope.payload as ArtifactStatus;
}

export async function verifyArtifact(client: Client, input: ArtifactRequest, opts: MessageOption[] = []): Promise<ArtifactStatus> {
  const envelope = await client.sendReservedRequest(METHOD_ARTIFACT_VERIFY, input, ...opts);
  return envelope.payload as ArtifactStatus;
}

export async function removeArtifact(client: Client, input: ArtifactRequest, opts: MessageOption[] = []): Promise<ArtifactRemoveResult> {
  const envelope = await client.sendReservedRequest(METHOD_ARTIFACT_REMOVE, input, ...opts);
  return envelope.payload as ArtifactRemoveResult;
}
