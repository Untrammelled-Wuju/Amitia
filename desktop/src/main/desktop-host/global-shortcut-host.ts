import { globalShortcut } from "electron";
import type { DesktopActionBridge } from "./action-bridge";
import type { DesktopSnapshot, ResolvedContribution } from "./types";

interface RegisteredGlobalShortcut {
  contributionId: string;
  extensionId: string;
  accelerator: string;
  input: any;
}

export class GlobalShortcutHost {
  private readonly actionBridge: DesktopActionBridge;
  private readonly shortcuts = new Map<string, RegisteredGlobalShortcut>();
  private readonly contribIndex = new Map<string, string>();

  constructor(actionBridge: DesktopActionBridge) {
    this.actionBridge = actionBridge;
  }

  applySnapshot(snapshot: DesktopSnapshot): void {
    this.unregisterAll();
    for (const contrib of snapshot.shortcuts) {
      if (contrib.status !== "registered") continue;
      const shortcut = contrib.definition.shortcut;
      if (!shortcut) continue;
      if (!shortcut.global) continue;
      this.register(contrib);
    }
  }

  unregisterByContribution(contributionId: string): void {
    const accelerator = this.contribIndex.get(contributionId);
    if (!accelerator) return;
    globalShortcut.unregister(accelerator);
    this.shortcuts.delete(accelerator);
    this.contribIndex.delete(contributionId);
  }

  unregisterAll(): void {
    for (const accelerator of this.shortcuts.keys()) {
      globalShortcut.unregister(accelerator);
    }
    this.shortcuts.clear();
    this.contribIndex.clear();
  }

  cleanup(): void {
    globalShortcut.unregisterAll();
    this.shortcuts.clear();
    this.contribIndex.clear();
  }

  isRegistered(accelerator: string): boolean {
    return this.shortcuts.has(accelerator);
  }

  private register(contrib: ResolvedContribution): boolean {
    const shortcut = contrib.definition.shortcut;
    if (!shortcut) return false;
    const accelerator = shortcut.accelerator;
    if (this.shortcuts.has(accelerator)) return false;
    const ok = globalShortcut.register(accelerator, () => {
      void this.actionBridge.invokeAction({
        contributionId: contrib.definition.contributionId,
        extensionId: contrib.definition.extensionId,
        input: contrib.definition.action.input,
      });
    });
    if (!ok) return false;
    this.shortcuts.set(accelerator, {
      contributionId: contrib.definition.contributionId,
      extensionId: contrib.definition.extensionId,
      accelerator,
      input: contrib.definition.action.input,
    });
    this.contribIndex.set(contrib.definition.contributionId, accelerator);
    return true;
  }
}
