import { contextBridge, ipcRenderer } from "electron";
import { ANIMATION_IPC_CHANNELS } from "../shared/animation-ipc";
import type { PackagePlaybackSnapshot } from "../desktop-pet/animation/contracts";
import type { PlaybackEvent, PlaybackSnapshot, PlayActionCommand, PlaybackRecoverySnapshot } from "../desktop-pet/animation/contracts";

type PetManagerState =
  | "uninitialized"
  | "ready"
  | "enabled"
  | "disabled"
  | "invalid";

type AssistantState =
  | "assistant_listening"
  | "assistant_thinking"
  | "assistant_speaking"
  | "assistant_finished"
  | "assistant_error";

interface PetActionSwitchPayload {
  actionKey: string;
  previousActionKey: string | null;
  source: string;
}

interface PetLoadErrorPayload {
  installationId: string;
  error: string;
  errorCode: string;
}

interface PetStatePayload {
  state: PetManagerState;
  installationId: string | null;
  reason?: string;
}

interface ChatStatePetPayload {
  state: AssistantState;
  actionKey: string;
  roundId: string | null;
}

interface ResolveResourceUrlResult {
  url: string;
  mime: string;
}

const legacyPetApi = {
  onActionSwitch(
    callback: (payload: PetActionSwitchPayload) => void,
  ): () => void {
    const listener = (
      _event: Electron.IpcRendererEvent,
      payload: PetActionSwitchPayload,
    ) => callback(payload);
    ipcRenderer.on("pet:action-switch", listener);
    return () => ipcRenderer.removeListener("pet:action-switch", listener);
  },
  onLoadError(callback: (payload: PetLoadErrorPayload) => void): () => void {
    const listener = (
      _event: Electron.IpcRendererEvent,
      payload: PetLoadErrorPayload,
    ) => callback(payload);
    ipcRenderer.on("pet:load-error", listener);
    return () => ipcRenderer.removeListener("pet:load-error", listener);
  },
  onState(callback: (payload: PetStatePayload) => void): () => void {
    const listener = (
      _event: Electron.IpcRendererEvent,
      payload: PetStatePayload,
    ) => callback(payload);
    ipcRenderer.on("pet:state", listener);
    return () => ipcRenderer.removeListener("pet:state", listener);
  },
  onChatState(callback: (payload: ChatStatePetPayload) => void): () => void {
    const listener = (
      _event: Electron.IpcRendererEvent,
      payload: ChatStatePetPayload,
    ) => callback(payload);
    ipcRenderer.on("pet:chat-state", listener);
    return () => ipcRenderer.removeListener("pet:chat-state", listener);
  },
  sendClick(x: number, y: number): void {
    ipcRenderer.send("pet:click", { x, y });
  },
  sendDoubleClick(x: number, y: number): void {
    ipcRenderer.send("pet:double-click", { x, y });
  },
  sendHover(x: number, y: number): void {
    ipcRenderer.send("pet:hover", { x, y });
  },
};

const animationApi = {
  getPackageSnapshot(): Promise<PackagePlaybackSnapshot | null> {
    return ipcRenderer.invoke(ANIMATION_IPC_CHANNELS.getPackageSnapshot);
  },

  resolveResourceUrl(relativePath: string): Promise<ResolveResourceUrlResult> {
    return ipcRenderer.invoke(ANIMATION_IPC_CHANNELS.resolveResourceUrl, relativePath);
  },

  reportEvent(event: PlaybackEvent): void {
    ipcRenderer.send(ANIMATION_IPC_CHANNELS.reportEvent, event);
  },

  reportSnapshot(snapshot: PlaybackSnapshot): void {
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

  sendRendererReady(): void {
    ipcRenderer.send(ANIMATION_IPC_CHANNELS.rendererReady);
  },

  sendRendererReadyAck(payload: { snapshotApplied: boolean }): void {
    ipcRenderer.send(ANIMATION_IPC_CHANNELS.rendererReadyAck, payload);
  },

  reportHitMask(width: number, height: number, data: Uint8Array, threshold: number): void {
    ipcRenderer.send(ANIMATION_IPC_CHANNELS.hitMask, { width, height, data, threshold });
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

contextBridge.exposeInMainWorld("petApi", legacyPetApi);
contextBridge.exposeInMainWorld("petAnimationApi", animationApi);
