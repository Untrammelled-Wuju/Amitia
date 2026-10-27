import { expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  handlers: new Map<string, (...args: any[]) => any>(),
  events: new Map<string, (...args: any[]) => any>(),
  install: vi.fn(),
  send: vi.fn(),
}));

vi.mock("electron", () => ({
  ipcMain: { handle: (name: string, handler: (...args: any[]) => any) => mocks.handlers.set(name, handler) },
  app: { getVersion: () => "1.0.0" },
  shell: { openExternal: vi.fn() },
}));
vi.mock("electron-updater", () => ({
  autoUpdater: { on: (name: string, handler: (...args: any[]) => any) => mocks.events.set(name, handler), quitAndInstall: mocks.install },
}));
vi.mock("electron-log", () => ({ default: { transports: { file: {} } } }));
vi.mock("./path-manager", () => ({ getAmitiaDataDir: () => "/tmp/amitia-update-test", getInstallDir: () => "/tmp", isDevMode: () => true }));
vi.mock("./segmented-updater", () => ({ installSegmentedFullDownload: vi.fn() }));

import { registerUpdateManager } from "./update-manager";

it("停止失败时禁止安装，随后成功关闭时允许安装", async () => {
  const prepare = vi.fn().mockRejectedValueOnce(new Error("插件仍在运行")).mockResolvedValueOnce(undefined);
  registerUpdateManager({ webContents: { send: mocks.send } } as any, { prepareForInstall: prepare });
  mocks.events.get("update-downloaded")!({ version: "1.1.0" });
  await mocks.handlers.get("update:restart-now")!();
  expect(mocks.install).not.toHaveBeenCalled();
  expect(mocks.send).toHaveBeenCalledWith("update:error", { message: "插件仍在运行" });
  await mocks.handlers.get("update:restart-now")!();
  expect(mocks.install).toHaveBeenCalledOnce();
});
