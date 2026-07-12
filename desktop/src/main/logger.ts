import path from "path"
import fs from "fs"
import { getAmitiaDataDir } from "./path-manager"

let logStream: fs.WriteStream | null = null

function suppressPipeErrors() {
  const onError = (err: NodeJS.ErrnoException) => {
    if (err.code === "EPIPE" || err.code === "ERR_STREAM_DESTROYED") return
    throw err
  }
  process.stdout.on("error", onError)
  process.stderr.on("error", onError)
}

export function initLogger(): void {
  suppressPipeErrors()
  const logDir = path.join(getAmitiaDataDir(), "logs")
  if (!fs.existsSync(logDir)) {
    fs.mkdirSync(logDir, { recursive: true })
  }
  const logPath = path.join(logDir, "electron.log")
  logStream = fs.createWriteStream(logPath, { flags: "a" })

  const origLog = console.log
  const origError = console.error

  console.log = (...args: unknown[]) => {
    try { origLog(...args) } catch (_) {}
    if (logStream) {
      const msg = `[${new Date().toISOString()}] [LOG] ${args.map(String).join(" ")}\n`
      try { logStream.write(msg) } catch (_) {}
    }
  }

  console.error = (...args: unknown[]) => {
    try { origError(...args) } catch (_) {}
    if (logStream) {
      const msg = `[${new Date().toISOString()}] [ERROR] ${args.map(String).join(" ")}\n`
      try { logStream.write(msg) } catch (_) {}
    }
  }
}

export function closeLogger(): void {
  if (logStream) {
    logStream.end()
    logStream = null
  }
}
