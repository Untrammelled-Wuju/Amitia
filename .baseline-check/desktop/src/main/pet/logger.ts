import { createWriteStream, mkdirSync, existsSync } from "node:fs";
import { join } from "node:path";
import { getAmitiaDataDir } from "../path-manager";

export type PetLogEventType =
  | "enable"
  | "disable"
  | "install_failed"
  | "corruption"
  | "action_switch"
  | "action_fallback"
  | "action_load_failed"
  | "window_recovery"
  | "drag_start"
  | "drag_end"
  | "runtime_crash";

const AGGREGATE_WINDOW_MS = 1000;
const MAX_MESSAGE_LENGTH = 1024;
const MAX_META_KEYS = 12;
const MAX_ARRAY_ITEMS = 8;

const SENSITIVE_KEY_PATTERNS: RegExp[] = [
  /token/i,
  /api[-_]?key/i,
  /secret/i,
  /password/i,
  /credential/i,
  /authorization/i,
  /bearer/i,
  /cookie/i,
  /session/i,
];

const BASE64_DATA_URL_PATTERN = /data:image\/[a-zA-Z0-9.+-]*;base64,[A-Za-z0-9+/=]+/g;
const LONG_BASE64_PATTERN = /[A-Za-z0-9+/]{200,}={0,2}/g;
const BEARER_TOKEN_PATTERN = /Bearer\s+[A-Za-z0-9._\-]+/gi;
const SK_TOKEN_PATTERN = /\bsk-[A-Za-z0-9]{16,}/g;
const JWT_PATTERN = /\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b/g;

interface AggregateEntry {
  count: number;
  firstAt: number;
  lastAt: number;
  lastMessage: string;
  lastMeta?: Record<string, unknown>;
}

function sanitize(value: unknown): unknown {
  if (value === null || value === undefined) return value;
  if (typeof value === "string") return sanitizeString(value);
  if (typeof value === "number" || typeof value === "boolean") return value;
  if (value instanceof Error) {
    return sanitizeString(`${value.name}: ${value.message}`);
  }
  if (Array.isArray(value)) {
    const trimmed = value.slice(0, MAX_ARRAY_ITEMS);
    return trimmed.map(sanitize);
  }
  if (typeof value === "object") {
    const result: Record<string, unknown> = {};
    let count = 0;
    for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
      if (count >= MAX_META_KEYS) break;
      if (SENSITIVE_KEY_PATTERNS.some((p) => p.test(k))) {
        result[k] = "[REDACTED]";
      } else {
        result[k] = sanitize(v);
      }
      count += 1;
    }
    return result;
  }
  return sanitizeString(String(value));
}

function sanitizeString(raw: string): string {
  let s = String(raw);
  if (s.length > MAX_MESSAGE_LENGTH) {
    s = s.slice(0, MAX_MESSAGE_LENGTH) + "...[truncated]";
  }
  s = s.replace(BASE64_DATA_URL_PATTERN, "[BASE64_IMAGE]");
  s = s.replace(BEARER_TOKEN_PATTERN, "Bearer [REDACTED]");
  s = s.replace(SK_TOKEN_PATTERN, "sk-[REDACTED]");
  s = s.replace(JWT_PATTERN, "[JWT_REDACTED]");
  s = s.replace(LONG_BASE64_PATTERN, "[BASE64_BLOCK]");
  return s;
}

function formatMeta(meta: Record<string, unknown> | undefined): string {
  if (!meta) return "";
  const parts: string[] = [];
  for (const [k, v] of Object.entries(meta)) {
    const value = typeof v === "string" ? v : JSON.stringify(v);
    parts.push(`${k}=${value}`);
  }
  return parts.length > 0 ? parts.join(" ") : "";
}

export class PetLogger {
  private stream: ReturnType<typeof createWriteStream> | null = null;
  private readonly aggregates: Map<string, AggregateEntry> = new Map();
  private readonly aggregateTimers: Map<string, ReturnType<typeof setTimeout>> =
    new Map();
  private static instance: PetLogger | null = null;
  private disposed = false;

  constructor(logDir?: string) {
    this.ensureStream(logDir);
  }

  static getInstance(): PetLogger {
    if (!PetLogger.instance) {
      PetLogger.instance = new PetLogger();
    }
    return PetLogger.instance;
  }

  static setInstance(instance: PetLogger): void {
    PetLogger.instance = instance;
  }

  private ensureStream(logDir?: string): void {
    try {
      const dir = logDir ?? join(getAmitiaDataDir(), "logs");
      if (!existsSync(dir)) {
        mkdirSync(dir, { recursive: true });
      }
      const path = join(dir, "pet.log");
      this.stream = createWriteStream(path, { flags: "a" });
    } catch (err) {
      console.warn("[PetLogger] 初始化失败, 回退到 console:", err);
      this.stream = null;
    }
  }

