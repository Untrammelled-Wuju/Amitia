import { BrowserWindow } from "electron";
import type { DesktopActionBridge } from "./action-bridge";
import type { DesktopSnapshot, ResolvedContribution } from "./types";

interface ParsedAccelerator {
  key: string;
  ctrl: boolean;
  cmd: boolean;
  alt: boolean;
  shift: boolean;
}

interface RegisteredShortcut {
  contributionId: string;
  extensionId: string;
  accelerator: string;
  parsed: ParsedAccelerator;
  input: any;
}

export class ShortcutHost {
  private readonly mainWindow: BrowserWindow;
  private readonly actionBridge: DesktopActionBridge;
  private readonly shortcuts = new Map<string, RegisteredShortcut>();
  private inputHandler:
    | ((event: Electron.Event, input: Electron.Input) => void)
    | null = null;
  private focusHandler: (() => void) | null = null;
  private blurHandler: (() => void) | null = null;

  constructor(mainWindow: BrowserWindow, actionBridge: DesktopActionBridge) {
    this.mainWindow = mainWindow;
    this.actionBridge = actionBridge;
    this.attachWindowListeners();
  }

  applySnapshot(snapshot: DesktopSnapshot): void {
    this.unregisterAll();
    for (const contrib of snapshot.shortcuts) {
      if (contrib.status !== "registered") continue;
      const shortcut = contrib.definition.shortcut;
      if (!shortcut) continue;
      if (shortcut.global) continue;
      this.register(contrib);
    }
    this.refreshAttachment();
  }

  cleanup(): void {
    this.detachInputHandler();
    this.shortcuts.clear();
    if (this.focusHandler) {
      this.mainWindow.removeListener("focus", this.focusHandler);
      this.focusHandler = null;
    }
    if (this.blurHandler) {
      this.mainWindow.removeListener("blur", this.blurHandler);
      this.blurHandler = null;
    }
  }

  private attachWindowListeners(): void {
    this.focusHandler = () => {
      if (this.shortcuts.size > 0) this.attachInputHandler();
    };
    this.blurHandler = () => {
      this.detachInputHandler();
    };
    this.mainWindow.on("focus", this.focusHandler);
    this.mainWindow.on("blur", this.blurHandler);
  }

  private register(contrib: ResolvedContribution): void {
    const shortcut = contrib.definition.shortcut;
    if (!shortcut) return;
    const accelerator = shortcut.accelerator;
    if (this.shortcuts.has(accelerator)) return;
    const parsed = this.parseAccelerator(accelerator);
    this.shortcuts.set(accelerator, {
      contributionId: contrib.definition.contributionId,
      extensionId: contrib.definition.extensionId,
      accelerator,
      parsed,
      input: contrib.definition.action.input,
    });
  }

  private unregisterAll(): void {
    const had = this.shortcuts.size > 0;
    this.shortcuts.clear();
    if (had) this.detachInputHandler();
  }

  private refreshAttachment(): void {
    if (this.shortcuts.size > 0) {
      if (!this.mainWindow.isDestroyed() && this.mainWindow.isFocused()) {
        this.attachInputHandler();
      }
    } else {
      this.detachInputHandler();
    }
  }

  private attachInputHandler(): void {
    if (this.inputHandler) return;
    if (this.mainWindow.isDestroyed()) return;
    this.inputHandler = (event, input) => {
      if (input.type !== "keyDown") return;
      for (const [, shortcut] of this.shortcuts) {
        if (this.matches(shortcut.parsed, input)) {
          event.preventDefault();
          void this.actionBridge.invokeAction({
            contributionId: shortcut.contributionId,
            extensionId: shortcut.extensionId,
            input: shortcut.input,
          });
          break;
        }
      }
    };
    this.mainWindow.webContents.on("before-input-event", this.inputHandler);
  }

  private detachInputHandler(): void {
    if (!this.inputHandler) return;
    if (!this.mainWindow.isDestroyed()) {
      this.mainWindow.webContents.removeListener(
        "before-input-event",
        this.inputHandler,
      );
    }
    this.inputHandler = null;
  }

  private matches(parsed: ParsedAccelerator, input: Electron.Input): boolean {
    const key = input.key.toLowerCase();
    if (parsed.key !== key) return false;
    const wantCmdOrCtrl = parsed.ctrl && parsed.cmd;
    if (wantCmdOrCtrl) {
      if (!input.control && !input.meta) return false;
    } else {
      if (parsed.ctrl !== input.control) return false;
      if (parsed.cmd !== input.meta) return false;
    }
    if (parsed.alt !== input.alt) return false;
    if (parsed.shift !== input.shift) return false;
    return true;
  }

  private parseAccelerator(accelerator: string): ParsedAccelerator {
    const parts = accelerator
      .split("+")
      .map((p) => p.trim().toLowerCase());
    const parsed: ParsedAccelerator = {
      key: "",
      ctrl: false,
      cmd: false,
      alt: false,
      shift: false,
    };
    for (const part of parts) {
      switch (part) {
        case "cmdorctrl":
          parsed.ctrl = true;
          parsed.cmd = true;
          break;
        case "ctrl":
        case "control":
          parsed.ctrl = true;
          break;
        case "cmd":
        case "command":
        case "commandorcontrol":
          parsed.cmd = true;
          break;
        case "alt":
        case "option":
          parsed.alt = true;
          break;
        case "shift":
          parsed.shift = true;
          break;
        default:
          parsed.key = part;
      }
    }
    return parsed;
  }
}
