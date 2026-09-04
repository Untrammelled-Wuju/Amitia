import { DesktopPetAnimationEngine } from "../desktop-pet/animation/animation-engine";
import type { EventListener } from "../desktop-pet/animation/animation-engine";
import { CanvasPetVisualSurface } from "../desktop-pet/animation/surface/canvas-pet-visual-surface";
import { AlphaHitMaskAdapter } from "../desktop-pet/animation/surface/alpha-hit-mask-adapter";
import { DecoderRegistryImpl } from "../desktop-pet/animation/decoders/decoder-registry";
import { ImageBitmapDecoder } from "../desktop-pet/animation/decoders/image-bitmap-decoder";
import { HtmlImageDecoder } from "../desktop-pet/animation/decoders/html-image-decoder";
import { DecodedFrameCache } from "../desktop-pet/animation/cache/decoded-frame-cache";
import { FrameSequenceLoader } from "../desktop-pet/animation/loaders/frame-sequence-loader";
import { ActionAssetRepositoryImpl } from "../desktop-pet/animation/loaders/action-asset-repository";
import { createActionNormalizer } from "../desktop-pet/animation/loaders/action-config-normalizer";
import type {
  DecodedFrame,
  PackagePlaybackSnapshot,
  PlaybackEvent,
  PlaybackEventType,
  PlaybackSnapshot,
  PlayActionCommand,
} from "../desktop-pet/animation/contracts";
import type { PetAnimationApi } from "./pet-animation-globals";
import type { PetDragIpcPayload, PetHitMaskPayload, PetPointerIpcPayload, RuntimeReadyPayload } from "../shared/animation-ipc";
import { resolveLogicalCanvasPoint } from "./pet-pointer-coordinates";

function resolveResourceUrl(relativePath: string, configUrl: string): string {
  try {
    if (/^https?:\/\//i.test(relativePath) || /^file:\/\//i.test(relativePath)) {
      return relativePath;
    }
    const resolved = new URL(relativePath, configUrl).href;
    return resolved;
  } catch {
    return relativePath;
  }
}

function createEngine(canvas: HTMLCanvasElement): {
  engine: DesktopPetAnimationEngine;
  surface: CanvasPetVisualSurface;
} {
  const surface = new CanvasPetVisualSurface({ canvas });

  const decoders = [
    new ImageBitmapDecoder(),
    new HtmlImageDecoder(),
  ];
  const decoderRegistry = new DecoderRegistryImpl(decoders);

  const cache = new DecodedFrameCache({
    budgetBytes: 128 * 1024 * 1024,
  });

  const frameLoader = new FrameSequenceLoader({
    decoderRegistry,
    maxConcurrency: 3,
  });

  const normalizer = createActionNormalizer();

  const assetRepository = new ActionAssetRepositoryImpl({
    decoderRegistry,
    frameLoader,
    cache,
    normalizer,
    resolveResourceUrl,
  });

  const engine = new DesktopPetAnimationEngine({
    surface,
    assetRepository,
  });

  return { engine, surface };
}

function reportEventToMain(api: PetAnimationApi, event: PlaybackEvent): void {
  try {
    api.reportEvent(event);
  } catch (error) {
    console.error("[PetAnimation] failed to report playback event to main", error);
  }
}

const SNAPSHOT_HEARTBEAT_MS = 15000;
const SNAPSHOT_FRAME_MERGE_MS = 500;
const SNAPSHOT_HIDDEN_MS = 5000;
const DRAG_THRESHOLD_PX = 5;
const DOUBLE_CLICK_THRESHOLD_MS = 250;
const DOUBLE_CLICK_DISTANCE_PX = 8;
const DRAG_CLICK_SUPPRESSION_MS = 350;

interface InteractionState {
  suppressClickUntil: number;
}

