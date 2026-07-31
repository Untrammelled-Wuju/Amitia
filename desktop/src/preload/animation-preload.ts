import { contextBridge, ipcRenderer } from "electron";
import { ANIMATION_IPC_CHANNELS } from "../shared/animation-ipc";
import type {
  PetDragIpcPayload,
  PetHitMaskPayload,
  RuntimeReadyPayload,
} from "../shared/animation-ipc";
import type { PackagePlaybackSnapshot } from "../desktop-pet/animation/contracts";
import type { PlaybackEvent, PlaybackSnapshot, PlayActionCommand, PlaybackRecoverySnapshot } from "../desktop-pet/animation/contracts";

interface ResolveResourceUrlResult {
  url: string;
  mime: string;
}

interface ReportEventPayload {
  type: PlaybackEvent["type"];
  playbackInstanceId?: string;
  commandId?: string;
  actionKey?: string;
  frameIndex?: number;
  reason?: string;
  playedDurationMs?: number;
  presentedFrames?: number;
  droppedFramesEstimate?: number;
  nextActionKey?: string;
  timestamp: number;
  error?: {
    code: string;
    message: string;
    actionKey?: string;
    frameIndex?: number;
    resourceUrl?: string;
    decoder?: string;
    playbackInstanceId?: string;
    commandId?: string;
    traceId?: string;
  };
  packageId?: string;
  packageRevision?: number;
  traceId?: string;
}

interface ReportSnapshotPayload {
  phase: PlaybackSnapshot["phase"];
  packageId: string | null;
  packageRevision: number;
  currentActionKey: string | null;
  currentCommandId: string | null;
  frameIndex: number | null;
  localElapsedMs: number;
  cycleIndex: number;
  playbackRate: number;
  queueLength: number;
  previousStableActionKey: string | null;
  defaultActionKey: string | null;
  lastTransitionAtMonotonicMs: number;
  lastError?: PlaybackSnapshot["lastError"];
}

