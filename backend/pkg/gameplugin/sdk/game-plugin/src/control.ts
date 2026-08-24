import { Client, MessageOption } from './client';

export const METHOD_AUTHORITY_SNAPSHOT = 'control.authority.snapshot';
export const METHOD_CONTROL_REGISTER_SINK = 'control.sink.register';
export const METHOD_CONTROL_OUTPUT = 'control.output';
export const METHOD_CONTROL_AUTHORITY_TAKEOVER = 'control.authority.takeover';
export const METHOD_CONTROL_AUTHORITY_RELEASE = 'control.authority.release';
export const METHOD_EMERGENCY_STOP = 'emergency.stop';
export const METHOD_CONTROL_SINK_DISPATCH = 'control.sink.dispatch';

export const CONTROL_MODE_OBSERVE_ONLY = 'observe_only';
export const CONTROL_MODE_ASSIST = 'assist';
export const CONTROL_MODE_SHARED_CONTROL = 'shared_control';
export const CONTROL_MODE_PLUGIN_CONTROL = 'plugin_control';
export const CONTROL_MODE_USER_CONTROL = 'user_control';
export const CONTROL_MODE_SUSPENDED = 'suspended';

// Backward-compatible symbol names using canonical GameHost v1 wire values.
export const CONTROL_MODE_OBSERVE = CONTROL_MODE_OBSERVE_ONLY;
export const CONTROL_MODE_SHARED = CONTROL_MODE_SHARED_CONTROL;
export const CONTROL_MODE_PLUGIN = CONTROL_MODE_PLUGIN_CONTROL;
export const CONTROL_MODE_USER = CONTROL_MODE_USER_CONTROL;

export const OUTPUT_KIND_CUSTOM_RPC = 'custom_rpc';
export const OUTPUT_KIND_CHANNEL = 'channel';
export const OUTPUT_KIND_BINARY = 'binary';
export const OUTPUT_KIND_EFFECT = 'effect';

export interface AuthoritySnapshot {
  runtimeId: string;
  pluginId: string;
  mode: string;
  epoch: number;
  updatedAt: string;
  lastTransitionReason?: string;
  lastTransitionActor?: string;
}

export interface ControlSinkRegisterInput {
  sinkId: string;
  kind: string;
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
  payload: unknown;
}

export interface ControlOutputResult {
  outputId: string;
  allowed: boolean;
  reason?: string;
  currentEpoch: number;
  generation: number;
}

export interface AuthorityTakeoverInput {
  expectedEpoch?: number;
}

export interface AuthorityTakeoverResult {
  previousMode: string;
  newMode: string;
  previousEpoch: number;
  newEpoch: number;
  snapshot: AuthoritySnapshot;
}

export interface AuthorityReleaseInput {
  targetMode?: string;
  expectedEpoch?: number;
}

export interface AuthorityReleaseResult {
  previousMode: string;
  newMode: string;
  previousEpoch: number;
  newEpoch: number;
  snapshot: AuthoritySnapshot;
}

export interface EmergencyStopInput {
  idempotencyKey?: string;
}

export interface EmergencyStopResult {
  operationId: string;
  runtimeId: string;
  state: string;
  actor: string;
  reason: string;
  success: boolean;
  criticalFailure: boolean;
  residue?: string[];
  startedAt: string;
  finishedAt: string;
}

export interface SinkEffectDispatchPayload {
  sinkId: string;
  serviceId: string;
  outputId: string;
  epoch: number;
  generation: number;
  payload: unknown;
}

export async function getAuthoritySnapshot(
  client: Client,
  opts: MessageOption[] = []
): Promise<AuthoritySnapshot> {
  const envelope = await client.sendReservedRequest(METHOD_AUTHORITY_SNAPSHOT, {}, ...opts);
  return envelope.payload as AuthoritySnapshot;
}

export async function registerControlSink(
  client: Client,
  input: ControlSinkRegisterInput,
  opts: MessageOption[] = []
): Promise<ControlSinkRegisterResult> {
  const envelope = await client.sendReservedRequest(METHOD_CONTROL_REGISTER_SINK, input, ...opts);
  return envelope.payload as ControlSinkRegisterResult;
}

export async function submitControlOutput(
  client: Client,
  input: ControlOutputInput,
  opts: MessageOption[] = []
): Promise<ControlOutputResult> {
  const envelope = await client.sendReservedRequest(METHOD_CONTROL_OUTPUT, input, ...opts);
  return envelope.payload as ControlOutputResult;
}

export async function takeoverAuthority(
  client: Client,
  input: AuthorityTakeoverInput = {},
  opts: MessageOption[] = []
): Promise<AuthorityTakeoverResult> {
  const envelope = await client.sendReservedRequest(METHOD_CONTROL_AUTHORITY_TAKEOVER, input, ...opts);
  return envelope.payload as AuthorityTakeoverResult;
}

export async function releaseAuthority(
  client: Client,
  input: AuthorityReleaseInput = {},
  opts: MessageOption[] = []
): Promise<AuthorityReleaseResult> {
  const envelope = await client.sendReservedRequest(METHOD_CONTROL_AUTHORITY_RELEASE, input, ...opts);
  return envelope.payload as AuthorityReleaseResult;
}

export async function emergencyStop(
  client: Client,
  input: EmergencyStopInput = {},
  opts: MessageOption[] = []
): Promise<EmergencyStopResult> {
  const envelope = await client.sendReservedRequest(METHOD_EMERGENCY_STOP, input, ...opts);
  return envelope.payload as EmergencyStopResult;
}

export interface SinkEffectCommitResult {
  accepted: boolean;
  committed: boolean;
  effectId: string;
  generation: number;
  errorCode?: string;
  message?: string;
}

export type SinkDispatchHandler = (payload: SinkEffectDispatchPayload) => Promise<SinkEffectCommitResult>;

export function registerSinkDispatchHandler(
  registry: import('./handler').HandlerRegistry,
  handler: SinkDispatchHandler
): void {
  registry.registerRequest(METHOD_CONTROL_SINK_DISPATCH, async (request) => {
    const payload = request.payload as SinkEffectDispatchPayload;
    const result = await handler(payload);
    return result;
  });
}