function normalizeInterruptionReason(reason: string): import("../desktop-pet/animation/contracts").InterruptionReason {
  switch (reason) {
    case "replaced":
    case "system_force":
    case "package_switch":
    case "resource_failure":
    case "window_destroyed":
    case "runtime_reconnect":
    case "runtime_stop":
    case "command_cancelled":
    case "max_duration_reached":
    case "user_disabled":
      return reason;
    default:
      return "system_force";
  }
}

const KEY_SNAPSHOT_EVENTS: ReadonlySet<PlaybackEventType> = new Set<PlaybackEventType>([
  "playback.action_started",
  "playback.action_completed",
  "playback.action_interrupted",
  "playback.action_failed",
  "playback.package_switched",
]);

let lastSnapshotHash = "";
let lastSnapshotSentAt = 0;
let frameUpdateTimer: ReturnType<typeof setTimeout> | null = null;

let currentPackageRevision = 0;
let currentActionKey: string | null = null;
let currentPlaybackInstanceId: string = "";
let maskRevision = 0;

function computeSnapshotHash(snapshot: PlaybackSnapshot): string {
  return `${snapshot.phase}:${snapshot.packageRevision}:${snapshot.currentActionKey}:${snapshot.currentCommandId}:${snapshot.frameIndex}:${snapshot.cycleIndex}:${snapshot.queueLength}`;
}

function sendSnapshotIfChanged(
  api: PetAnimationApi,
  engine: DesktopPetAnimationEngine,
  force = false,
): void {
  const snapshot = engine.getSnapshot();
  const hash = computeSnapshotHash(snapshot);
  if (!force && hash === lastSnapshotHash) return;
  lastSnapshotHash = hash;
  lastSnapshotSentAt = Date.now();
  try {
    api.reportSnapshot(snapshot);
  } catch {
    void 0;
  }
}

function scheduleFrameSnapshot(
  api: PetAnimationApi,
  engine: DesktopPetAnimationEngine,
): void {
  if (frameUpdateTimer !== null) return;
  frameUpdateTimer = setTimeout(() => {
    frameUpdateTimer = null;
    if (engine.isDisposed()) return;
    sendSnapshotIfChanged(api, engine);
  }, SNAPSHOT_FRAME_MERGE_MS);
}

function startSnapshotReporting(
  api: PetAnimationApi,
  engine: DesktopPetAnimationEngine,
): () => void {
  const heartbeat = setInterval(() => {
    if (engine.isDisposed()) return;
    const now = Date.now();
    const interval = document.hidden ? SNAPSHOT_HIDDEN_MS : SNAPSHOT_HEARTBEAT_MS;
    if (now - lastSnapshotSentAt >= interval) {
      sendSnapshotIfChanged(api, engine, true);
    }
  }, 1000);

  return () => {
    clearInterval(heartbeat);
    if (frameUpdateTimer !== null) {
      clearTimeout(frameUpdateTimer);
      frameUpdateTimer = null;
    }
  };
}

function buildPointerPayload(
  e: MouseEvent,
  canvas: HTMLCanvasElement,
): PetPointerIpcPayload {
  const point = resolveLogicalCanvasPoint(canvas, e.clientX, e.clientY);
  return {
    canvasX: point.x,
    canvasY: point.y,
    screenX: e.screenX,
    screenY: e.screenY,
    occurredAt: Date.now(),
  };
}

