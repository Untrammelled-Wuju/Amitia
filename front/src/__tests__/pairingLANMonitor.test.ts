import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/runtime/request-auth", () => ({ createAuthenticatedFetchInit: async (_path: string, init: RequestInit) => init }));
vi.mock("@/runtime/runtime-adapter", () => ({ resolveApiUrl: async (path: string) => path }));
import { startPairingLANMonitor } from "@/runtime/pairing-lan-monitor";

afterEach(() => vi.unstubAllGlobals());

describe("pairing LAN monitor", () => {
  it("在首个快照后就绪，并在地址代次变化时通知", async () => {
    let stream!: ReadableStreamDefaultController<Uint8Array>;
    const body = new ReadableStream<Uint8Array>({ start(controller) { stream = controller; } });
    vi.stubGlobal("fetch", vi.fn(async () => new Response(body, { headers: { "content-type": "text/event-stream" } })));
    const controller = new AbortController();
    const changed = vi.fn();
    const failed = vi.fn();
    const ready = startPairingLANMonitor(controller, changed, failed);
    const encoder = new TextEncoder();
    stream.enqueue(encoder.encode('event: lan_endpoints\ndata: {"revision":1}\n\n'));
    await ready;
    stream.enqueue(encoder.encode('event: lan_endpoints\ndata: {"revision":2}\n\n'));
    await vi.waitFor(() => expect(changed).toHaveBeenCalledOnce());
    controller.abort();
    stream.close();
    expect(failed).not.toHaveBeenCalled();
  });

  it("事件流断开时传递原始错误", async () => {
    let stream!: ReadableStreamDefaultController<Uint8Array>;
    const body = new ReadableStream<Uint8Array>({ start(controller) { stream = controller; } });
    vi.stubGlobal("fetch", vi.fn(async () => new Response(body, { headers: { "content-type": "text/event-stream" } })));
    const failure = new Error("原始网络错误");
    const failed = vi.fn();
    const ready = startPairingLANMonitor(new AbortController(), vi.fn(), failed);
    stream.enqueue(new TextEncoder().encode('event: lan_endpoints\ndata: {"revision":1}\n\n'));
    await ready;
    stream.error(failure);
    await vi.waitFor(() => expect(failed).toHaveBeenCalledWith(failure));
  });

  it("事件流心跳超时会中止监测并报告错误", async () => {
    vi.useFakeTimers();
    let stream!: ReadableStreamDefaultController<Uint8Array>;
    const body = new ReadableStream<Uint8Array>({ start(controller) { stream = controller; } });
    vi.stubGlobal("fetch", vi.fn(async () => new Response(body, { headers: { "content-type": "text/event-stream" } })));
    const controller = new AbortController();
    const failed = vi.fn();
    const ready = startPairingLANMonitor(controller, vi.fn(), failed);
    stream.enqueue(new TextEncoder().encode('event: lan_endpoints\ndata: {"revision":1}\n\n'));
    await ready;
    await vi.advanceTimersByTimeAsync(30000);
    stream.enqueue(new TextEncoder().encode('event: lan_heartbeat\ndata: {}\n\n'));
    await vi.advanceTimersByTimeAsync(0);
    await vi.advanceTimersByTimeAsync(44000);
    expect(failed).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1000);
    expect(failed.mock.calls[0][0]).toEqual(new Error("局域网地址事件流心跳超时"));
    expect(controller.signal.aborted).toBe(true);
    stream.close();
    vi.useRealTimers();
  });
});
