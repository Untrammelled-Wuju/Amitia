import fs from "node:fs";
import path from "node:path";

export const LOG_MAX_FILE_BYTES = 50 * 1024 * 1024;
export const LOG_RETENTION_MS = 30 * 24 * 60 * 60 * 1000;

const MANAGED_LOG_PATTERN =
  /^[a-z0-9][a-z0-9-]*\.(\d{8})\.(\d{3})\.log(?:\.\d+)?$/i;

export interface RotatingLogOptions {
  maxFileBytes?: number;
  retentionMs?: number;
  now?: () => Date;
}

function pad(value: number, width: number): string {
  return String(value).padStart(width, "0");
}

export function logDateKey(date: Date): string {
  return `${date.getFullYear()}${pad(date.getMonth() + 1, 2)}${pad(date.getDate(), 2)}`;
}

function managedLogName(prefix: string, dateKey: string, sequence: number): string {
  return `${prefix}.${dateKey}.${pad(sequence, 3)}.log`;
}

export function isManagedLogName(name: string): boolean {
  return MANAGED_LOG_PATTERN.test(name);
}

function isRetainedLogName(name: string): boolean {
  const lower = name.toLowerCase();
  if (lower.endsWith(".log")) return true;
  const match = lower.match(/\.log\.(\d+)$/);
  return match !== null;
}

export function cleanupExpiredLogs(
  logDir: string,
  retentionMs = LOG_RETENTION_MS,
  now = new Date(),
): string[] {
  if (!fs.existsSync(logDir)) return [];
  const cutoff = now.getTime() - retentionMs;
  const removed: string[] = [];
  for (const entry of fs.readdirSync(logDir, { withFileTypes: true })) {
    if (!entry.isFile() || !isRetainedLogName(entry.name)) continue;
    const target = path.join(logDir, entry.name);
    const stat = fs.statSync(target);
    if (stat.mtimeMs >= cutoff) continue;
    fs.unlinkSync(target);
    removed.push(target);
  }
  return removed;
}

export class DailySizeRotatingLog {
  private readonly logDir: string;
  private readonly prefix: string;
  private readonly maxFileBytes: number;
  private readonly retentionMs: number;
  private readonly now: () => Date;
  private fd: number | null = null;
  private currentPath = "";
  private currentDateKey = "";
  private currentSequence = 0;
  private currentSize = 0;

  constructor(
    logDir: string,
    prefix: string,
    options: RotatingLogOptions = {},
  ) {
    this.logDir = logDir;
    this.prefix = prefix;
    this.maxFileBytes = options.maxFileBytes ?? LOG_MAX_FILE_BYTES;
    this.retentionMs = options.retentionMs ?? LOG_RETENTION_MS;
    this.now = options.now ?? (() => new Date());
    fs.mkdirSync(this.logDir, { recursive: true });
    cleanupExpiredLogs(this.logDir, this.retentionMs, this.now());
    this.openForDate(this.now(), false);
  }

  write(value: string): void {
    const buffer = Buffer.from(value, "utf8");
    if (buffer.length === 0) return;
    const dateKey = logDateKey(this.now());
    const sizeRotation =
      this.currentSize > 0 &&
      this.currentSize + buffer.length > this.maxFileBytes;
    if (
      this.fd === null ||
      dateKey !== this.currentDateKey ||
      sizeRotation
    ) {
      this.openForDate(this.now(), sizeRotation && dateKey === this.currentDateKey);
    }
    if (this.fd === null) {
      throw new Error("rotating log is not open");
    }
    let offset = 0;
    while (offset < buffer.length) {
      const written = fs.writeSync(
        this.fd,
        buffer,
        offset,
        buffer.length - offset,
      );
      if (written <= 0) {
        throw new Error(`rotating log write returned ${written}`);
      }
      offset += written;
    }
    this.currentSize += buffer.length;
  }

  close(): void {
    if (this.fd === null) return;
    fs.closeSync(this.fd);
    this.fd = null;
    this.currentPath = "";
    this.currentDateKey = "";
    this.currentSequence = 0;
    this.currentSize = 0;
  }

  get path(): string {
    return this.currentPath;
  }

  private openForDate(date: Date, advanceSequence: boolean): void {
    const dateKey = logDateKey(date);
    if (this.fd !== null) {
      fs.closeSync(this.fd);
      this.fd = null;
    }
    let sequence = advanceSequence
      ? this.currentSequence + 1
      : this.nextSequence(dateKey);
    let target = path.join(
      this.logDir,
      managedLogName(this.prefix, dateKey, sequence),
    );
    while (fs.existsSync(target) && fs.statSync(target).size >= this.maxFileBytes) {
      sequence += 1;
      target = path.join(
        this.logDir,
        managedLogName(this.prefix, dateKey, sequence),
      );
    }
    this.fd = fs.openSync(target, "a");
    this.currentPath = target;
    this.currentDateKey = dateKey;
    this.currentSequence = sequence;
    this.currentSize = fs.fstatSync(this.fd).size;
  }

  private nextSequence(dateKey: string): number {
    let maximum = 0;
    for (const entry of fs.readdirSync(this.logDir, { withFileTypes: true })) {
      if (!entry.isFile()) continue;
      const match = entry.name.match(
        new RegExp(`^${this.escapeRegExp(this.prefix)}\\.${dateKey}\\.(\\d{3})\\.log$`),
      );
      if (match) maximum = Math.max(maximum, Number(match[1]));
    }
    return maximum === 0 ? 1 : maximum;
  }

  private escapeRegExp(value: string): string {
    return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  }
}
