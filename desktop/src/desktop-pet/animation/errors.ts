export const PLAYBACK_ERROR_CODES = {
  COMMAND_INVALID: "PLAYBACK_COMMAND_INVALID",
  COMMAND_EXPIRED: "PLAYBACK_COMMAND_EXPIRED",
  ACTION_NOT_FOUND: "PLAYBACK_ACTION_NOT_FOUND",
  ACTION_NOT_INTERRUPTIBLE: "PLAYBACK_ACTION_NOT_INTERRUPTIBLE",
  QUEUE_FULL: "PLAYBACK_QUEUE_FULL",
  PACKAGE_REVISION_MISMATCH: "PLAYBACK_PACKAGE_REVISION_MISMATCH",
  ACTION_CONFIG_INVALID: "PLAYBACK_ACTION_CONFIG_INVALID",
  FRAME_PATH_INVALID: "PLAYBACK_FRAME_PATH_INVALID",
  FRAME_FETCH_FAILED: "PLAYBACK_FRAME_FETCH_FAILED",
  FRAME_DECODE_FAILED: "PLAYBACK_FRAME_DECODE_FAILED",
  TIMELINE_INVALID: "PLAYBACK_TIMELINE_INVALID",
  SURFACE_FAILED: "PLAYBACK_SURFACE_FAILED",
  CACHE_BUDGET_EXCEEDED: "PLAYBACK_CACHE_BUDGET_EXCEEDED",
  FALLBACK_FAILED: "PLAYBACK_FALLBACK_FAILED",
  ENGINE_DISPOSED: "PLAYBACK_ENGINE_DISPOSED",
  INTERNAL_STATE_INVALID: "PLAYBACK_INTERNAL_STATE_INVALID",
} as const;

export type PlaybackErrorCode = typeof PLAYBACK_ERROR_CODES[keyof typeof PLAYBACK_ERROR_CODES];

const RECOVERABLE_ERRORS = new Set<PlaybackErrorCode>([
  PLAYBACK_ERROR_CODES.ACTION_NOT_FOUND,
  PLAYBACK_ERROR_CODES.FRAME_FETCH_FAILED,
  PLAYBACK_ERROR_CODES.FRAME_DECODE_FAILED,
  PLAYBACK_ERROR_CODES.QUEUE_FULL,
  PLAYBACK_ERROR_CODES.COMMAND_EXPIRED,
  PLAYBACK_ERROR_CODES.PACKAGE_REVISION_MISMATCH,
  PLAYBACK_ERROR_CODES.ACTION_CONFIG_INVALID,
  PLAYBACK_ERROR_CODES.FRAME_PATH_INVALID,
  PLAYBACK_ERROR_CODES.COMMAND_INVALID,
]);

const ENGINE_LEVEL_ERRORS = new Set<PlaybackErrorCode>([
  PLAYBACK_ERROR_CODES.SURFACE_FAILED,
  PLAYBACK_ERROR_CODES.INTERNAL_STATE_INVALID,
  PLAYBACK_ERROR_CODES.FALLBACK_FAILED,
]);

export class PlaybackError extends Error {
  readonly code: PlaybackErrorCode;
  readonly actionKey?: string;
  readonly frameIndex?: number;
  readonly resourceUrl?: string;
  readonly decoder?: string;
  readonly playbackInstanceId?: string;
  readonly commandId?: string;
  readonly traceId?: string;
  readonly packageId?: string;
  readonly packageRevision?: number;

  constructor(
    code: PlaybackErrorCode,
    message: string,
    context?: {
      actionKey?: string;
      frameIndex?: number;
      resourceUrl?: string;
      decoder?: string;
      playbackInstanceId?: string;
      commandId?: string;
      traceId?: string;
      packageId?: string;
      packageRevision?: number;
    },
  ) {
    super(message);
    this.name = "PlaybackError";
    this.code = code;
    if (context) {
      this.actionKey = context.actionKey;
      this.frameIndex = context.frameIndex;
      this.resourceUrl = context.resourceUrl;
      this.decoder = context.decoder;
      this.playbackInstanceId = context.playbackInstanceId;
      this.commandId = context.commandId;
      this.traceId = context.traceId;
      this.packageId = context.packageId;
      this.packageRevision = context.packageRevision;
    }
  }

  isRecoverable(): boolean {
    return RECOVERABLE_ERRORS.has(this.code);
  }

  isEngineLevel(): boolean {
    return ENGINE_LEVEL_ERRORS.has(this.code);
  }

  toView(): {
    code: string;
    message: string;
    actionKey?: string;
    frameIndex?: number;
    resourceUrl?: string;
    decoder?: string;
    playbackInstanceId?: string;
    commandId?: string;
    traceId?: string;
  } {
    return {
      code: this.code,
      message: this.message,
      actionKey: this.actionKey,
      frameIndex: this.frameIndex,
      resourceUrl: this.resourceUrl,
      decoder: this.decoder,
      playbackInstanceId: this.playbackInstanceId,
      commandId: this.commandId,
      traceId: this.traceId,
    };
  }

  static isAbort(error: unknown): boolean {
    if (error instanceof DOMException && error.name === "AbortError") return true;
    if (error instanceof Error && error.name === "AbortError") return true;
    return false;
  }

  static fromUnknown(error: unknown, fallbackCode: PlaybackErrorCode = PLAYBACK_ERROR_CODES.INTERNAL_STATE_INVALID): PlaybackError {
    if (error instanceof PlaybackError) return error;
    if (error instanceof Error) {
      return new PlaybackError(fallbackCode, error.message);
    }
    return new PlaybackError(fallbackCode, String(error));
  }
}

export function isRecoverableError(code: PlaybackErrorCode): boolean {
  return RECOVERABLE_ERRORS.has(code);
}

export function isEngineLevelError(code: PlaybackErrorCode): boolean {
  return ENGINE_LEVEL_ERRORS.has(code);
}
