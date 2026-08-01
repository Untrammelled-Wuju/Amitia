import type { PlaybackEvent } from "../animation/contracts";
import type { DesktopRuntimeHandlerV2 } from "./runtime-handler-v2";
import type { PlaybackEventPayload, StateSnapshotPayload } from "./protocol-v2";

export interface PlaybackEventBridgeHooks {
  shouldReportEvent: (event: PlaybackEvent) => boolean;
}

export class PlaybackEventBridge {
  private readonly runtime: DesktopRuntimeHandlerV2;
  private readonly hooks: PlaybackEventBridgeHooks;
  private lastPlaybackId = "";
  private lastCommandId = "";
  private lastActionKey = "";
  private playbackStartedAt = 0;

  constructor(runtime: DesktopRuntimeHandlerV2, hooks: PlaybackEventBridgeHooks = {
    shouldReportEvent: () => true,
  }) {
    this.runtime = runtime;
    this.hooks = hooks;
  }

  handlePlaybackEvent(event: PlaybackEvent): void {
    if (!this.hooks.shouldReportEvent(event)) {
      return;
    }

    if (event.commandId) {
      this.lastCommandId = event.commandId;
    }

    switch (event.type) {
      case "playback.action_started":
        this.handleActionStarted(event);
        break;
      case "playback.action_completed":
        this.handleActionCompleted(event);
        break;
      case "playback.action_interrupted":
        this.handleActionInterrupted(event);
        break;
      case "playback.action_failed":
        this.handleActionFailed(event);
        break;
      default:
        break;
    }
  }

  private handleActionStarted(event: PlaybackEvent): void {
    if (event.actionKey) {
      this.lastActionKey = event.actionKey;
    }
    if (event.playbackInstanceId) {
      this.lastPlaybackId = event.playbackInstanceId;
    }
    this.playbackStartedAt = Date.now();

    if (!this.runtime.isConnected()) {
      return;
    }

    void this.runtime.sendPlaybackStarted(this.lastPlaybackId, this.lastCommandId);
  }

  private handleActionCompleted(event: PlaybackEvent): void {
    if (!this.runtime.isConnected()) {
      return;
    }

    const playedMs = event.playedDurationMs ?? (Date.now() - this.playbackStartedAt);

    void this.runtime.sendPlaybackEnded(
      event.playbackInstanceId ?? this.lastPlaybackId,
      event.commandId ?? this.lastCommandId,
      event.actionKey ?? this.lastActionKey,
      playedMs,
      event.reason ?? "natural_end",
    );
  }

  private handleActionInterrupted(event: PlaybackEvent): void {
    if (!this.runtime.isConnected()) {
      return;
    }

    const playedMs = event.playedDurationMs ?? (Date.now() - this.playbackStartedAt);
    void this.runtime.sendPlaybackEnded(
      event.playbackInstanceId ?? this.lastPlaybackId,
      event.commandId ?? this.lastCommandId,
      event.actionKey ?? this.lastActionKey,
      playedMs,
      event.reason ?? "interrupted",
    );
  }

  private handleActionFailed(event: PlaybackEvent): void {
    if (!this.runtime.isConnected()) {
      return;
    }

    void this.runtime.sendPlaybackFailed(
      event.playbackInstanceId ?? this.lastPlaybackId,
      event.commandId ?? this.lastCommandId,
      event.actionKey ?? this.lastActionKey,
      event.error?.code ?? "unknown",
      event.error?.message ?? "unknown error",
    );
  }

  async reportStateSnapshot(snapshot: {
    packageId: string;
    packageRevision: number;
    defaultActionKey: string;
    currentActionKey?: string;
    phase: string;
    frameIndex?: number | null;
    cycleIndex?: number | null;
    playbackInstanceId?: string;
    currentCommandId?: string;
  }): Promise<void> {
    if (!this.runtime.isConnected()) {
      return;
    }

    const seq = this.runtime.getEventSequence();
    const gen = this.runtime.getConnectionGeneration();

    const payload: StateSnapshotPayload = {
      connectionGeneration: gen,
      eventSequence: seq,
      actualStateHash: `${snapshot.packageId}:${snapshot.packageRevision}:${snapshot.currentActionKey ?? ""}`,
      instanceStatus: snapshot.phase === "playing" ? "ready" : snapshot.phase,
      windowStatus: "visible",
      rendererStatus: snapshot.phase === "failed" ? "crashed" : "runtime_ready",
      playbackStatus: snapshot.phase === "playing" ? "playing" : (
        snapshot.phase === "paused" ? "paused" : "idle"
      ),
      appliedDesiredRevision: snapshot.packageRevision,
      appliedSettingsRevision: 0,
      installationId: snapshot.packageId,
      petId: snapshot.packageId,
      releaseId: snapshot.packageId,
      stableActionKey: snapshot.defaultActionKey,
      currentActionKey: snapshot.currentActionKey ?? snapshot.defaultActionKey,
      playbackInstanceId: snapshot.playbackInstanceId,
      currentCommandId: snapshot.currentCommandId,
      lastProcessedCommandSequence: 0,
      capturedAt: new Date().toISOString(),
    };

    await this.runtime.sendRendererState(payload);
  }
}
