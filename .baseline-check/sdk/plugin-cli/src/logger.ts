import type { CliLogger, LogLevel } from "./types.js";

const SEVERITY_ORDER: Record<LogLevel, number> = {
  quiet: 100,
  error: 4,
  warn: 3,
  info: 2,
  debug: 1,
};

export class ConsoleLogger implements CliLogger {
  private level: LogLevel = "info";

  constructor(level: LogLevel = "info") {
    this.level = level;
  }

  setLevel(level: LogLevel): void {
    this.level = level;
  }

  getLevel(): LogLevel {
    return this.level;
  }

  debug(message: string, fields?: Record<string, unknown>): void {
    this.emit("debug", message, fields);
  }

  info(message: string, fields?: Record<string, unknown>): void {
    this.emit("info", message, fields);
  }

  warn(message: string, fields?: Record<string, unknown>): void {
    this.emit("warn", message, fields);
  }

  error(message: string, fields?: Record<string, unknown>): void {
    this.emit("error", message, fields);
  }

  private emit(level: LogLevel, message: string, fields?: Record<string, unknown>): void {
    if (this.level === "quiet") return;
    if (SEVERITY_ORDER[level] < SEVERITY_ORDER[this.level]) return;
    const prefix = `[${level.toUpperCase()}]`;
    const payload = fields ? `${message} ${JSON.stringify(sanitize(fields))}` : message;
    const target = level === "error" || level === "warn" ? process.stderr : process.stdout;
    target.write(`${prefix} ${payload}\n`);
  }
}

const SENSITIVE_KEYS = [
  /secret/i,
  /password/i,
  /token/i,
  /credential/i,
  /api[-_]?key/i,
  /private[-_]?key/i,
  /authorization/i,
  /cookie/i,
];

export function sanitize(fields: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(fields)) {
    if (SENSITIVE_KEYS.some((re) => re.test(key))) {
      out[key] = "***REDACTED***";
    } else if (typeof value === "string" && value.length > 4096) {
      out[key] = `${value.slice(0, 4096)}...[truncated]`;
    } else {
      out[key] = value;
    }
  }
  return out;
}
