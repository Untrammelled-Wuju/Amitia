import type { PackagePlaybackSnapshot } from "../desktop-pet/animation/contracts";
import type { PlaybackEvent, PlaybackSnapshot, PlayActionCommand, PlaybackRecoverySnapshot } from "../desktop-pet/animation/contracts";
import type { PetDragIpcPayload, PetHitMaskPayload, RuntimeInitFailedPayload, RuntimeReadyPayload } from "../shared/animation-ipc";

export interface ResolveResourceUrlResult {
  url: string;
  mime: string;
}

export interface PetAnimationApi {
  getPackageSnapshot(): Promise<PackagePlaybackSnapshot | null>;
  resolveResourceUrl(relativePath: string): Promise<ResolveResourceUrlResult>;
  reportEvent(event: PlaybackEvent): void;
  reportSnapshot(snapshot: PlaybackSnapshot): void;
  getDiagnostics(): Promise<unknown>;
  sendClick(x: number, y: number): void;
  sendDoubleClick(x: number, y: number): void;
  sendHover(x: number, y: number): void;
  sendRendererBootstrapped(): void;
  sendRuntimeReady(payload: RuntimeReadyPayload): void;
  sendRuntimeInitFailed(payload: RuntimeInitFailedPayload): void;
  reportHitMask(payload: PetHitMaskPayload): void;
  sendDragStart(payload: PetDragIpcPayload): void;
  sendDragMove(payload: PetDragIpcPayload): void;
  sendDragEnd(payload: PetDragIpcPayload): void;
  sendDragCancel(payload: PetDragIpcPayload): void;
  onPlayAction(callback: (command: PlayActionCommand) => void): () => void;
  onPause(callback: () => void): () => void;
  onResume(callback: () => void): () => void;
  onStop(callback: () => void): () => void;
  onSwitchPackage(callback: (snapshot: PackagePlaybackSnapshot) => void): () => void;
  onWindowHidden(callback: () => void): () => void;
  onWindowShown(callback: () => void): () => void;
  onSystemSuspend(callback: () => void): () => void;
  onSystemResume(callback: () => void): () => void;
  onRecovery(callback: (snapshot: PlaybackRecoverySnapshot) => void): () => void;
  onUpdateDefaultAction(callback: (actionKey: string) => void): () => void;
}

declare global {
  interface Window {
    petAnimationApi?: PetAnimationApi;
    petApi?: {
      onActionSwitch(callback: (payload: unknown) => void): () => void;
      onLoadError(callback: (payload: unknown) => void): () => void;
      onState(callback: (payload: unknown) => void): () => void;
      onChatState(callback: (payload: unknown) => void): () => void;
      sendClick(x: number, y: number): void;
      sendDoubleClick(x: number, y: number): void;
      sendHover(x: number, y: number): void;
    };
  }
}

export {};
