export interface CharacterWatcherOptions {
  coreBaseURL?: string;
  pollIntervalMs?: number;
  requestTimeoutMs?: number;
  authHeadersProvider?: () =>
    | Record<string, string>
    | Promise<Record<string, string>>;
  onActiveCharacterChanged?: (characterId: string) => void | Promise<void>;
}

interface ApiEnvelope<T> {
  code?: number;
  msg?: string;
  data?: T;
}

interface RoleProfilePayload {
  id?: string;
  characterID?: string;
  characterId?: string;
}

const DEFAULT_CORE_BASE_URL = "http://127.0.0.1:18899";
const DEFAULT_POLL_INTERVAL_MS = 5000;
const DEFAULT_REQUEST_TIMEOUT_MS = 3000;

export class CharacterWatcher {
  private readonly coreBaseURL: string;
  private readonly pollIntervalMs: number;
  private readonly requestTimeoutMs: number;
  private readonly authHeadersProvider?: CharacterWatcherOptions["authHeadersProvider"];
  private readonly onActiveCharacterChanged?:
    | ((characterId: string) => void | Promise<void>);
  private lastCharacterId: string | null = null;
  private stopped = true;
  private timer: NodeJS.Timeout | null = null;
  private tickInFlight = false;

  constructor(options: CharacterWatcherOptions = {}) {
    this.coreBaseURL = (options.coreBaseURL ?? DEFAULT_CORE_BASE_URL).replace(
      /\/+$/,
      "",
    );
    this.pollIntervalMs = options.pollIntervalMs ?? DEFAULT_POLL_INTERVAL_MS;
    this.requestTimeoutMs =
      options.requestTimeoutMs ?? DEFAULT_REQUEST_TIMEOUT_MS;
    this.authHeadersProvider = options.authHeadersProvider;
    this.onActiveCharacterChanged = options.onActiveCharacterChanged;
  }

  async start(): Promise<void> {
    if (!this.onActiveCharacterChanged || this.pollIntervalMs <= 0) {
      this.stopped = false;
      return;
    }
    this.stopped = false;

    // Initial reconciliation is deliberate. The active character can change while
    // Electron is stopped, so merely remembering the first observed ID would leave
    // a stale desktop pet running until the next user-initiated character switch.
    // A transient auth/network failure must not disable the watcher permanently; the
    // interval below remains armed so reconciliation is retried automatically.
    try {
      await this.tick();
    } catch (err) {
      console.warn(
        "[CharacterWatcher] 首次角色同步失败，将继续重试:",
        err instanceof Error ? err.message : String(err),
      );
    }

    if (this.stopped) return;
    this.timer = setInterval(() => {
      void this.tick().catch((err) => {
        console.warn(
          "[CharacterWatcher] 轮询失败:",
          err instanceof Error ? err.message : String(err),
        );
      });
    }, this.pollIntervalMs);
    this.timer.unref?.();
  }

  stop(): void {
    this.stopped = true;
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }

  private async tick(): Promise<void> {
    if (this.stopped || this.tickInFlight) return;
    this.tickInFlight = true;
    try {
      const characterId = await this.fetchActiveCharacterId();
      if (!characterId || this.lastCharacterId === characterId) return;

      // Commit the observed ID only after the local pet has reconciled
      // successfully. A failed switch is retried on the next poll instead of being
      // silently acknowledged forever.
      await this.onActiveCharacterChanged?.(characterId);
      this.lastCharacterId = characterId;
    } finally {
      this.tickInFlight = false;
    }
  }

  private async fetchActiveCharacterId(): Promise<string> {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), this.requestTimeoutMs);
    timeout.unref?.();

    try {
      const headers = this.authHeadersProvider
        ? await this.authHeadersProvider()
        : {};
      const response = await fetch(
        `${this.coreBaseURL}/api/companion/role-profile`,
        {
          method: "GET",
          headers: {
            Accept: "application/json",
            ...headers,
          },
          signal: controller.signal,
        },
      );
      if (!response.ok) {
        throw new Error(`role profile HTTP ${response.status}`);
      }

      const parsed = (await response.json()) as
        | ApiEnvelope<RoleProfilePayload>
        | RoleProfilePayload;
      if (
        parsed &&
        typeof parsed === "object" &&
        "code" in parsed &&
        typeof parsed.code === "number"
      ) {
        if (parsed.code < 200 || parsed.code >= 300 || !parsed.data) {
          throw new Error(
            `role profile API ${parsed.code}${parsed.msg ? `: ${parsed.msg}` : ""}`,
          );
        }
        return this.extractCharacterId(parsed.data);
      }
      return this.extractCharacterId(parsed as RoleProfilePayload);
    } finally {
      clearTimeout(timeout);
    }
  }

  private extractCharacterId(payload: RoleProfilePayload | null | undefined): string {
    if (!payload) return "";
    const value = payload.characterId ?? payload.characterID ?? payload.id ?? "";
    return typeof value === "string" ? value.trim() : "";
  }
}
