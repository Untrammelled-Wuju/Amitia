import { app } from "electron"
import { spawn, ChildProcessWithoutNullStreams } from "child_process"
import path from "path"
import fs from "fs"

let coreProcess: ChildProcessWithoutNullStreams | null = null

export function getCorePath(): string {
  if (app.isPackaged) {
    return path.join(process.resourcesPath, "core", "AmitiaCore.exe")
  }
  return path.join(process.cwd(), "resources", "core", "AmitiaCore.exe")
}

export function getCoreResourcesPath(): string {
  if (app.isPackaged) {
    return process.resourcesPath
  }
  return path.join(process.cwd(), "resources")
}

export function getAmitiaDataDir(): string {
  if (app.isPackaged) {
    return path.resolve(path.dirname(app.getPath("exe")), "..", "AmitiaData")
  }
  return path.resolve(process.cwd(), "..", "AmitiaData")
}

export function ensureAmitiaDataDir(): string {
  const dataDir = getAmitiaDataDir()
  const dirs = [
    dataDir,
    path.join(dataDir, "config"),
    path.join(dataDir, "data"),
    path.join(dataDir, "logs"),
    path.join(dataDir, "uploads"),
    path.join(dataDir, "qdrant"),
    path.join(dataDir, "surrealdb"),
    path.join(dataDir, "memory"),
    path.join(dataDir, "runtime"),
  ]
  for (const dir of dirs) {
    if (!fs.existsSync(dir)) {
      fs.mkdirSync(dir, { recursive: true })
    }
  }
  ensureDefaultConfig(dataDir)
  return dataDir
}

function ensureDefaultConfig(dataDir: string): void {
  const destConfig = path.join(dataDir, "config", "config.yml")
  if (fs.existsSync(destConfig)) {
    return
  }
  const resourcesPath = getCoreResourcesPath()
  const templatePath = path.join(resourcesPath, "config-template", "config.yaml")
  if (fs.existsSync(templatePath)) {
    fs.copyFileSync(templatePath, destConfig)
  }
}

export function startCore(): void {
  if (coreProcess) {
    return
  }

  const corePath = getCorePath()
  const dataDir = ensureAmitiaDataDir()

  if (!fs.existsSync(corePath)) {
    throw new Error(`Amitia Core not found: ${corePath}`)
  }

  const env: NodeJS.ProcessEnv = {
    ...process.env,
    CONFIG_PATH: path.join(dataDir, "config"),
    AMITIA_RUN_MODE: "desktop",
    AMITIA_DATA_DIR: dataDir,
    AMITIA_HOST: "127.0.0.1",
    AMITIA_PORT: "8899",
  }

  coreProcess = spawn(corePath, [], {
    cwd: dataDir,
    env,
    windowsHide: true,
  })

  coreProcess.stdout.on("data", (data) => {
    console.log(`[AmitiaCore] ${data.toString()}`)
  })

  coreProcess.stderr.on("data", (data) => {
    console.error(`[AmitiaCore Error] ${data.toString()}`)
  })

  coreProcess.on("exit", (code) => {
    console.log(`[AmitiaCore] exited with code ${code}`)
    coreProcess = null
  })

  coreProcess.on("error", (error) => {
    console.error("[AmitiaCore] failed to start:", error)
    coreProcess = null
  })
}

export function stopCore(): void {
  if (coreProcess && !coreProcess.killed) {
    coreProcess.kill()
    coreProcess = null
  }
}

export function isCoreRunning(): boolean {
  return coreProcess !== null && !coreProcess.killed
}

export async function waitForCoreReady(timeoutMs = 30000): Promise<void> {
  const startedAt = Date.now()
  const healthUrl = "http://127.0.0.1:8899/api/health"

  while (Date.now() - startedAt < timeoutMs) {
    if (!isCoreRunning() && Date.now() - startedAt > 3000) {
      throw new Error("Amitia Core进程意外退出")
    }
    try {
      const res = await fetch(healthUrl)
      if (res.ok) {
        return
      }
    } catch {
    }
    await new Promise((resolve) => setTimeout(resolve, 500))
  }

  throw new Error("Amitia Core启动超时")
}
