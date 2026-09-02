import type { MenuHost } from "./menu-host";
import type { TrayHost } from "./tray-host";
import type { ShortcutHost } from "./shortcut-host";
import type { GlobalShortcutHost } from "./global-shortcut-host";
import type { DesktopSnapshot, ResolvedContribution } from "./types";

export class SnapshotApplier {
  private readonly menuHost: MenuHost;
  private readonly trayHost: TrayHost;
  private readonly shortcutHost: ShortcutHost;
  private readonly globalShortcutHost: GlobalShortcutHost;
  private previousSnapshot: DesktopSnapshot | null = null;

  constructor(
    menuHost: MenuHost,
    trayHost: TrayHost,
    shortcutHost: ShortcutHost,
    globalShortcutHost: GlobalShortcutHost,
  ) {
    this.menuHost = menuHost;
    this.trayHost = trayHost;
    this.shortcutHost = shortcutHost;
    this.globalShortcutHost = globalShortcutHost;
  }

  apply(snapshot: DesktopSnapshot): boolean {
    if (this.hasBlockingConflicts(snapshot)) {
      return false;
    }
    const filtered = this.filterSupported(snapshot);
    try {
      this.applyInternal(filtered);
      this.previousSnapshot = filtered;
      return true;
    } catch (err) {
      console.error("[DesktopHost] Snapshot apply failed, rolling back", err);
      if (this.previousSnapshot) {
        try {
          this.applyInternal(this.previousSnapshot);
        } catch (rollbackErr) {
          console.error("[DesktopHost] Rollback failed", rollbackErr);
          this.cleanupAll();
        }
      } else {
        this.cleanupAll();
      }
      return false;
    }
  }

  private applyInternal(snapshot: DesktopSnapshot): void {
    this.globalShortcutHost.applySnapshot(snapshot);
    this.shortcutHost.applySnapshot(snapshot);
    this.menuHost.applySnapshot(snapshot);
    this.trayHost.applySnapshot(snapshot);
  }

  private cleanupAll(): void {
    this.globalShortcutHost.cleanup();
    this.shortcutHost.cleanup();
    this.menuHost.cleanup();
    this.trayHost.cleanup();
  }

  private hasBlockingConflicts(snapshot: DesktopSnapshot): boolean {
    return snapshot.conflicts?.some(
      (c) =>
        !c.resolved &&
        (c.severity === "error" || c.severity === "critical"),
    ) ?? false;
  }

  private filterSupported(snapshot: DesktopSnapshot): DesktopSnapshot {
    const supported = (snapshot.contributions ?? []).filter((c) =>
      this.isPlatformSupported(c),
    );
    const supportedIds = new Set(
      supported.map((c) => c.definition.contributionId),
    );
    const filterTree = (
      tree: Record<string, ResolvedContribution[]>,
    ): Record<string, ResolvedContribution[]> => {
      const result: Record<string, ResolvedContribution[]> = {};
      for (const [key, value] of Object.entries(tree)) {
        result[key] = value.filter((c) =>
          supportedIds.has(c.definition.contributionId),
        );
      }
      return result;
    };
    return {
      ...snapshot,
      contributions: supported,
      menuTree: filterTree(snapshot.menuTree ?? {}),
      trayTree: filterTree(snapshot.trayTree ?? {}),
      shortcuts: (snapshot.shortcuts ?? []).filter((c) =>
        supportedIds.has(c.definition.contributionId),
      ),
    };
  }

  private isPlatformSupported(contrib: ResolvedContribution): boolean {
    const platforms =
      contrib.definition.visibility?.platform ??
      contrib.definition.enabledRule?.platform;
    if (platforms && !platforms.includes(process.platform)) return false;
    return true;
  }
}
