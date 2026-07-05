import path from "path"
import fs from "fs"
import { getAmitiaDataDir } from "./path-manager"

let logStream: fs.WriteStream | null = null

export function initLogger(): void {
  const logDir = path.join(getAmitiaDataDir(), "logs")
  if (!fs.existsSync(logDir)) {
    fs.mkdirSync(logDir, { recursive: true })
  }
  const logPath = path.join(logDir, "electron.log")
  logStream = fs.createWriteStream(logPath, { flags: "a" })

  const origLog = console.log
  const origError = console.error

  console.log = (...args: unknown[]) => {
    origLog(...args)
    if (logStream) {
      const msg = `[${new Date().toISOString()}] [LOG] ${args.map(String).join(" ")}\n`
      logStream.write(msg)
    }
  }

  console.error = (...args: unknown[]) => {
    origError(...args)
    if (logStream) {
      const msg = `[${new Date().toISOString()}] [ERROR] ${args.map(String).join(" ")}\n`
      logStream.write(msg)
    }
  }
}

export function closeLogger(): void {
  if (logStream) {
    logStream.end()
    logStream = null
  }
}