function attachInteractionListeners(
  api: PetAnimationApi,
  canvas: HTMLCanvasElement,
  interactionState: InteractionState,
): () => void {
  let clickTimer: ReturnType<typeof setTimeout> | null = null;
  let pendingClick: PetPointerIpcPayload | null = null;
  let lastHoverX = 0;
  let lastHoverY = 0;
  let lastHoverTime = 0;
  let isHovering = false;

  const flushPendingSingleClick = () => {
    if (!pendingClick) return;
    const payload = pendingClick;
    pendingClick = null;
    api.sendClick(payload);
  };

  const onClick = (e: MouseEvent) => {
    if (e.button !== 0 || Date.now() < interactionState.suppressClickUntil) {
      return;
    }
    const payload = buildPointerPayload(e, canvas);
    if (clickTimer !== null && pendingClick) {
      const dx = payload.canvasX - pendingClick.canvasX;
      const dy = payload.canvasY - pendingClick.canvasY;
      const distance = Math.sqrt(dx * dx + dy * dy);
      if (distance <= DOUBLE_CLICK_DISTANCE_PX) {
        clearTimeout(clickTimer);
        clickTimer = null;
        pendingClick = null;
        api.sendDoubleClick(payload);
        return;
      }

      // A second click far away is not a double click. Deliver the first click
      // immediately and begin a fresh double-click window for the new point.
      clearTimeout(clickTimer);
      clickTimer = null;
      flushPendingSingleClick();
    }

    pendingClick = payload;
    clickTimer = setTimeout(() => {
      clickTimer = null;
      flushPendingSingleClick();
    }, DOUBLE_CLICK_THRESHOLD_MS);
  };

  const onMouseMove = (e: MouseEvent) => {
    const now = Date.now();
    const payload = buildPointerPayload(e, canvas);
    const dx = payload.canvasX - lastHoverX;
    const dy = payload.canvasY - lastHoverY;
    const dist = Math.sqrt(dx * dx + dy * dy);
    if (isHovering && now - lastHoverTime < 100 && dist < 8) return;
    isHovering = true;
    lastHoverX = payload.canvasX;
    lastHoverY = payload.canvasY;
    lastHoverTime = now;
    api.sendHover(payload);
  };

  const onMouseLeave = () => {
    if (!isHovering) return;
    // A hover IPC message represents an affirmative hover observation. Sending
    // (-1, -1) here used to look like another hover to Backend Behavior and
    // could keep the pet in gesture_hover after the pointer had already left.
    // The backend gesture lease now expires the last observation naturally.
    isHovering = false;
  };

  document.addEventListener("click", onClick);
  document.addEventListener("mousemove", onMouseMove);
  document.addEventListener("mouseleave", onMouseLeave);

  return () => {
    document.removeEventListener("click", onClick);
    document.removeEventListener("mousemove", onMouseMove);
    document.removeEventListener("mouseleave", onMouseLeave);
    if (clickTimer !== null) {
      clearTimeout(clickTimer);
      clickTimer = null;
    }
    pendingClick = null;
  };
}

function buildDragPayload(
  e: PointerEvent,
  canvas: HTMLCanvasElement,
): PetDragIpcPayload {
  const point = resolveLogicalCanvasPoint(canvas, e.clientX, e.clientY);
  return {
    pointerId: e.pointerId,
    screenX: e.screenX,
    screenY: e.screenY,
    canvasX: point.x,
    canvasY: point.y,
    occurredAt: Date.now(),
  };
}

function attachDragListeners(
  api: PetAnimationApi,
  canvas: HTMLCanvasElement,
  interactionState: InteractionState,
): () => void {
  let pointerDown = false;
  let dragging = false;
  let startX = 0;
  let startY = 0;
  let activePointerId = -1;

  const onPointerDown = (e: PointerEvent) => {
    if (pointerDown) return;
    pointerDown = true;
    activePointerId = e.pointerId;
    startX = e.screenX;
    startY = e.screenY;
    dragging = false;
  };

  const onPointerMove = (e: PointerEvent) => {
    if (!pointerDown || e.pointerId !== activePointerId) return;
    const dx = e.screenX - startX;
    const dy = e.screenY - startY;
    const dist = Math.sqrt(dx * dx + dy * dy);
    if (!dragging && dist >= DRAG_THRESHOLD_PX) {
      dragging = true;
      interactionState.suppressClickUntil = Date.now() + DRAG_CLICK_SUPPRESSION_MS;
      api.sendDragStart(buildDragPayload(e, canvas));
      return;
    }
    if (dragging) {
      api.sendDragMove(buildDragPayload(e, canvas));
    }
  };

  const onPointerUp = (e: PointerEvent) => {
    if (!pointerDown || e.pointerId !== activePointerId) return;
    if (dragging) {
      interactionState.suppressClickUntil = Date.now() + DRAG_CLICK_SUPPRESSION_MS;
      api.sendDragEnd(buildDragPayload(e, canvas));
    }
    pointerDown = false;
    dragging = false;
    activePointerId = -1;
  };

  const onPointerCancel = (e: PointerEvent) => {
    if (!pointerDown || e.pointerId !== activePointerId) return;
    if (dragging) {
      interactionState.suppressClickUntil = Date.now() + DRAG_CLICK_SUPPRESSION_MS;
      api.sendDragCancel(buildDragPayload(e, canvas));
    }
    pointerDown = false;
    dragging = false;
    activePointerId = -1;
  };

  document.addEventListener("pointerdown", onPointerDown);
  document.addEventListener("pointermove", onPointerMove);
  document.addEventListener("pointerup", onPointerUp);
  document.addEventListener("pointercancel", onPointerCancel);

  return () => {
    document.removeEventListener("pointerdown", onPointerDown);
    document.removeEventListener("pointermove", onPointerMove);
    document.removeEventListener("pointerup", onPointerUp);
    document.removeEventListener("pointercancel", onPointerCancel);
  };
}

