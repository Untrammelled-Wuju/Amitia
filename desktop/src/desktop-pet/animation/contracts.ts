export type LoopType = "loop" | "once" | "hold" | "ping_pong";

export type ReturnTarget =
  | { type: "default" }
  | { type: "previous" }
  | { type: "current_activity" }
  | { type: "none" }
  | { type: "action"; actionKey: string };

export type QueuePolicy =
  | "replace_current"
  | "enqueue"
  | "play_after_current"
  | "drop_if_busy"
  | "coalesce";

export type InterruptPolicy =
  | "respect_action"
  | "force_system"
  | "never_interrupt";

export type LoadPriority = "critical" | "high" | "normal" | "low";

export type PlayerPhase =
  | "uninitialized"
  | "loading_default"
  | "ready"
  | "loading_action"
  | "playing"
  | "paused"
  | "holding"
  | "recovering"
  | "failed"
  | "disposed";

export type InterruptionReason =
  | "replaced"
  | "system_force"
  | "package_switch"
  | "resource_failure"
  | "window_destroyed"
  | "runtime_reconnect"
  | "max_duration_reached"
  | "user_disabled";

export type CompletionReason =
  | "natural_end"
  | "max_duration_reached"
  | "expired";

export interface NormalizedFrame {
  readonly index: number;
  readonly resourceUrl: string;
  readonly durationMs: number;
  readonly cumulativeStartMs: number;
  readonly cumulativeEndMs: number;
  readonly frameId: string;
  readonly assetId: string;
  readonly contentHash: string;
}

export interface ActionSpecSnapshot {
  readonly actionKey: string;
  readonly displayName: string;
  readonly category?: string;
  readonly version: number;
  readonly loopType: LoopType;
  readonly defaultPriority?: number;
  readonly interruptible: boolean;
  readonly interruptAfterMs?: number;
  readonly minimumPlayMs?: number;
  readonly maximumPlayMs?: number | null;
  readonly cooldownMs?: number;
  readonly mutexGroup?: string | null;
  readonly returnTarget: ReturnTarget;
  readonly supportsDefaultIdle?: boolean;
  readonly isStableStateCandidate?: boolean;
  readonly isTransitionOnly?: boolean;
}

export interface RawActionConfig {
  readonly actionKey: string;
  readonly displayName: string;
  readonly actionName?: string;
  readonly version: number;
  readonly loopType: string;
  readonly playbackMode?: string;
  readonly fps: number;
  readonly defaultFps?: number;
  readonly frameDurationMs: number;
  readonly frameCount: number;
  readonly frames: ReadonlyArray<{ index?: number; file: string; durationMs?: number; frameId?: string; assetId?: string; contentHash?: string } | string>;
  readonly anchor?: { type?: string; x?: number; y?: number; coordinateSpace?: string };
  readonly interruptible?: boolean;
  readonly returnAction?: string;
  readonly returnTo?: { type?: string; actionKey?: string };
  readonly minimumPlayMs?: number;
  readonly interruptAfterMs?: number;
  readonly maximumPlayMs?: number | null;
  readonly defaultPriority?: number;
  readonly priority?: number;
  readonly cooldownMs?: number;
  readonly mutexGroup?: string | null;
  readonly supportsDefaultIdle?: boolean;
  readonly isStableStateCandidate?: boolean;
  readonly isTransitionOnly?: boolean;
  readonly category?: string;
  readonly schemaVersion?: number;
}

export interface LoadedAction {
  readonly packageId: string;
  readonly packageRevision: number;
  readonly actionKey: string;
  readonly displayName: string;
  readonly actionVersion: number;
  readonly loopType: LoopType;
  readonly frames: readonly NormalizedFrame[];
  readonly baseDurationMs: number;
  readonly cycleDurationMs: number;
  readonly anchor: { type: string; x: number; y: number };
  readonly interruptible: boolean;
  readonly interruptAfterMs: number;
  readonly minimumPlayMs: number;
  readonly maximumPlayMs: number | null;
  readonly defaultPriority: number;
  readonly cooldownMs: number;
  readonly mutexGroup: string | null;
  readonly returnTarget: ReturnTarget;
  readonly supportsDefaultIdle: boolean;
  readonly isStableStateCandidate: boolean;
  readonly isTransitionOnly: boolean;
  readonly warnings: readonly string[];
  readonly specSnapshot?: ActionSpecSnapshot;
}

