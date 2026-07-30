import type {
  LoadedAction,
  PackagePlaybackSnapshot,
  PlaybackRecoverySnapshot,
  PlayerPhase,
} from "./contracts";

export interface RecoveryContext {
  getCurrentPhase(): PlayerPhase;
  getPackageSnapshot(): PackagePlaybackSnapshot | null;
  getCurrentAction(): LoadedAction | null;
  getPreviousStableActionKey(): string | null;
  getLocalElapsedMs(): number;
  getCycleIndex(): number;
}

export class PlaybackRecoveryController {
  private lastStableSnapshot: PlaybackRecoverySnapshot | null = null;

  captureSnapshot(ctx: RecoveryContext): PlaybackRecoverySnapshot {
    const pkg = ctx.getPackageSnapshot();
    const currentAction = ctx.getCurrentAction();
    const isStable = currentAction !== null &&
      currentAction.isStableStateCandidate &&
      !currentAction.isTransitionOnly;

    const stableKey = isStable
      ? currentAction.actionKey
      : ctx.getPreviousStableActionKey();

    const snapshot: PlaybackRecoverySnapshot = {
      packageId: pkg?.packageId ?? "",
      packageRevision: pkg?.packageRevision ?? 0,
      defaultActionKey: pkg?.defaultActionKey ?? "",
      lastStableActionKey: stableKey,
      lastStableLocalElapsedMs: isStable ? ctx.getLocalElapsedMs() : 0,
      lastStableCycleIndex: isStable ? ctx.getCycleIndex() : 0,
    };

    this.lastStableSnapshot = snapshot;
    return snapshot;
  }

  getLastSnapshot(): PlaybackRecoverySnapshot | null {
    return this.lastStableSnapshot;
  }

  shouldRecoverToDefault(ctx: RecoveryContext): boolean {
    const phase = ctx.getCurrentPhase();
    if (phase === "failed" || phase === "disposed") return false;
    if (phase === "uninitialized") return false;
    const snapshot = this.lastStableSnapshot;
    if (!snapshot) return true;
    const pkg = ctx.getPackageSnapshot();
    if (!pkg) return true;
    if (snapshot.packageRevision !== pkg.packageRevision) return true;
    return false;
  }

  resolveRecoveryTarget(
    ctx: RecoveryContext,
    availableActionKeys: Set<string>,
  ): { actionKey: string; useExactPosition: boolean; localElapsedMs: number; cycleIndex: number } | null {
    const snapshot = this.lastStableSnapshot;
    const pkg = ctx.getPackageSnapshot();
    if (!pkg) return null;

    if (snapshot && snapshot.packageRevision === pkg.packageRevision) {
      if (
        snapshot.lastStableActionKey &&
        availableActionKeys.has(snapshot.lastStableActionKey)
      ) {
        return {
          actionKey: snapshot.lastStableActionKey,
          useExactPosition: true,
          localElapsedMs: snapshot.lastStableLocalElapsedMs,
          cycleIndex: snapshot.lastStableCycleIndex,
        };
      }
    }

    if (pkg.defaultActionKey && availableActionKeys.has(pkg.defaultActionKey)) {
      return {
        actionKey: pkg.defaultActionKey,
        useExactPosition: false,
        localElapsedMs: 0,
        cycleIndex: 0,
      };
    }

    return null;
  }

  clear(): void {
    this.lastStableSnapshot = null;
  }
}