async function main(): Promise<void> {
  const api = window.petAnimationApi;
  if (!api) {
    console.error("[PetAnimation] petAnimationApi not available");
    return;
  }

  const canvas = document.getElementById("pet-canvas") as HTMLCanvasElement | null;
  if (!canvas) {
    const reason = "canvas element not found";
    console.error(`[PetAnimation] ${reason}`);
    api.sendRuntimeInitFailed({ reason });
    return;
  }

  const { engine, surface } = createEngine(canvas);
  const hitMaskAdapter = new AlphaHitMaskAdapter({ surface });
  const hitMaskPlaceholderFrame: DecodedFrame = {
    frameIndex: 0,
    bitmap: new Image(),
    width: 0,
    height: 0,
    estimatedBytes: 0,
    sourceUrl: "",
    decoderName: "",
    contentHash: "",
  };

  const eventListener: EventListener = (event: PlaybackEvent) => {
    reportEventToMain(api, event);
    if (engine.isDisposed()) return;

    if (event.type === "playback.action_started" && event.actionKey) {
      currentActionKey = event.actionKey;
      if (event.playbackInstanceId) {
        currentPlaybackInstanceId = event.playbackInstanceId;
      }
    }

    if (KEY_SNAPSHOT_EVENTS.has(event.type)) {
      if (frameUpdateTimer !== null) {
        clearTimeout(frameUpdateTimer);
        frameUpdateTimer = null;
      }
      sendSnapshotIfChanged(api, engine, true);
    } else if (event.type === "playback.frame_presented") {
      scheduleFrameSnapshot(api, engine);
      const hitMaskUpdated = hitMaskAdapter.updateHitMask(hitMaskPlaceholderFrame);
      const mask = hitMaskAdapter.getMask();
      if (hitMaskUpdated && mask.width > 0 && mask.height > 0) {
        maskRevision += 1;
        const hitMaskPayload: PetHitMaskPayload = {
          width: mask.width,
          height: mask.height,
          data: mask.data,
          threshold: mask.threshold,
          packageRevision: currentPackageRevision,
          actionKey: currentActionKey ?? "",
          frameIndex: event.frameIndex ?? 0,
          playbackInstanceId: currentPlaybackInstanceId,
          maskRevision,
        };
        try {
          api.reportHitMask(hitMaskPayload);
        } catch {
          void 0;
        }
      }
    }
  };
  const unsubEvent = engine.onEvent(eventListener);

  const interactionState: InteractionState = { suppressClickUntil: 0 };
  const disposeListeners = attachInteractionListeners(api, canvas, interactionState);
  const disposeDragListeners = attachDragListeners(api, canvas, interactionState);

  const unsubPlayAction = api.onPlayAction((command: PlayActionCommand) => {
    const rendererCommand: PlayActionCommand = {
      ...command,
      playbackInstanceId: command.playbackInstanceId || crypto.randomUUID(),
    };
    void engine.playAction(rendererCommand).catch((error) => {
      console.error("[PetAnimation] playAction failed", error);
    });
  });

  const unsubPause = api.onPause(() => {
    engine.pause();
  });

  const unsubResume = api.onResume(() => {
    engine.resume();
  });

  const unsubStop = api.onStop((reason) => {
    engine.stop(normalizeInterruptionReason(reason));
  });

  const unsubSwitchPackage = api.onSwitchPackage((snapshot: PackagePlaybackSnapshot) => {
    void engine.switchPackage(snapshot).catch((error) => {
      console.error("[PetAnimation] package switch failed:", error);
    });
  });

  const unsubWindowHidden = api.onWindowHidden(() => {
    engine.onWindowHidden();
  });

  const unsubWindowShown = api.onWindowShown(() => {
    engine.onWindowShown();
  });

  const unsubSystemSuspend = api.onSystemSuspend(() => {
    engine.onSystemSuspend();
  });

  const unsubSystemResume = api.onSystemResume(() => {
    engine.onSystemResume();
  });

  const unsubRecovery = api.onRecovery((snapshot) => {
    void engine.onRendererRecover(snapshot).catch((error) => {
      console.error("[PetAnimation] renderer recovery failed", error);
    });
  });

  const unsubUpdateDefault = api.onUpdateDefaultAction((actionKey: string) => {
    engine.updateDefaultAction(actionKey, null);
  });

  const onVisibilityChange = () => {
    if (document.hidden) {
      engine.onWindowHidden();
    } else {
      engine.onWindowShown();
    }
  };
  document.addEventListener("visibilitychange", onVisibilityChange);

  const disposeSnapshotReporting = startSnapshotReporting(api, engine);

  window.addEventListener("beforeunload", () => {
    unsubEvent();
    unsubPlayAction();
    unsubPause();
    unsubResume();
    unsubStop();
    unsubSwitchPackage();
    unsubWindowHidden();
    unsubWindowShown();
    unsubSystemSuspend();
    unsubSystemResume();
    unsubRecovery();
    unsubUpdateDefault();
    disposeListeners();
    disposeDragListeners();
    disposeSnapshotReporting();
    document.removeEventListener("visibilitychange", onVisibilityChange);
    hitMaskAdapter.dispose();
    engine.dispose();
  });

  api.sendRendererBootstrapped();

  try {
    const snapshot = await api.getPackageSnapshot();
    if (snapshot) {
      currentPackageRevision = snapshot.packageRevision;
      await engine.initialize(snapshot);
      if (engine.getPhase() !== "playing") {
        throw new Error(`runtime initialized without playable first frame: phase=${engine.getPhase()}`);
      }
      console.log("[PetAnimation] engine initialized successfully");
      const runtimeReadyPayload: RuntimeReadyPayload = {
        snapshotApplied: true,
        packageId: snapshot.packageId,
        packageRevision: snapshot.packageRevision,
        defaultActionKey: snapshot.defaultActionKey,
      };
      api.sendRuntimeReady(runtimeReadyPayload);
    } else {
      console.warn("[PetAnimation] no package snapshot available, waiting...");
    }
  } catch (error) {
    const reason = error instanceof Error ? error.message : String(error);
    console.error("[PetAnimation] initialization failed:", error);
    api.sendRuntimeInitFailed({
      reason: reason.slice(0, 2048),
      packageRevision: currentPackageRevision || undefined,
    });
  }
}

main().catch((error) => {
  const reason = error instanceof Error ? error.message : String(error);
  console.error("[PetAnimation] uncaught error:", error);
  try {
    window.petAnimationApi?.sendRuntimeInitFailed({
      reason: reason.slice(0, 2048),
      packageRevision: currentPackageRevision || undefined,
    });
  } catch {
    void 0;
  }
});
