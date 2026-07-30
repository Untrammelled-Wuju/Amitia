import type { PackagePlaybackSnapshot } from "../desktop-pet/animation/contracts";
import type { PlaybackEvent, PlaybackSnapshot, PlayActionCommand, PlaybackRecoverySnapshot } from "../desktop-pet/animation/contracts";

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
      onFrameUpdate(callback: (payload: unknown) => void): () => void;
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