export interface LoadedActionAssets {
  readonly action: LoadedAction;
  readonly decodedFrames: readonly DecodedFrame[];
  readonly totalEstimatedBytes: number;
}

export interface DecodedFrame {
  readonly frameIndex: number;
  readonly bitmap: ImageBitmap | HTMLImageElement;
  readonly width: number;
  readonly height: number;
  readonly estimatedBytes: number;
  readonly sourceUrl: string;
  readonly decoderName: string;
  readonly contentHash: string;
}

export interface PlayActionCommand {
  readonly commandId: string;
  readonly idempotencyKey: string;
  readonly installationId: string;
  readonly petInstanceId: string;
  readonly packageRevision: number;
  readonly actionKey: string;
  readonly priority: number;
  readonly queuePolicy: QueuePolicy;
  readonly interruptPolicy: InterruptPolicy;
  readonly playbackRate: number;
  readonly issuedAt: string;
  readonly expiresAt?: string;
  readonly returnOverride?: ReturnTarget;
  readonly traceId?: string;
  readonly source?: string;
}

export type CommandAckStatus =
  | "accepted"
  | "queued"
  | "rejected"
  | "expired"
  | "duplicate"
  | "satisfied"
  | "cancelled"
  | "stale";

export interface CommandAck {
  readonly commandId: string;
  readonly status: CommandAckStatus;
  readonly reason: string | null;
  readonly playbackInstanceId: string | null;
  readonly queuePosition: number;
  readonly actualPackageRevision: number;
}

export interface PlaybackSnapshot {
  readonly phase: PlayerPhase;
  readonly packageId: string | null;
  readonly packageRevision: number;
  readonly currentActionKey: string | null;
  readonly currentCommandId: string | null;
  readonly frameIndex: number | null;
  readonly localElapsedMs: number;
  readonly cycleIndex: number;
  readonly playbackRate: number;
  readonly queueLength: number;
  readonly previousStableActionKey: string | null;
  readonly defaultActionKey: string | null;
  readonly lastTransitionAtMonotonicMs: number;
  readonly lastError?: PlaybackErrorView;
}

export interface PlaybackErrorView {
  readonly code: string;
  readonly message: string;
  readonly actionKey?: string;
  readonly frameIndex?: number;
  readonly resourceUrl?: string;
  readonly decoder?: string;
  readonly playbackInstanceId?: string;
  readonly commandId?: string;
  readonly traceId?: string;
}

export interface PackagePlaybackSnapshot {
  readonly packageId: string;
  readonly packageRevision: number;
  readonly schemaVersion: number;
  readonly canvas: { width: number; height: number };
  readonly defaultActionKey: string;
  readonly actions: ReadonlyArray<{
    actionKey: string;
    configUrl: string;
    specSnapshot?: ActionSpecSnapshot;
  }>;
  readonly previewUrl?: string;
  readonly interpolationMode?: "nearest" | "smooth";
}

export interface PlaybackClock {
  now(): number;
  requestTick(callback: (now: number) => void): number;
  cancelTick(handle: number): void;
}

export interface TimelinePosition {
  readonly frameIndex: number;
  readonly cycleIndex: number;
  readonly localMs: number;
  readonly completed: boolean;
}

export interface FrameTimeline {
  readonly frames: readonly NormalizedFrame[];
  readonly forwardDurationMs: number;
  readonly pingPongSequence: readonly number[];
  readonly pingPongDurationMs: number;
  locate(localMs: number, loopType: LoopType): TimelinePosition;
}

export interface PetVisualSurface {
  configureCanvas(input: { width: number; height: number; scale: number; interpolationMode?: "nearest" | "smooth" }): void;
  present(frame: DecodedFrame, input: {
    anchor: { type: string; x: number; y: number };
    frameIndex: number;
    actionKey: string;
  }): Promise<PresentedFrameInfo> | PresentedFrameInfo;
  retainLastFrame(): void;
  clear(reason: string): void;
  captureHitMask(): AlphaHitMaskSnapshot;
  dispose(): void;
}

export interface PresentedFrameInfo {
  readonly presented: boolean;
  readonly frameIndex: number;
  readonly timestamp: number;
  readonly error?: string;
}

