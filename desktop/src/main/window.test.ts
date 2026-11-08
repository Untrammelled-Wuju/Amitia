import { EventEmitter } from "node:events";
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { expect, it, vi } from "vitest";

vi.mock("electron", () => ({ protocol: { registerSchemesAsPrivileged: vi.fn() } }));
vi.mock("./path-manager", () => ({
  isDevMode: () => true,
  getInstallDir: () => "/isolated/desktop",
}));
vi.mock("./branding", () => ({}));
vi.mock("./external-web-ui", () => ({}));

it("开发渲染器失败后使用本地错误页并保留原始错误链", async () => {
  const server = createServer((_request, response) => {
    response.statusCode = 200;
    response.end("ok");
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address() as AddressInfo;
  vi.stubEnv("VITE_DEV_SERVER_URL", `http://127.0.0.1:${address.port}`);
  try {
    const { revealMainWindow } = await import("./window");
    const original = new Error("renderer failed", { cause: new Error("original connection error") });
    const win = Object.assign(new EventEmitter(), {
      isDestroyed: () => false,
      loadURL: vi.fn().mockRejectedValue(original),
      loadFile: vi.fn().mockResolvedValue(undefined),
      webContents: { executeJavaScript: vi.fn().mockResolvedValue(undefined), stop: vi.fn() },
      show: vi.fn(),
      focus: vi.fn(),
    });
    const revealed = revealMainWindow(win as never);
    win.emit("ready-to-show");
    await revealed;
    expect(win.loadFile).toHaveBeenCalledWith(expect.stringMatching(/src[/\\]renderer[/\\]core-error\.html$/));
    const [script] = win.webContents.executeJavaScript.mock.calls[0];
    expect(script).toContain("renderer failed");
    expect(script).toContain("original connection error");
    expect(win.show).toHaveBeenCalledOnce();
  } finally {
    vi.unstubAllEnvs();
    await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  }
});
