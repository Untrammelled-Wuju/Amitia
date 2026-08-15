import { Client, MessageOption } from './client';

export const METHOD_AUTHORITY_SNAPSHOT = 'control.authority.snapshot';
export const METHOD_CONTROL_REGISTER_SINK = 'control.sink.register';
export const METHOD_CONTROL_OUTPUT = 'control.output';
export const METHOD_CONTROL_AUTHORITY_TAKEOVER = 'control.authority.takeover';
export const METHOD_CONTROL_AUTHORITY_RELEASE = 'control.authority.release';
export const METHOD_EMERGENCY_STOP_INITIATE = 'emergency.stop.initiate';
export const METHOD_EMERGENCY_STOP_STATUS = 'emergency.stop.status';
export const METHOD_CONTROL_SINK_DISPATCH = 'control.sink.dispatch';

export const CONTROL_MODE_OBSERVE = 'observe';
export const CONTROL_MODE_ASSIST = 'assist';
export const CONTROL_MODE_SHARED = 'shared';
export const CONTROL_MODE_PLUGIN = 'plugin';
export const CONTROL_MODE_USER = 'user';
export const CONTROL_MODE_SUSPENDED = 'suspended';

export const OUTPUT_KIND_CUSTOM_RPC = 'custom_rpc';
export const OUTPUT_KIND_CHANNEL = 'channel';
export const OUTPUT_KIND_BINARY = 'binary';
export const OUTPUT_KIND_EFFECT = 'effect';

export interface AuthoritySnapshot {
  runtimeId: string;
  pluginId: string;
  mode: string;
  epoch: number;
  serviceId?: string;
  updatedAt?: number;
  valid: boolean;
}

export interface ControlSinkRegisterInput {
  sinkId: string;
  kind: string;
  serviceId?: string;
  metadata?: Record<string, unknown>;
}

export interface ControlSinkRegisterResult {
  sinkId: string;
  registered: boolean;
}

export interface ControlOutputInput {
  outputId: string;
  sinkId: string;
  epoch: number;
  kind: string;
  serviceId?: string;
  payload: unknown;
}

export interface ControlOutputResult {
  outputId: string;
  allowed: boolean;
  reason?: string;
  currentEpoch?: number;
}

export interface AuthorityTakeoverInput {
  targetMode: string;
  actor: string;
  expectedEpoch?: number;
  serviceId?: string;
}

export interface AuthorityTakeoverResult {
  previousMode: string;
  newMode: string;
  previousEpoch: number;
  newEpoch: number;
  success: boolean;
  reason?: string;
}

export interface AuthorityReleaseInput {
  targetMode: string;
  actor: string;
  expectedEpoch?: number;
  serviceId?: string;
}

export interface AuthorityReleaseResult {
  previousMode: string;
  newMode: string;
  previousEpoch: number;
  newEpoch: number;
  success: boolean;
  reason?: string;
}

export interface EmergencyStopStatusInput {
  operationId?: string;
}

export interface EmergencyStopStatusResult {
  operationId: string;
  state: string;
  active: boolean;
  reason?: string;
  initiatedAt?: number;
  completedAt?: number;
}

export interface SinkEffectDispatchPayload {
  sinkId: string;
  serviceId: string;
  payload: unknown;
}

export async function getAuthoritySnapshot(
  client: Client,
  runtimeId: string,
  serviceId?: string,
  opts: MessageOption[] = []
): Promise<AuthoritySnapshot> {
  const input: Record<string, unknown> = { runtimeId };
  if (serviceId) {
    input.serviceId = serviceId;
  }
  const envelope = await client.sendReservedRequest(METHOD_AUTHORITY_SNAPSHOT, input, undefined, ...opts);
  return envelope.payload as AuthoritySnapshot;
}

export async function registerControlSink(
  client: Client,
  input: ControlSinkRegisterInput,
  opts: MessageOption[] = []
): Promise<ControlSinkRegisterResult> {
  const envelope = await client.sendReservedRequest(METHOD_CONTROL_REGISTER_SINK, input, undefined, ...opts);
  return envelope.payload as ControlSinkRegisterResult;
}

export async function submitControlOutput(
  client: Client,
  input: ControlOutputInput,
  opts: MessageOption[] = []
): Promise<ControlOutputResult> {
  const envelope = await client.sendReservedRequest(METHOD_CONTROL_OUTPUT, input, undefined, ...opts);
  return envelope.payload as ControlOutputResult;
}

export async function takeoverAuthority(
  client: Client,
  input: AuthorityTakeoverInput,
  _runtimeId: string,
  opts: MessageOption[] = []
): Promise<AuthorityTakeoverResult> {
  const envelope = await client.sendReservedRequest(METHOD_CONTROL_AUTHORITY_TAKEOVER, input, undefined, ...opts);
  return envelope.payload as AuthorityTakeoverResult;
}

export async function releaseAuthority(
  client: Client,
  input: AuthorityReleaseInput,
  _runtimeId: string,
  opts: MessageOption[] = []
): Promise<AuthorityReleaseResult> {
  const envelope = await client.sendReservedRequest(METHOD_CONTROL_AUTHORITY_RELEASE, input, undefined, ...opts);
  return envelope.payload as AuthorityReleaseResult;
}

export async function getEmergencyStopStatus(
  client: Client,
  input: EmergencyStopStatusInput = {},
  opts: MessageOption[] = []
): Promise<EmergencyStopStatusResult> {
  const envelope = await client.sendReservedRequest(METHOD_EMERGENCY_STOP_STATUS, input, undefined, ...opts);
  return envelope.payload as EmergencyStopStatusResult;
}

export type SinkDispatchHandler = (payload: SinkEffectDispatchPayload) => Promise<void>;

export function registerSinkDispatchHandler(
  registry: import('./handler').HandlerRegistry,
  handler: SinkDispatchHandler
): void {
  registry.registerNotification(METHOD_CONTROL_SINK_DISPATCH, async (notification) => {
    const payload = notification.payload as SinkEffectDispatchPayload;
    await handler(payload);
  });
}
