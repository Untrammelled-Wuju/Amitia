import { afterEach, expect, it, vi } from "vitest";

const alert = vi.hoisted(() => vi.fn().mockResolvedValue(undefined));
vi.mock("element-plus", () => ({ ElMessageBox: { alert } }));
import { copyText } from "@/conversation/rendering/utils";

afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); alert.mockClear(); });

it("桌面复制使用原生桥接", async () => {
  const writeClipboardText = vi.fn().mockResolvedValue(undefined);
  vi.stubGlobal("amitiaDesktop", { writeClipboardText });
  expect(await copyText("消息内容")).toBe(true);
  expect(writeClipboardText).toHaveBeenCalledWith("消息内容");
});

it("复制失败时展示原始错误", async () => {
  const error = new Error("clipboard failure");
  vi.stubGlobal("amitiaDesktop", { writeClipboardText: vi.fn().mockRejectedValue(error) });
  vi.spyOn(console, "error").mockImplementation(() => {});
  expect(await copyText("消息内容")).toBe(false);
  expect(alert.mock.calls[0]?.[0]).toContain("clipboard failure");
  expect(alert.mock.calls[0]?.[0]).toContain("stack");
});

it("浏览器复制使用浏览器剪贴板", async () => {
  vi.stubGlobal("amitiaDesktop", undefined);
  const writeText = vi.fn().mockResolvedValue(undefined);
  Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
  expect(await copyText("网页消息")).toBe(true);
  expect(writeText).toHaveBeenCalledWith("网页消息");
});
