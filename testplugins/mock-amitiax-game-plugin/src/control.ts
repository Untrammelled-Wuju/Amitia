import type { Envelope } from '@amitia/game-plugin-sdk';
import {
  AuthoritySnapshot,
  ControlOutputResult,
  ControlSinkRegisterResult,
  getAuthoritySnapshot,
  registerControlSink,
  submitControlOutput,
  registerSinkDispatchHandler,
  takeoverAuthority,
  releaseAuthority,
  emergencyStop,
  ControlSinkRegisterInput,
  SinkEffectDispatchPayload,
  SinkEffectCommitResult,
  Client,
  ControlOutputInput,
  AuthorityTakeoverInput,
  AuthorityReleaseInput,
} from '@amitia/game-plugin-sdk';

export class ControlService {
  private authorityMode: string = 'observe_only';
  private authorityEpoch: number = 0;
  private sinkRegistered: boolean = false;
  private emergencyActive: boolean = false;
  private effectCommitCount: number = 0;
  private dispatchHandlerRegistered: boolean = false;
  private processedOutputIds: Set<string> = new Set();
  private effectStates: Map<string, { outputId: string; committedAt: number; epoch: number; data: unknown }> = new Map();

  getAuthorityMode(): string {
    return this.authorityMode;
  }

  getAuthorityEpoch(): number {
    return this.authorityEpoch;
  }

  isEmergencyActive(): boolean {
    return this.emergencyActive;
  }

  isSinkRegistered(): boolean {
    return this.sinkRegistered;
  }

  isDispatchHandlerRegistered(): boolean {
    return this.dispatchHandlerRegistered;
  }

  isControlAllowed(): boolean {
    return !this.emergencyActive &&
      (this.authorityMode === 'shared_control' || this.authorityMode === 'plugin_control');
  }

  getEffectCommitCount(): number {
    return this.effectCommitCount;
  }

  updateAuthority(mode: string, epoch: number): void {
    this.authorityMode = mode;
    this.authorityEpoch = epoch;
  }

  setEmergencyActive(active: boolean): void {
    this.emergencyActive = active;
    if (active) {
      this.authorityMode = 'suspended';
    }
  }

  setSinkRegistered(registered: boolean): void {
    this.sinkRegistered = registered;
  }

  setDispatchHandlerRegistered(registered: boolean): void {
    this.dispatchHandlerRegistered = registered;
  }

  recordEffectCommit(): void {
    this.effectCommitCount++;
  }

  checkPreconditions(): { allowed: boolean; reason: string } {
    if (this.emergencyActive) {
      return { allowed: false, reason: 'emergency_stop_active' };
    }
    if (!this.isControlAllowed()) {
      return { allowed: false, reason: 'authority_mode_denied' };
    }
    return { allowed: true, reason: 'preconditions_met' };
  }

  isOutputProcessed(outputId: string): boolean {
    return this.processedOutputIds.has(outputId);
  }

  recordOutputProcessed(outputId: string, epoch: number, data: unknown): void {
    this.processedOutputIds.add(outputId);
    this.effectStates.set(outputId, { outputId, committedAt: Date.now(), epoch, data });
  }

  getEffectState(outputId: string): { outputId: string; committedAt: number; epoch: number; data: unknown } | undefined {
    return this.effectStates.get(outputId);
  }

  getAllEffectStates(): Array<{ outputId: string; committedAt: number; epoch: number; data: unknown }> {
    return Array.from(this.effectStates.values());
  }

  getProcessedOutputCount(): number {
    return this.processedOutputIds.size;
  }
}

export class ControlHandler {
  private controlService: ControlService;

  constructor(controlService: ControlService) {
    this.controlService = controlService;
  }

  async handleAuthoritySnapshot(_request: Envelope): Promise<AuthoritySnapshot> {
    return {
      mode: this.controlService.getAuthorityMode(),
      epoch: this.controlService.getAuthorityEpoch(),
      runtimeId: '',
      pluginId: '',
      updatedAt: new Date().toISOString(),
    };
  }

  async handleSinkRegister(request: Envelope): Promise<ControlSinkRegisterResult> {
    const payload = (request.payload as { sinkId?: string; kind?: string }) || {};
    const sinkId = payload.sinkId || 'worldgame.effect';
    const kind = payload.kind || 'effect';
    this.controlService.setSinkRegistered(true);
    return { sinkId, registered: true };
  }

