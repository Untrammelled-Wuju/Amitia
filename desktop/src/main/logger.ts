import path from "path";
import { format } from "node:util";
import { getAmitiaDataDir } from "./path-manager";
import { recordDesktopClientError } from "./client-diagnostics";
import { DailySizeRotatingLog } from "./log-rotation";

let logWriter: DailySizeRotatingLog | null = null;

function suppressPipeErrors() {
  const onError = (err: NodeJS.ErrnoException) => {
    if (err.code === "EPIPE" || err.code === "ERR_STREAM_DESTROYED") return;
    throw err;
  };
  process.stdout.on("error", onError);
  process.stderr.on("error", onError);
}

export function initLogger(): void {
  if (logWriter) return;
  suppressPipeErrors();
  const logDir = path.join(getAmitiaDataDir(), "logs");
  logWriter = new DailySizeRotatingLog(logDir, "electron");

  const origLog = console.log;
  const origError = console.error;

  console.log = (...args: unknown[]) => {
    try {
      origLog(...args);
    } catch (error) {
      reportLoggerFailure("写入标准输出失败", error);
    }
    if (logWriter) {
      const msg = `[${new Date().toISOString()}] [LOG] ${format(...args)}\n`;
      try {
        logWriter.write(msg);
      } catch (error) {
        reportLoggerFailure("写入主日志失败", error);
      }
    }
  };

  console.error = (...args: unknown[]) => {
    try {
      origError(...args);
    } catch (error) {
      reportLoggerFailure("写入标准错误失败", error);
    }
    const text = format(...args);
    if (logWriter) {
      const level = /deprecationwarning/i.test(text) ? "[WARN]" : "[ERROR]";
      const msg = `[${new Date().toISOString()}] ${level} ${text}\n`;
      try {
        logWriter.write(msg);
      } catch (error) {
        reportLoggerFailure("写入错误日志失败", error);
      }
    }
    recordDesktopClientError({
      message: args.map((value) => value instanceof Error ? value.message : String(value)).join(" "),
      stack: args.map((value) => value instanceof Error ? value.stack ?? "" : "").filter(Boolean).join("\n"),
      source: "electron:main:console.error",
    });
  };
}

export function closeLogger(): void {
  if (logWriter) {
    const writer = logWriter;
    logWriter = null;
    writer.close();
  }
}

function reportLoggerFailure(context: string, error: unknown): void {
  const detail = error instanceof Error
    ? `${error.name}: ${error.message}\n${error.stack ?? ""}`
    : String(error);
  recordDesktopClientError({
    message: `${context}: ${detail}`,
    stack: error instanceof Error ? error.stack ?? "" : "",
    source: "electron:main:logger",
  });
  process.stderr.write(`[${new Date().toISOString()}] [ERROR] ${context}: ${detail}\n`);
}
