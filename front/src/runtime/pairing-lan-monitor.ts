import { createAuthenticatedFetchInit } from "@/runtime/request-auth";
import { resolveApiUrl } from "@/runtime/runtime-adapter";
import { consumeSSEBody } from "@/runtime/sse-stream";

export function startPairingLANMonitor(controller: AbortController, onChange: () => void, onFailure: (error: unknown) => void): Promise<void> {
  return new Promise((resolve, reject) => {
    let ready = false;
    let watchdog: ReturnType<typeof setTimeout> | undefined;
    const resetWatchdog = () => {
      if (watchdog) clearTimeout(watchdog);
      watchdog = setTimeout(() => {
        const error = new Error("局域网地址事件流心跳超时");
        onFailure(error);
        controller.abort(error);
      }, 45000);
    };
    const onAbort = () => {
      if (!ready) reject(controller.signal.reason);
    };
    controller.signal.addEventListener("abort", onAbort, { once: true });
    const timer = setTimeout(() => {
      const error = new Error("等待局域网地址事件流超时");
      if (!ready) reject(error);
      controller.abort(error);
    }, 10000);
    void (async () => {
      try {
        const path = "/internal/device-mesh/lan/events";
        const response = await fetch(await resolveApiUrl(path), await createAuthenticatedFetchInit(path, {
          headers: { Accept: "text/event-stream", "Cache-Control": "no-store" },
          signal: controller.signal,
        }));
        if (!response.ok || !response.body || !response.headers.get("content-type")?.includes("text/event-stream")) {
          throw new Error(`局域网地址事件流连接失败: HTTP ${response.status} ${await response.text()}`);
        }
        await consumeSSEBody(response.body, controller.signal, (event) => {
          if (event.event === "lan_heartbeat") {
            if (ready) resetWatchdog();
            return;
          }
          if (event.event !== "lan_endpoints") return;
          const data = JSON.parse(event.data) as { revision: unknown };
          if (!Number.isSafeInteger(data.revision) || Number(data.revision) < 0) throw new Error(`局域网地址事件无效: ${event.data}`);
          resetWatchdog();
          if (!ready) {
            ready = true;
            clearTimeout(timer);
            controller.signal.removeEventListener("abort", onAbort);
            resolve();
          } else {
            onChange();
          }
        });
        if (!controller.signal.aborted) throw new Error("局域网地址事件流意外结束");
      } catch (error) {
        if (!controller.signal.aborted) {
          if (ready) onFailure(error);
          else reject(error);
        }
      } finally {
        clearTimeout(timer);
        if (watchdog) clearTimeout(watchdog);
        controller.signal.removeEventListener("abort", onAbort);
      }
    })();
  });
}
