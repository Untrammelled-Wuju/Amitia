import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

describe("application bootstrap", () => {
  beforeEach(() => {
    vi.resetModules();
    vi.useFakeTimers();
    vi.spyOn(console, "error").mockImplementation(() => undefined);
    document.body.innerHTML = '<div id="app"></div>';
  });

  afterEach(() => {
    window.dispatchEvent(new Event("amitia:mounted"));
    vi.useRealTimers();
    vi.restoreAllMocks();
    vi.doUnmock("../main");
  });

  it("shows a recoverable error when an application dependency cannot be parsed", async () => {
    vi.doMock("../main", () => {
      throw new SyntaxError("Unexpected reserved word");
    });
    await import("../bootstrap");
    await vi.dynamicImportSettled();
    expect(document.querySelector('[role="alert"]')?.textContent).toContain("主界面启动失败");
    expect(document.querySelector("button")?.textContent).toBe("重新加载");
  });

  it("shows startup failures from asynchronous initialization", async () => {
    vi.doMock("../main", () => ({ applicationReady: Promise.reject(new Error("连接失败")) }));
    await import("../bootstrap");
    await vi.dynamicImportSettled();
    expect(document.querySelector('[role="alert"]')?.textContent).toContain("连接失败");
  });

  it("shows a timeout when initialization never finishes", async () => {
    vi.doMock("../main", () => ({ applicationReady: new Promise(() => undefined) }));
    await import("../bootstrap");
    await vi.advanceTimersByTimeAsync(30000);
    expect(document.querySelector('[role="alert"]')?.textContent).toContain("启动超时");
  });

  it("does not replace the mounted application after the timeout", async () => {
    vi.doMock("../main", () => ({ applicationReady: Promise.resolve() }));
    await import("../bootstrap");
    await vi.dynamicImportSettled();
    document.getElementById("app")!.textContent = "已启动";
    await vi.advanceTimersByTimeAsync(30000);
    expect(document.getElementById("app")?.textContent).toBe("已启动");
    expect(document.querySelector('[role="alert"]')).toBeNull();
  });
});
