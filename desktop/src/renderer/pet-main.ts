import { DesktopPetAnimationEngine } from "../desktop-pet/animation/animation-engine";
import type { EventListener } from "../desktop-pet/animation/animation-engine";
import { CanvasPetVisualSurface } from "../desktop-pet/animation/surface/canvas-pet-visual-surface";
import { DecoderRegistryImpl } from "../desktop-pet/animation/decoders/decoder-registry";
import { ImageBitmapDecoder } from "../desktop-pet/animation/decoders/image-bitmap-decoder";
import { HtmlImageDecoder } from "../desktop-pet/animation/decoders/html-image-decoder";
import { DecodedFrameCache } from "../desktop-pet/animation/cache/decoded-frame-cache";
import { FrameSequenceLoader } from "../desktop-pet/animation/loaders/frame-sequence-loader";
import { ActionAssetRepositoryImpl } from "../desktop-pet/animation/loaders/action-asset-repository";
import { createActionNormalizer } from "../desktop-pet/animation/loaders/action-config-normalizer";
import type { PackagePlaybackSnapshot, PlayActionCommand, PlaybackEvent } from "../desktop-pet/animation/contracts";
import type { PetAnimationApi } from "./pet-animation-globals";

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

function createEngine(canvas: HTMLCanvasElement): DesktopPetAnimationEngine {
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

  return new DesktopPetAnimationEngine({
    surface,
    assetRepository,
  });
}

function reportEventToMain(api: PetAnimationApi, event: PlaybackEvent): void {
  try {
    api.reportEvent(event);
  } catch {
    void 0;
  }
}

function reportSnapshotToMain(api: PetAnimationApi, engine: DesktopPetAnimationEngine): void {
  try {
    const snapshot = engine.getSnapshot();
    api.reportSnapshot(snapshot);
  } catch {
    void 0;
  }
}

let snapshotReportTimer: ReturnType<typeof setInterval> | null = null;

function startSnapshotReporting(api: PetAnimationApi, engine: DesktopPetAnimationEngine): void {
  if (snapshotReportTimer !== null) return;
  snapshotReportTimer = setInterval(() => {
    if (engine.isDisposed()) {
      if (snapshotReportTimer !== null) {
        clearInterval(snapshotReportTimer);
        snapshotReportTimer = null;
      }
      return;
    }
    reportSnapshotToMain(api, engine);
  }, 1000);
}

function stopSnapshotReporting(): void {
  if (snapshotReportTimer !== null) {
    clearInterval(snapshotReportTimer);
    snapshotReportTimer = null;
  }
}

function attachInteractionListeners(api: PetAnimationApi): void {
  let lastHoverTime = 0;

  document.addEventListener("click", (e) => {
    api.sendClick(e.offsetX, e.offsetY);
  });

  document.addEventListener("dblclick", (e) => {
    api.sendDoubleClick(e.offsetX, e.offsetY);
  });

  document.addEventListener("mousemove", (e) => {
    const now = Date.now();
    if (now - lastHoverTime < 500) return;
    lastHoverTime = now;
    api.sendHover(e.offsetX, e.offsetY);
  });
}

async function main(): Promise<void> {
  const canvas = document.getElementById("pet-canvas") as HTMLCanvasElement | null;
  if (!canvas) {
    console.error("[PetAnimation] canvas element not found");
    return;
  }

  const api = window.petAnimationApi;
  if (!api) {
    console.error("[PetAnimation] petAnimationApi not available");
    return;
  }

  const engine = createEngine(canvas);

  const eventListener: EventListener = (event: PlaybackEvent) => {
    reportEventToMain(api, event);
  };
  engine.onEvent(eventListener);

  attachInteractionListeners(api);

  const unsubPlayAction = api.onPlayAction((command: PlayActionCommand) => {
    void engine.playAction(command);
  });

  const unsubPause = api.onPause(() => {
    engine.pause();
  });

  const unsubResume = api.onResume(() => {
    engine.resume();
  });

  const unsubStop = api.onStop(() => {
    engine.stop();
  });

  const unsubSwitchPackage = api.onSwitchPackage((snapshot: PackagePlaybackSnapshot) => {
    void engine.switchPackage(snapshot);
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
    engine.onRendererRecover(snapshot);
  });

  const unsubUpdateDefault = api.onUpdateDefaultAction((actionKey: string) => {
    engine.updateDefaultAction(actionKey, null);
  });

  document.addEventListener("visibilitychange", () => {
    if (document.hidden) {
      engine.onWindowHidden();
    } else {
      engine.onWindowShown();
    }
  });

  window.addEventListener("beforeunload", () => {
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
    stopSnapshotReporting();
    engine.dispose();
  });

  startSnapshotReporting(api, engine);

  try {
    const snapshot = await api.getPackageSnapshot();
    if (snapshot) {
      await engine.initialize(snapshot);
      console.log("[PetAnimation] engine initialized successfully");
    } else {
      console.warn("[PetAnimation] no package snapshot available, waiting...");
    }
  } catch (error) {
    console.error("[PetAnimation] initialization failed:", error);
  }
}

main().catch((error) => {
  console.error("[PetAnimation] uncaught error:", error);
});