export interface AlphaHitMaskSnapshot {
  readonly width: number;
  readonly height: number;
  readonly data: Uint8Array;
  readonly threshold: number;
}

export type PlaybackEventType =
  | "playback.command_accepted"
  | "playback.command_rejected"
  | "playback.command_queued"
  | "playback.action_loading"
  | "playback.action_started"
  | "playback.frame_presented"
  | "playback.action_holding"
  | "playback.action_completed"
  | "playback.action_interrupted"
  | "playback.action_expired"
  | "playback.action_failed"
  | "playback.fallback_started"
  | "playback.fallback_failed"
  | "playback.default_changed"
  | "playback.package_switched"
  | "playback.cache_pressure"
  | "playback.recovered";

export interface PlaybackEvent {
  readonly type: PlaybackEventType;
  readonly playbackInstanceId?: string;
  readonly commandId?: string;
  readonly actionKey?: string;
  readonly frameIndex?: number;
  readonly reason?: string;
  readonly playedDurationMs?: number;
  readonly presentedFrames?: number;
  readonly droppedFramesEstimate?: number;
  readonly nextActionKey?: string;
  readonly timestamp: number;
  readonly error?: PlaybackErrorView;
  readonly packageId?: string;
  readonly packageRevision?: number;
  readonly traceId?: string;
}

export interface AnimationDiagnostics {
  readonly engineVersion: string;
  readonly snapshot: PlaybackSnapshot;
  readonly currentAction?: {
    key: string;
    loopType: LoopType;
    frameCount: number;
    cycleDurationMs: number;
    loadedBytes: number;
  };
  readonly queue: ReadonlyArray<{ actionKey: string; priority: number; expiresAt?: string }>;
  readonly cache: { budgetBytes: number; usedBytes: number; entries: number };
  readonly clock: { visible: boolean; suspended: boolean; lastGapMs: number };
  readonly recentTransitions: ReadonlyArray<{ from: string; to: string; reason: string }>;
  readonly recentErrors: PlaybackErrorView[];
}

export interface ActionAssetRepository {
  loadAction(input: {
    packageSnapshot: PackagePlaybackSnapshot;
    actionKey: string;
    signal: AbortSignal;
    priority: LoadPriority;
  }): Promise<LoadedActionAssets>;
}

export interface DecoderRegistry {
  decode(input: {
    url: string;
    signal: AbortSignal;
    contentHash?: string;
  }): Promise<DecodedFrame>;
  canHandle(mime: string): boolean;
}

export interface GenerationToken {
  readonly revision: number;
  readonly generation: number;
  isCurrent(): boolean;
  signal: AbortSignal;
}

export interface GenerationManager {
  next(revision: number): GenerationToken;
  current(): GenerationToken | null;
  isCurrent(token: GenerationToken): boolean;
}

export interface QueuedCommand {
  readonly command: PlayActionCommand;
  readonly acceptedMonotonicMs: number;
  readonly ack: CommandAck;
}

export interface CacheEntry {
  readonly key: string;
  readonly frame: DecodedFrame;
  readonly estimatedBytes: number;
  lastAccessedMs: number;
  refCount: number;
}

export interface PlaybackRecoverySnapshot {
  readonly packageId: string;
  readonly packageRevision: number;
  readonly defaultActionKey: string;
  readonly lastStableActionKey: string | null;
  readonly lastStableLocalElapsedMs: number;
  readonly lastStableCycleIndex: number;
}

export const ANIMATION_ENGINE_VERSION = "1.0.0";

export const DEFAULT_CACHE_BUDGET_BYTES = 128 * 1024 * 1024;

export const DEFAULT_QUEUE_LIMITS = {
  total: 16,
  perActionKey: 2,
  perMutexGroup: 4,
} as const;

export const FRAME_DURATION_MIN_MS = 16;
export const FRAME_DURATION_MAX_MS = 10000;
export const FPS_MIN = 1;
export const FPS_MAX = 120;
export const LEGACY_FRAME_DURATION_MS = 100;

export const CLOCK_LARGE_GAP_THRESHOLD_MS = 5000;

export const FALLBACK_MAX_DEPTH = 4;

export const IDEMPOTENCY_TTL_MS = 60000;
export const IDEMPOTENCY_MAX_ENTRIES = 256;

export const FINALIZED_SET_TTL_MS = 30000;
export const FINALIZED_SET_MAX_ENTRIES = 512;
