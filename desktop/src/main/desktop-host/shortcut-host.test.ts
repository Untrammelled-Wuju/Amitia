import { EventEmitter } from "node:events";
import { describe, expect, it, vi } from "vitest";
import { ShortcutHost } from "./shortcut-host";
import type { DesktopSnapshot } from "./types";

describe("ShortcutHost", () => {
  it("只阻止精确匹配的已注册快捷键", () => {
    const webContents = new EventEmitter();
    const window = Object.assign(new EventEmitter(), {
      webContents,
      isDestroyed: () => false,
      isFocused: () => true,
    });
    const invokeAction = vi.fn();
    const host = new ShortcutHost(window as never, { invokeAction } as never);
    host.applySnapshot({ shortcuts: [{ status: "registered", definition: {
      contributionId: "test", extensionId: "test", shortcut: { accelerator: "CmdOrCtrl+K", global: false }, action: { input: {} },
    } }, { status: "registered", definition: {
      contributionId: "alias", extensionId: "test", shortcut: { accelerator: "CommandOrControl+J", global: false }, action: { input: {} },
    } }, { status: "registered", definition: {
      contributionId: "both", extensionId: "test", shortcut: { accelerator: "Command+Control+B", global: false }, action: { input: {} },
    } }] } as DesktopSnapshot);
    const dispatch = (key: string, control: boolean, meta: boolean) => {
      const preventDefault = vi.fn();
      webContents.emit("before-input-event", { preventDefault }, { type: "keyDown", key, control, meta, alt: false, shift: false });
      return preventDefault;
    };
    const control = process.platform !== "darwin";
    const meta = !control;
    expect(dispatch("m", control, meta)).not.toHaveBeenCalled();
    expect(dispatch("k", true, true)).not.toHaveBeenCalled();
    expect(dispatch("k", control, meta)).toHaveBeenCalledOnce();
    expect(dispatch("j", control, meta)).toHaveBeenCalledOnce();
    expect(dispatch("b", control, meta)).not.toHaveBeenCalled();
    expect(dispatch("b", true, true)).toHaveBeenCalledOnce();
    expect(invokeAction).toHaveBeenCalledTimes(3);
    host.cleanup();
  });
});
