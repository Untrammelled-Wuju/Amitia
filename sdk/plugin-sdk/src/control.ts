export interface RunnerClient {
  call(method: string, payload: unknown): Promise<unknown>;
  registerRequest(method: string, handler: (payload: unknown) => Promise<unknown> | unknown): void;
  unregisterRequest?(method: string): void;
}

export interface ControlSinkRegisterInput {
  readonly sinkId: string;
  readonly kind: string;
  readonly serviceId?: string;
  readonly description?: string;
}

export interface ControlSinkRegisterResult {
  readonly sinkId: string;
  readonly kind: string;
  readonly registered: boolean;
}

export interface ControlOutputResult {
  readonly outputId: string;
  readonly allowed: boolean;
  readonly reason?: string;
  readonly generation: number;
  readonly currentEpoch: number;
}

export interface AuthoritySnapshot {
  readonly mode: string;
  readonly epoch: number;
  readonly valid: boolean;
  readonly emergency: boolean;
}

export interface AuthorityTakeoverInput {
  readonly targetMode: string;
  readonly actor: string;
}

export interface AuthorityTakeoverResult {
  readonly targetMode: string;
  readonly actor: string;
  readonly previousMode: string;
  readonly currentMode: string;
  readonly currentEpoch: number;
}

export interface EmergencyStopStatus {
  readonly operationId: string;
  readonly state: string;
  readonly active: boolean;
}

export class ControlEffectError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "ControlEffectError";
  }
}

export function registerControlSink(
  client: RunnerClient,
  input: ControlSinkRegisterInput,
): ControlSinkRegisterResult {
  void client;
  return {
    sinkId: input.sinkId,
    kind: input.kind,
    registered: true,
  };
}

export async function submitControlOutput(
  client: RunnerClient,
  sinkId: string,
  outputId: string,
  payload: unknown,
  epoch?: number,
): Promise<ControlOutputResult> {
  const result = await client.call("mockgame.control.output", {
    sinkId,
    outputId,
    data: payload,
    epoch,
  });
  const r = (result ?? {}) as { outputId?: string; allowed?: boolean; reason?: string; generation?: number; currentEpoch?: number };
  return {
    outputId: r.outputId ?? outputId,
    allowed: r.allowed ?? false,
    reason: r.reason,
    generation: r.generation ?? 0,
    currentEpoch: r.currentEpoch ?? 0,
  };
}

export function registerSinkDispatchHandler(
  client: RunnerClient,
  sinkId: string,
  handler: (payload: unknown) => Promise<unknown> | unknown,
): () => void {
  const method = `sink.dispatch.${sinkId}`;
  client.registerRequest(method, handler);
  return () => {
    client.unregisterRequest?.(method);
  };
}

export async function takeoverAuthority(
  client: RunnerClient,
  input: AuthorityTakeoverInput,
): Promise<AuthorityTakeoverResult> {
  const result = await client.call("authority.takeover", {
    targetMode: input.targetMode,
    actor: input.actor,
  });
  const r = (result ?? {}) as AuthorityTakeoverResult;
  return {
    targetMode: r.targetMode ?? input.targetMode,
    actor: r.actor ?? input.actor,
    previousMode: r.previousMode ?? "observe",
    currentMode: r.currentMode ?? input.targetMode,
    currentEpoch: r.currentEpoch ?? 0,
  };
}

export async function releaseAuthority(
  client: RunnerClient,
  input: AuthorityTakeoverInput,
  _reason?: string,
): Promise<AuthorityTakeoverResult> {
  const result = await client.call("authority.release", {
    targetMode: input.targetMode,
    actor: input.actor,
  });
  const r = (result ?? {}) as AuthorityTakeoverResult;
  return {
    targetMode: r.targetMode ?? input.targetMode,
    actor: r.actor ?? input.actor,
    previousMode: r.previousMode ?? "plugin",
    currentMode: r.currentMode ?? input.targetMode,
    currentEpoch: r.currentEpoch ?? 0,
  };
}

export async function getAuthoritySnapshot(
  client: RunnerClient,
  runtimeId?: string,
  serviceId?: string,
): Promise<AuthoritySnapshot> {
  const result = await client.call("authority.snapshot", {
    runtimeId,
    serviceId,
  });
  const r = (result ?? {}) as AuthoritySnapshot;
  return {
    mode: r.mode ?? "observe",
    epoch: r.epoch ?? 0,
    valid: r.valid ?? true,
    emergency: r.emergency ?? false,
  };
}

export async function getEmergencyStopStatus(
  client: RunnerClient,
  runtimeId?: string,
): Promise<EmergencyStopStatus> {
  const result = await client.call("emergency.status", {
    runtimeId,
  });
  const r = (result ?? {}) as EmergencyStopStatus;
  return {
    operationId: r.operationId ?? "unknown",
    state: r.state ?? "inactive",
    active: r.active ?? false,
  };
}
