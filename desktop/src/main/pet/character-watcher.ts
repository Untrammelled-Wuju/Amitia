import http from "node:http";

export interface CharacterWatcherOptions {
  coreHost?: string;
  corePort?: number;
  pollIntervalMs?: number;
  onActiveCharacterChanged?: (characterId: string) => void | Promise<void>;
}

const DEFAULT_CORE_HOST = "127.0.0.1";
const DEFAULT_CORE_PORT = 18899;
const DEFAULT_POLL_INTERVAL_MS = 5000;

export class CharacterWatcher {
  private readonly coreHost: string;
  private readonly corePort: number;
  private readonly pollIntervalMs: number;
  private readonly onActiveCharacterChanged?:
    | ((characterId: string) => void | Promise<void>);
  private lastCharacterId: string | null = null;
  private stopped = true;
  private timer: NodeJS.Timeout | null = null;

  constructor(options: CharacterWatcherOptions = {}) {
    this.coreHost = options.coreHost ?? DEFAULT_CORE_HOST;
    this.corePort = options.corePort ?? DEFAULT_CORE_PORT;
    this.pollIntervalMs =
      options.pollIntervalMs ?? DEFAULT_POLL_INTERVAL_MS;
    this.onActiveCharacterChanged = options.onActiveCharacterChanged;
  }

  async start(): Promise<void> {
    if (!this.onActiveCharacterChanged || this.pollIntervalMs <= 0) {
      this.stopped = false;
      return;
    }
    this.stopped = false;
    await this.tick();
    this.timer = setInterval(() => {
      void this.tick().catch((err) => {
        console.warn(
          "[CharacterWatcher] 轮询失败:",
          err instanceof Error ? err.message : String(err),
        );
      });
    }, this.pollIntervalMs);
  }

  stop(): void {
    this.stopped = true;
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }

  private async tick(): Promise<void> {
    if (this.stopped) return;
    const characterId = await this.fetchActiveCharacterId();
    if (!characterId) return;
    if (this.lastCharacterId === characterId) return;
    const previous = this.lastCharacterId;
    this.lastCharacterId = characterId;
    if (!previous) return;
    try {
      await this.onActiveCharacterChanged?.(characterId);
    } catch (err) {
      console.warn(
        "[CharacterWatcher] 角色切换回调失败:",
        err instanceof Error ? err.message : String(err),
      );
    }
  }

  private fetchActiveCharacterId(): Promise<string> {
    return new Promise((resolve) => {
      const req = http.request(
        {
          hostname: this.coreHost,
          port: this.corePort,
          path: "/api/companion/role-profile",
          method: "GET",
          timeout: 2000,
        },
        (res) => {
          res.resume();
          if (res.statusCode !== 200) {
            resolve("");
            return;
          }
          let data = "";
          res.setEncoding("utf8");
          res.on("data", (chunk) => {
            data += chunk;
          });
          res.on("end", () => {
            if (!data) {
              resolve("");
              return;
            }
            try {
              const parsed = JSON.parse(data) as
                | { data?: { characterID?: string; characterId?: string } }
                | { characterID?: string; characterId?: string };
              const id =
                (parsed as { data?: { characterID?: string; characterId?: string } })
                  .data?.characterID ??
                (parsed as { data?: { characterID?: string; characterId?: string } })
                  .data?.characterId ??
                (parsed as { characterID?: string; characterId?: string })
                  .characterID ??
                (parsed as { characterID?: string; characterId?: string })
                  .characterId ??
                "";
              resolve(typeof id === "string" ? id : "");
            } catch {
              resolve("");
            }
          });
        },
      );
      req.on("error", () => resolve(""));
      req.on("timeout", () => {
        req.destroy();
        resolve("");
      });
      req.end();
    });
  }
}