  log(
    type: PetLogEventType,
    message: string,
    meta?: Record<string, unknown>,
  ): void {
    if (this.disposed) return;
    const sanitized = sanitizeString(message);
    const sanitizedMeta = meta
      ? (sanitize(meta) as Record<string, unknown>)
      : undefined;
    if (type === "action_switch" || type === "action_fallback") {
      this.aggregate(type, sanitized, sanitizedMeta);
      return;
    }
    this.write(type, sanitized, sanitizedMeta);
  }

  logEnable(installationId: string, name: string): void {
    this.log("enable", `启用桌宠 name=${name}`, { installationId, name });
  }

  logDisable(installationId: string, reason?: string): void {
    this.log("disable", `停用桌宠 reason=${reason ?? "user"}`, {
      installationId,
    });
  }

  logInstallFailed(installationId: string, error: string): void {
    this.log("install_failed", `安装失败 error=${error}`, { installationId });
  }

  logCorruption(
    installationId: string,
    errorCode: string,
    detail: string,
  ): void {
    this.log(
      "corruption",
      `资源损坏 code=${errorCode} detail=${detail}`,
      { installationId, errorCode },
    );
  }

  logActionSwitch(
    newKey: string,
    oldKey: string | null,
    source: string,
  ): void {
    this.log(
      "action_switch",
      `动作切换 new=${newKey} old=${oldKey ?? "null"} source=${source}`,
      { newKey, oldKey: oldKey ?? "", source },
    );
  }

  logActionFallback(
    requestedKey: string,
    fallbackKey: string | null,
    source: string,
  ): void {
    this.log(
      "action_fallback",
      `动作回退 requested=${requestedKey} fallback=${fallbackKey ?? "null"} source=${source}`,
      { requestedKey, fallbackKey: fallbackKey ?? "", source },
    );
  }

  logActionLoadFailed(actionKey: string, error: string): void {
    this.log(
      "action_load_failed",
      `动作加载失败 key=${actionKey} error=${error}`,
      { actionKey },
    );
  }

  logWindowRecovered(reason: string, installationId?: string): void {
    this.log("window_recovery", `窗口恢复 reason=${reason}`, {
      installationId: installationId ?? "",
      reason,
    });
  }

  logDragStart(installationId?: string): void {
    this.log("drag_start", "拖动开始", {
      installationId: installationId ?? "",
    });
  }

  logDragEnd(installationId?: string): void {
    this.log("drag_end", "拖动结束", {
      installationId: installationId ?? "",
    });
  }

  logRuntimeCrash(context: string, error: unknown): void {
    const msg =
      error instanceof Error
        ? `${error.name}: ${error.message}`
        : String(error);
    this.log("runtime_crash", `运行时崩溃 context=${context} error=${msg}`, {
      context,
    });
  }

  dispose(): void {
    this.disposed = true;
    for (const timer of this.aggregateTimers.values()) {
      clearTimeout(timer);
    }
    this.aggregateTimers.clear();
    this.aggregates.clear();
    if (this.stream) {
      try {
        this.stream.end();
      } catch {
        void 0;
      }
      this.stream = null;
    }
  }

  private aggregate(
    type: PetLogEventType,
    message: string,
    meta: Record<string, unknown> | undefined,
  ): void {
    const now = Date.now();
    const key = type;
    const existing = this.aggregates.get(key);
    if (existing) {
      existing.count += 1;
      existing.lastAt = now;
      existing.lastMessage = message;
      existing.lastMeta = meta;
      return;
    }
    this.aggregates.set(key, {
      count: 1,
      firstAt: now,
      lastAt: now,
      lastMessage: message,
      lastMeta: meta,
    });
    const timer = setTimeout(() => this.flushAggregate(key), AGGREGATE_WINDOW_MS);
    if (typeof timer.unref === "function") {
      timer.unref();
    }
    this.aggregateTimers.set(key, timer);
  }

  private flushAggregate(key: string): void {
    const entry = this.aggregates.get(key);
    if (!entry) return;
    this.aggregates.delete(key);
    this.aggregateTimers.delete(key);
    if (entry.count <= 1) {
      this.write(key as PetLogEventType, entry.lastMessage, entry.lastMeta);
      return;
    }
    const message = `${entry.lastMessage} (聚合 ${entry.count} 次, 跨度 ${entry.lastAt - entry.firstAt}ms)`;
    this.write(key as PetLogEventType, message, entry.lastMeta);
  }

  private write(
    type: PetLogEventType,
    message: string,
    meta: Record<string, unknown> | undefined,
  ): void {
    const ts = new Date().toISOString();
    const metaStr = formatMeta(meta);
    const line = `[${ts}] [${type}] ${message}${metaStr ? ` ${metaStr}` : ""}\n`;
    if (this.stream) {
      try {
        this.stream.write(line);
      } catch {
        void 0;
      }
    }
    try {
      console.log(`[Pet] ${line.trimEnd()}`);
    } catch {
      void 0;
    }
  }
}
