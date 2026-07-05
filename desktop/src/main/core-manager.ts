import { app } from "electron"
import { spawn, ChildProcessWithoutNullStreams } from "child_process"
import http from "http"
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
  ensureInitialSQL(dataDir)
  ensureCoreBinaries(dataDir)
  return dataDir
}

function ensureDefaultConfig(dataDir: string): void {
  const destConfig = path.join(dataDir, "config", "config.yml")
  const resourcesPath = getCoreResourcesPath()
  const templatePath = path.join(resourcesPath, "config-template", "config.yaml")
  console.log("[CoreManager] 配置模板路径:", templatePath)
  if (fs.existsSync(templatePath)) {
    fs.copyFileSync(templatePath, destConfig)
    console.log("[CoreManager] 配置文件已更新:", destConfig)
  } else {
    console.error("[CoreManager] 配置模板不存在:", templatePath)
  }
}

function ensureInitialSQL(dataDir: string): void {
  const destSQL = path.join(dataDir, "data", "sql.sql")
  if (fs.existsSync(destSQL)) {
    console.log("[CoreManager] sql.sql已存在, 跳过复制:", destSQL)
    return
  }
  const resourcesPath = getCoreResourcesPath()
  const sourceSQL = path.join(resourcesPath, "data", "sql.sql")
  console.log("[CoreManager] sql.sql资源路径:", sourceSQL)
  if (fs.existsSync(sourceSQL)) {
    fs.copyFileSync(sourceSQL, destSQL)
    console.log("[CoreManager] sql.sql已复制到:", destSQL)
  } else {
    console.error("[CoreManager] sql.sql资源不存在:", sourceSQL)
  }
}

function ensureCoreBinaries(dataDir: string): void {
  const resourcesPath = getCoreResourcesPath()
  const binaries = [
    { src: path.join(resourcesPath, "qdrant", "qdrant.zip"), dest: path.join(dataDir, "qdrant", "qdrant.zip") },
    { src: path.join(resourcesPath, "surrealdb", "surreal.zip"), dest: path.join(dataDir, "surrealdb", "surreal.zip") },
  ]
  for (const { src, dest } of binaries) {
    if (fs.existsSync(dest)) {
      console.log("[CoreManager] 二进制已存在, 跳过复制:", dest)
      continue
    }
    if (fs.existsSync(src)) {
      fs.copyFileSync(src, dest)
      console.log("[CoreManager] 二进制已复制:", dest)
    } else {
      console.error("[CoreManager] 二进制资源不存在:", src)
    }
  }
}

export function startCore(): void {
  if (coreProcess) {
    console.log("[CoreManager] 核心进程已在运行, 跳过")
    return
  }

  const corePath = getCorePath()
  const dataDir = ensureAmitiaDataDir()

  console.log("[CoreManager] 核心路径:", corePath)
  console.log("[CoreManager] 数据目录:", dataDir)
  console.log("[CoreManager] 核心文件存在:", fs.existsSync(corePath))
  console.log("[CoreManager] 核心文件大小:", fs.existsSync(corePath) ? fs.statSync(corePath).size : 0)

  if (!fs.existsSync(corePath)) {
    const msg = `Amitia Core未找到: ${corePath}`
    console.error("[CoreManager]", msg)
    throw new Error(msg)
  }

  const configPath = path.join(dataDir, "config")
  const env: NodeJS.ProcessEnv = {
    ...process.env,
    CONFIG_PATH: configPath,
    AMITIA_RUN_MODE: "desktop",
    AMITIA_DATA_DIR: dataDir,
  }

  console.log("[CoreManager] CONFIG_PATH:", configPath)
  console.log("[CoreManager] cwd:", dataDir)

  coreProcess = spawn(corePath, [], {
    cwd: dataDir,
    env,
    windowsHide: true,
  })

  console.log("[CoreManager] 核心进程已启动, PID:", coreProcess.pid)

  coreProcess.stdout.on("data", (data) => {
    console.log(`[AmitiaCore] ${data.toString().trim()}`)
  })

  coreProcess.stderr.on("data", (data) => {
    console.error(`[AmitiaCore Error] ${data.toString().trim()}`)
  })

  coreProcess.on("exit", (code, signal) => {
    console.log(`[AmitiaCore] 进程退出, code=${code}, signal=${signal}`)
    coreProcess = null
  })

  coreProcess.on("error", (error) => {
    console.error("[AmitiaCore] 启动失败:", error.message)
    coreProcess = null
  })
}

export function stopCore(): void {
  if (coreProcess && !coreProcess.killed) {
    console.log("[CoreManager] 正在停止核心进程, PID:", coreProcess.pid)
    coreProcess.kill()
    coreProcess = null
  }
}

export function isCoreRunning(): boolean {
  return coreProcess !== null && !coreProcess.killed && coreProcess.exitCode === null
}

function httpHealthCheck(url: string, timeoutMs: number): Promise<boolean> {
  return new Promise((resolve) => {
    const req = http.get(url, { timeout: timeoutMs }, (res) => {
      res.resume()
      resolve(res.statusCode === 200)
    })
    req.on("error", () => {
      resolve(false)
    })
    req.on("timeout", () => {
      req.destroy()
      resolve(false)
    })
  })
}

export async function waitForCoreReady(timeoutMs = 60000): Promise<void> {
  const startedAt = Date.now()
  const healthUrl = "http://127.0.0.1:18080/api/health"

  while (Date.now() - startedAt < timeoutMs) {
    if (!isCoreRunning() && Date.now() - startedAt > 2000) {
      console.error("[CoreManager] 核心进程已退出, 退出码:", coreProcess?.exitCode)
      throw new Error("Amitia Core进程意外退出")
    }

    const ok = await httpHealthCheck(healthUrl, 2000)
    if (ok) {
      console.log("[CoreManager] 健康检查通过")
      return
    }

    await new Promise((resolve) => setTimeout(resolve, 500))
  }

  console.error("[CoreManager] 健康检查超时, 已等待", timeoutMs, "ms")
  throw new Error("Amitia Core启动超时")
}