const animationApi = {
  getPackageSnapshot(): Promise<PackagePlaybackSnapshot | null> {
    return ipcRenderer.invoke(ANIMATION_IPC_CHANNELS.getPackageSnapshot);
  },

  resolveResourceUrl(relativePath: string): Promise<ResolveResourceUrlResult> {
    return ipcRenderer.invoke(ANIMATION_IPC_CHANNELS.resolveResourceUrl, relativePath);
  },

  reportEvent(event: ReportEventPayload): void {
    ipcRenderer.send(ANIMATION_IPC_CHANNELS.reportEvent, event);
  },

  reportSnapshot(snapshot: ReportSnapshotPayload): void {
    ipcRenderer.send(ANIMATION_IPC_CHANNELS.reportSnapshot, snapshot);
  },

  getDiagnostics(): Promise<unknown> {
    return ipcRenderer.invoke(ANIMATION_IPC_CHANNELS.getDiagnostics);
  },

  sendClick(x: number, y: number): void {
    ipcRenderer.send(ANIMATION_IPC_CHANNELS.sendClick, { x, y });
  },

  sendDoubleClick(x: number, y: number): void {
    ipcRenderer.send(ANIMATION_IPC_CHANNELS.sendDoubleClick, { x, y });
  },

  sendHover(x: number, y: number): void {
    ipcRenderer.send(ANIMATION_IPC_CHANNELS.sendHover, { x, y });
  },

  sendRendererBootstrapped(): void {
    ipcRenderer.send(ANIMATION_IPC_CHANNELS.rendererBootstrapped);
  },

  sendRuntimeReady(payload: RuntimeReadyPayload): void {
    ipcRenderer.send(ANIMATION_IPC_CHANNELS.runtimeReady, payload);
  },

  reportHitMask(payload: PetHitMaskPayload): void {
    ipcRenderer.send(ANIMATION_IPC_CHANNELS.hitMask, payload);
  },

  sendDragStart(payload: PetDragIpcPayload): void {
    ipcRenderer.send(ANIMATION_IPC_CHANNELS.dragStart, payload);
  },

  sendDragMove(payload: PetDragIpcPayload): void {
    ipcRenderer.send(ANIMATION_IPC_CHANNELS.dragMove, payload);
  },

  sendDragEnd(payload: PetDragIpcPayload): void {
    ipcRenderer.send(ANIMATION_IPC_CHANNELS.dragEnd, payload);
  },

  sendDragCancel(payload: PetDragIpcPayload): void {
    ipcRenderer.send(ANIMATION_IPC_CHANNELS.dragCancel, payload);
  },

  onPlayAction(
    callback: (command: PlayActionCommand) => void,
  ): () => void {
    const listener = (
      _event: Electron.IpcRendererEvent,
      command: PlayActionCommand,
    ) => callback(command);
    ipcRenderer.on(ANIMATION_IPC_CHANNELS.playAction, listener);
    return () =>
      ipcRenderer.removeListener(ANIMATION_IPC_CHANNELS.playAction, listener);
  },

  onPause(callback: () => void): () => void {
    const listener = () => callback();
    ipcRenderer.on(ANIMATION_IPC_CHANNELS.pause, listener);
    return () =>
      ipcRenderer.removeListener(ANIMATION_IPC_CHANNELS.pause, listener);
  },

  onResume(callback: () => void): () => void {
    const listener = () => callback();
    ipcRenderer.on(ANIMATION_IPC_CHANNELS.resume, listener);
    return () =>
      ipcRenderer.removeListener(ANIMATION_IPC_CHANNELS.resume, listener);
  },

  onStop(callback: () => void): () => void {
    const listener = () => callback();
    ipcRenderer.on(ANIMATION_IPC_CHANNELS.stop, listener);
    return () =>
      ipcRenderer.removeListener(ANIMATION_IPC_CHANNELS.stop, listener);
  },

  onSwitchPackage(
    callback: (snapshot: PackagePlaybackSnapshot) => void,
  ): () => void {
    const listener = (
      _event: Electron.IpcRendererEvent,
      snapshot: PackagePlaybackSnapshot,
    ) => callback(snapshot);
    ipcRenderer.on(ANIMATION_IPC_CHANNELS.switchPackage, listener);
    return () =>
      ipcRenderer.removeListener(
        ANIMATION_IPC_CHANNELS.switchPackage,
        listener,
      );
  },

  onWindowHidden(callback: () => void): () => void {
    const listener = () => callback();
    ipcRenderer.on(ANIMATION_IPC_CHANNELS.windowHidden, listener);
    return () =>
      ipcRenderer.removeListener(
        ANIMATION_IPC_CHANNELS.windowHidden,
        listener,
      );
  },

  onWindowShown(callback: () => void): () => void {
    const listener = () => callback();
    ipcRenderer.on(ANIMATION_IPC_CHANNELS.windowShown, listener);
    return () =>
      ipcRenderer.removeListener(
        ANIMATION_IPC_CHANNELS.windowShown,
        listener,
      );
  },

  onSystemSuspend(callback: () => void): () => void {
    const listener = () => callback();
    ipcRenderer.on(ANIMATION_IPC_CHANNELS.systemSuspend, listener);
    return () =>
      ipcRenderer.removeListener(
        ANIMATION_IPC_CHANNELS.systemSuspend,
        listener,
      );
  },

  onSystemResume(callback: () => void): () => void {
    const listener = () => callback();
    ipcRenderer.on(ANIMATION_IPC_CHANNELS.systemResume, listener);
    return () =>
      ipcRenderer.removeListener(
        ANIMATION_IPC_CHANNELS.systemResume,
        listener,
      );
  },

  onRecovery(
    callback: (snapshot: PlaybackRecoverySnapshot) => void,
  ): () => void {
    const listener = (
      _event: Electron.IpcRendererEvent,
      snapshot: PlaybackRecoverySnapshot,
    ) => callback(snapshot);
    ipcRenderer.on(ANIMATION_IPC_CHANNELS.recovery, listener);
    return () =>
      ipcRenderer.removeListener(
        ANIMATION_IPC_CHANNELS.recovery,
        listener,
      );
  },

  onUpdateDefaultAction(
    callback: (actionKey: string) => void,
  ): () => void {
    const listener = (
      _event: Electron.IpcRendererEvent,
      actionKey: string,
    ) => callback(actionKey);
    ipcRenderer.on(ANIMATION_IPC_CHANNELS.updateDefaultAction, listener);
    return () =>
      ipcRenderer.removeListener(
        ANIMATION_IPC_CHANNELS.updateDefaultAction,
        listener,
      );
  },
};

contextBridge.exposeInMainWorld("petAnimationApi", animationApi);
