import type { HostFeature } from './protocol';

export type PluginSessionStatus = 'created' | 'connecting' | 'ready' | 'paused' | 'closed' | 'failed';
export type PluginOperationStatus = 'succeeded' | 'failed' | 'cancelled' | 'rejected';

export interface PluginSessionOpenRequest {
  context?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
  payload?: unknown;
}

export interface PluginSession {
  id: string;
  status: PluginSessionStatus;
  startedAt?: string;
  updatedAt?: string;
  metadata?: Record<string, unknown>;
  payload?: unknown;
}

export interface PluginEvent {
  id: string;
  sessionId?: string;
  type: string;
  occurredAt: string;
  metadata?: Record<string, unknown>;
  payload?: unknown;
}

export interface PluginOperation {
  id: string;
  sessionId?: string;
  type: string;
  parameters?: unknown;
  idempotencyKey?: string;
  deadlineMs?: number;
}

export interface PluginOperationResult {
  operationId: string;
  sessionId?: string;
  status: PluginOperationStatus;
  output?: unknown;
  errorCode?: string;
  errorMessage?: string;
  retryable?: boolean;
}

export interface PluginArtifact {
  id: string;
  type: 'file' | 'directory' | 'zip';
  platforms?: string[];
  architectures?: string[];
  compatibilityVersions?: string[];
  source: string;
  target: string;
  required?: boolean;
  sha256?: string;
}

export interface PluginNetworkPolicy {
  mode?: 'none' | 'loopback' | 'unrestricted';
}

export interface PluginServiceSpec {
  id: string;
  moduleId: string;
  name?: string;
  kind?: 'process';
  required?: boolean;
  dependsOn?: string[];
  metadata?: Record<string, string>;
}

export interface PluginChannelSpec {
  id: string;
  serviceId?: string;
  kind: 'event' | 'state' | 'log' | 'metric' | 'custom';
  schemaId?: string;
  metadata?: Record<string, string>;
}

export interface PluginControlEffectSinkSpec {
  id: string;
  serviceId: string;
  description?: string;
}

export interface PluginHostSpec {
  protocolVersion: string;
  runtimeModuleId?: string;
  hostFeatures?: HostFeature[];
  services?: PluginServiceSpec[];
  channels?: PluginChannelSpec[];
  controlEffectSinks?: PluginControlEffectSinkSpec[];
  artifacts?: PluginArtifact[];
  network?: PluginNetworkPolicy;
  metadata?: Record<string, unknown>;
}

