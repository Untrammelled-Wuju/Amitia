import type { ChannelDirection, FrequencyHint, HostFeature } from './protocol';

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
  mode: 'none' | 'loopback' | 'restricted' | 'unrestricted';
  /** Exact hosts or wildcard subdomains such as *.example.com. Restricted mode only. */
  allowedDomains?: string[];
  /** Exact IP literals (IPv4 or IPv6). Restricted mode only. */
  allowedIPs?: string[];
  /** Explicit destination ports. Restricted mode only. */
  allowedPorts?: number[];
  /** Host-mediated transports. Omitted in restricted mode defaults to HTTP/HTTPS for compatibility. */
  allowedTransports?: ('http' | 'https' | 'tcp' | 'udp' | 'websocket')[];
  /** Allow the portable `host-loopback` destination without sharing the host network namespace. */
  allowHostLoopback?: boolean;
  /** Maximum simultaneously open host-mediated socket handles for this service (1..64, default 16). */
  maxConnections?: number;
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
  kind: 'event' | 'state' | 'log' | 'metric' | 'custom' | 'binary';
  schemaId?: string;
  /** Channel flow direction. Omitted defaults to plugin_to_host for backward compatibility. */
  direction?: ChannelDirection;
  /** Informational expected message-frequency hint; the host does not treat this as rate limiting. */
  frequencyHint?: FrequencyHint;
  metadata?: Record<string, string>;
}

export interface PluginControlEffectSinkSpec {
  id: string;
  serviceId: string;
  description?: string;
}

interface PluginHostSpecBase {
  protocolVersion: 'amitia-game-host/1';
  hostFeatures?: HostFeature[];
  channels?: PluginChannelSpec[];
  controlEffectSinks?: PluginControlEffectSinkSpec[];
  artifacts?: PluginArtifact[];
  network: PluginNetworkPolicy;
  metadata?: Record<string, unknown>;
}

export type PluginHostSpec = PluginHostSpecBase & (
  | { runtimeModuleId: string; services?: PluginServiceSpec[] }
  | { runtimeModuleId?: string; services: [PluginServiceSpec, ...PluginServiceSpec[]] }
);