  async handleOutput(client: Client, request: Envelope): Promise<ControlOutputResult> {
    const payload = (request.payload as { outputId?: string; sinkId?: string; epoch?: number; data?: unknown }) || {};
    const outputId = payload.outputId || 'unknown';
    const preconditions = this.controlService.checkPreconditions();
    if (!preconditions.allowed) {
      return { outputId, allowed: false, reason: preconditions.reason, currentEpoch: this.controlService.getAuthorityEpoch(), generation: 0 };
    }
    if (payload.epoch !== undefined && payload.epoch !== this.controlService.getAuthorityEpoch()) {
      return { outputId, allowed: false, reason: 'stale_epoch', currentEpoch: this.controlService.getAuthorityEpoch(), generation: 0 };
    }
    if (this.controlService.isOutputProcessed(outputId)) {
      return { outputId, allowed: true, currentEpoch: this.controlService.getAuthorityEpoch(), generation: 0 };
    }
    const input: ControlOutputInput = {
      sinkId: payload.sinkId || 'worldgame.effect',
      outputId,
      payload: payload.data || {},
      epoch: payload.epoch ?? this.controlService.getAuthorityEpoch(),
    };
    const result = await submitControlOutput(client, input);
    if (!result.allowed) {
      return { outputId, allowed: false, reason: result.reason ?? 'host_rejected', currentEpoch: result.currentEpoch, generation: result.generation };
    }
    this.controlService.recordOutputProcessed(outputId, result.currentEpoch, payload.data);
    this.controlService.recordEffectCommit();
    return { outputId, allowed: true, currentEpoch: result.currentEpoch, generation: result.generation };
  }

  async handleDispatch(payload: SinkEffectDispatchPayload): Promise<SinkEffectCommitResult> {
    const preconditions = this.controlService.checkPreconditions();
    if (!preconditions.allowed) {
      return {
        accepted: false,
        committed: false,
        effectId: payload.outputId,
        generation: payload.generation,
        errorCode: preconditions.reason,
        message: 'dispatch rejected',
      };
    }
    if (this.controlService.isOutputProcessed(payload.outputId)) {
      return {
        accepted: true,
        committed: true,
        effectId: payload.outputId,
        generation: payload.generation,
      };
    }
    this.controlService.recordOutputProcessed(payload.outputId, this.controlService.getAuthorityEpoch(), payload);
    this.controlService.recordEffectCommit();
    return {
      accepted: true,
      committed: true,
      effectId: payload.outputId,
      generation: payload.generation,
    };
  }

  async handleTakeover(request: Envelope): Promise<unknown> {
    const payload = (request.payload as { targetMode?: string; actor?: string }) || {};
    return {
      targetMode: payload.targetMode || 'user_control',
      previousMode: this.controlService.getAuthorityMode(),
      currentMode: this.controlService.getAuthorityMode(),
      currentEpoch: this.controlService.getAuthorityEpoch(),
    };
  }

  async handleRelease(request: Envelope): Promise<unknown> {
    const payload = (request.payload as { targetMode?: string; actor?: string }) || {};
    return {
      targetMode: payload.targetMode || 'observe_only',
      previousMode: this.controlService.getAuthorityMode(),
      currentMode: this.controlService.getAuthorityMode(),
      currentEpoch: this.controlService.getAuthorityEpoch(),
    };
  }

  async handleTakeoverFull(client: import('@amitia/game-plugin-sdk').Client, request: Envelope): Promise<unknown> {
    const payload = (request.payload as { expectedEpoch?: number }) || {};
    const input: AuthorityTakeoverInput = { expectedEpoch: payload.expectedEpoch };
    return takeoverAuthority(client, input);
  }

  async handleReleaseFull(client: import('@amitia/game-plugin-sdk').Client, request: Envelope): Promise<unknown> {
    const payload = (request.payload as { targetMode?: string; expectedEpoch?: number }) || {};
    const input: AuthorityReleaseInput = {
      targetMode: payload.targetMode || 'observe_only',
      expectedEpoch: payload.expectedEpoch,
    };
    return releaseAuthority(client, input);
  }

  async handleEmergencyStatus(): Promise<unknown> {
    return {
      operationId: 'mock-emergency-check',
      state: this.controlService.isEmergencyActive() ? 'active' : 'inactive',
      active: this.controlService.isEmergencyActive(),
    };
  }

  async handleEffectStatus(request: Envelope): Promise<unknown> {
    const payload = (request.payload as { outputId?: string }) || {};
    const outputId = payload.outputId;
    if (outputId) {
      const state = this.controlService.getEffectState(outputId);
      return {
        outputId,
        found: state !== undefined,
        state: state || null,
        processed: this.controlService.isOutputProcessed(outputId),
      };
    }
    const allStates = this.controlService.getAllEffectStates();
    return {
      effectCount: this.controlService.getEffectCommitCount(),
      processedOutputs: this.controlService.getProcessedOutputCount(),
      effects: allStates,
    };
  }
}

export {
  AuthoritySnapshot,
  ControlOutputResult,
  ControlSinkRegisterResult,
  ControlSinkRegisterInput,
  getAuthoritySnapshot,
  registerControlSink,
  submitControlOutput,
  registerSinkDispatchHandler,
  takeoverAuthority,
  releaseAuthority,
  emergencyStop,
};
