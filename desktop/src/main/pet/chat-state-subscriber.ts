import http from "node:http";
import type { ChatStateIpcPayload } from "./chat-state-bridge";

export interface ChatStateSubscriberOptions {
  coreHost?: string;
  corePort?: number;
  onPayload?: (payload: ChatStateIpcPayload) => void;
  pollIntervalMs?: number;
}

const DEFAULT_CORE_HOST = "127.0.0.1";
const DEFAULT_CORE_PORT = 18899;
const DEFAULT_POLL_INTERVAL_MS = 0;

export class ChatStateSubscriber {
  private readonly coreHost: string;
  private readonly corePort: number;
  private readonly onPayload?: (payload: ChatStateIpcPayload) => void;
  private readonly pollIntervalMs: number;
  private stopped = true;

  constructor(options: ChatStateSubscriberOptions = {}) {
    this.coreHost = options.coreHost ?? DEFAULT_CORE_HOST;
    this.corePort = options.corePort ?? DEFAULT_CORE_PORT;
    this.onPayload = options.onPayload;
    this.pollIntervalMs =
      options.pollIntervalMs ?? DEFAULT_POLL_INTERVAL_MS;
  }

  async start(): Promise<void> {
    if (!this.onPayload || this.pollIntervalMs <= 0) {
      this.stopped = false;
      return;
    }
    this.stopped = false;
    void this.pollLoop();
  }

  stop(): void {
    this.stopped = true;
  }

  private async pollLoop(): Promise<void> {
    while (!this.stopped) {
      try {
        await this.fetchOnce();
      } catch (err) {
        console.warn(
          "[ChatStateSubscriber] 拉取聊天状态失败:",
          err instanceof Error ? err.message : String(err),
        );
      }
      await new Promise((resolve) =>
        setTimeout(resolve, this.pollIntervalMs),
      );
    }
  }

  private fetchOnce(): Promise<void> {
    return new Promise((resolve, reject) => {
      const req = http.request(
        {
          hostname: this.coreHost,
          port: this.corePort,
          path: "/api/desktop-pets/chat-state",
          method: "GET",
          timeout: 2000,
        },
        (res) => {
          res.resume();
          if (res.statusCode !== 200) {
            resolve();
            return;
          }
          let data = "";
          res.setEncoding("utf8");
          res.on("data", (chunk) => {
            data += chunk;
          });
          res.on("end", () => {
            if (!data) {
              resolve();
              return;
            }
            try {
              const parsed = JSON.parse(data) as ChatStateIpcPayload;
              if (parsed && typeof parsed.state === "string") {
                this.onPayload?.(parsed);
              }
            } catch {
              // ignore parse error
            }
            resolve();
          });
        },
      );
      req.on("error", (err) => reject(err));
      req.on("timeout", () => {
        req.destroy();
        resolve();
      });
      req.end();
    });
  }
}
