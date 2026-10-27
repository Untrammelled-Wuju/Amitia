import { execFile, spawn, type ChildProcess } from "node:child_process"
import fs from "node:fs"
import net from "node:net"
import path from "node:path"
import { pathToFileURL } from "node:url"
import type { QQPersonalConfig, SnowLumaRuntimeMode } from "./config.js"
import { moduleDataDir } from "./config.js"
import { extractLatestCredentials, resolveDockerBin } from "./snowluma-info.js"
import { assertSupportedProjectNode, SnowLumaInstaller, type SnowLumaInstallStatus } from "./snowluma-installer.js"

type ManagedChild = ChildProcess

export interface SnowLumaRuntimeStatus {
  platform: NodeJS.Platform
  mode: SnowLumaRuntimeMode
  managed: boolean
  running: boolean
  pid: number
  startedAt: string
  error: string
}

export interface ReverseWebSocketTarget {
  url: string
  warning: string
}

const ONEBOT_CONFIG = path.join("config", "onebot.json")
const DEFAULT_NATIVE_HTTP_PORT = 16100
const DEFAULT_NATIVE_WS_PORT = 16101
const SHUTDOWN_MESSAGE = "amitia:snowluma:shutdown"
const SHUTDOWN_BRIDGE = ".amitia-snowluma-shutdown.mjs"

function isMissing(error: unknown): boolean {
  return error instanceof Error && "code" in error && error.code === "ENOENT"
}

function sanitizeRuntimeLine(line: string): string {
  return String(line || "")
    .replace(/initial credentials: user=\S+ password=\S+/g, "initial credentials: user=*** password=***")
    .replace(/(accessToken|password|token)=([^,\s}]+)/gi, "$1=***")
    .trim()
}

function parseOneBotConfig(root: string): Record<string, unknown> | null {
  const file = path.join(root, ONEBOT_CONFIG)
  try {
    const parsed = JSON.parse(fs.readFileSync(file, "utf8")) as unknown
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      throw new Error("config/onebot.json 必须是 JSON 对象")
    }
    return parsed as Record<string, unknown>
  } catch (error) {
    if (isMissing(error)) return null
    if (error instanceof SyntaxError) throw new Error("config/onebot.json 不是有效 JSON")
    throw error
  }
}

function enabledServerWithPort(config: Record<string, unknown>, port: number): string | null {
  const networks = config.networks
  if (!networks || typeof networks !== "object" || Array.isArray(networks)) return null
  for (const kind of ["httpServers", "wsServers"]) {
    const servers = (networks as Record<string, unknown>)[kind]
    if (!Array.isArray(servers)) continue
    for (const server of servers) {
      if (!server || typeof server !== "object" || Array.isArray(server)) continue
      const item = server as Record<string, unknown>
      if (item.enabled !== false && Number(item.port) === port) {
        return `${kind}:${String(item.name || "未命名")}`
      }
    }
  }
  return null
}

function managedOneBotConfig(websocketUrl: string, token: string): Record<string, unknown> {
  return {
    networks: {
      httpServers: [],
      httpClients: [],
      wsServers: [],
      wsClients: [
        {
          name: "amitia",
          enabled: true,
          url: websocketUrl,
          role: "Universal",
          accessToken: token,
          messageFormat: "array",
          reportSelfMessage: false,
          reconnectIntervalMs: 5000,
        },
      ],
    },
    statusCommand: {
      enabled: false,
      swallow: false,
      cooldownSeconds: 5,
      trigger: "#sl",
    },
    historySync: { enabled: false },
    notifications: { channelIds: [] },
  }
}

function writeJsonAtomic(file: string, value: unknown): void {
  fs.mkdirSync(path.dirname(file), { recursive: true, mode: 0o700 })
  const temporary = `${file}.tmp`
  fs.writeFileSync(temporary, `${JSON.stringify(value, null, 2)}\n`, { mode: 0o600 })
  fs.renameSync(temporary, file)
  try {
    fs.chmodSync(file, 0o600)
  } catch {
    return
  }
}

function writeShutdownBridge(root: string): string {
  fs.mkdirSync(root, { recursive: true, mode: 0o700 })
  const file = path.join(root, SHUTDOWN_BRIDGE)
  const source = [
    `const shutdown = () => process.exit(0)`,
    `process.on("message", (message) => { if (message === ${JSON.stringify(SHUTDOWN_MESSAGE)}) shutdown() })`,
    `process.on("disconnect", shutdown)`,
    "",
  ].join("\n")
  let current = ""
  try {
    current = fs.readFileSync(file, "utf8")
  } catch {
    current = ""
  }
  if (current !== source) fs.writeFileSync(file, source, { mode: 0o600 })
  return file
}

export function effectiveSnowLumaRoot(root: string, allowedRoot = moduleDataDir()): string {
  const configured = String(root || "").trim()
  return path.resolve(configured || path.join(allowedRoot, "snowluma"))
}

function isInside(root: string, candidate: string, platform = process.platform): boolean {
  const relative = path.relative(root, candidate)
  if (platform === "win32") {
    const normalized = relative.toLowerCase()
    return normalized !== ".." && !normalized.startsWith(`..\\`) && !path.win32.isAbsolute(relative)
  }
  return relative !== ".." && !relative.startsWith(`..${path.sep}`) && !path.isAbsolute(relative)
}

function validateManagedPath(candidate: string, allowedRoot: string, label: string): void {
  const canonical = (value: string) => {
    try {
      return fs.realpathSync(value)
    } catch {
      return path.resolve(value)
    }
  }
  if (!isInside(canonical(allowedRoot), canonical(candidate))) {
    throw new Error(`${label}必须位于插件数据目录：${path.resolve(allowedRoot)}`)
  }
}

export function validateNativeSnowLuma(root: string, allowedRoot = moduleDataDir()): { nodeBin: string; entryPoint: string } {
  const resolvedRoot = path.resolve(String(root || "").trim())
  validateManagedPath(resolvedRoot, allowedRoot, "SnowLuma 目录")
  const entryPoint = path.join(resolvedRoot, "index.mjs")
  try {
    if (!fs.statSync(entryPoint).isFile()) throw new Error()
  } catch {
    throw new Error(`SnowLuma 目录中缺少 index.mjs：${resolvedRoot}`)
  }
  assertSupportedProjectNode()
  return { nodeBin: process.execPath, entryPoint }
}

export function prepareSnowLumaOneBotConfig(root: string, websocketUrl: string, token: string): { created: boolean; updated: boolean } {
  const resolvedRoot = path.resolve(root)
  const file = path.join(resolvedRoot, ONEBOT_CONFIG)
  const existing = parseOneBotConfig(resolvedRoot)
  if (!existing) {
    writeJsonAtomic(file, managedOneBotConfig(websocketUrl, token))
    return { created: true, updated: false }
  }
  const blocked3000 = enabledServerWithPort(existing, 3000)
  if (blocked3000) throw new Error(`SnowLuma 现有配置使用了 3000 端口（${blocked3000}），请改用其他端口后再启动`)
  const networks = existing.networks
  if (!networks || typeof networks !== "object" || Array.isArray(networks)) return { created: false, updated: false }
  const clients = (networks as Record<string, unknown>).wsClients
  if (!Array.isArray(clients)) return { created: false, updated: false }
  const managed = clients.find((entry) => entry && typeof entry === "object" && !Array.isArray(entry) && (entry as Record<string, unknown>).name === "amitia")
  if (!managed) return { created: false, updated: false }
  Object.assign(managed as Record<string, unknown>, {
    enabled: true,
    url: websocketUrl,
    accessToken: token,
    role: "Universal",
    messageFormat: "array",
    reconnectIntervalMs: 5000,
  })
  writeJsonAtomic(file, existing)
  return { created: false, updated: true }
}

function dockerInspect(bin: string, container: string, timeoutMs = 4000): Promise<Record<string, unknown>> {
  return new Promise((resolve, reject) => {
    execFile(bin, ["inspect", container], { encoding: "utf8", timeout: timeoutMs, maxBuffer: 8 * 1024 * 1024 }, (error, stdout) => {
      if (error) {
        reject(error)
        return
      }
      try {
        const parsed = JSON.parse(String(stdout || "[]")) as unknown
        resolve(Array.isArray(parsed) && parsed[0] && typeof parsed[0] === "object" ? parsed[0] as Record<string, unknown> : {})
      } catch {
        reject(new Error("docker inspect 输出无法解析"))
      }
    })
  })
}

export async function resolveReverseWebSocketTarget(config: QQPersonalConfig, platform = process.platform): Promise<ReverseWebSocketTarget> {
  const suffix = `${config.websocketPath.startsWith("/") ? "" : "/"}${config.websocketPath}`
  if (config.runtimeMode !== "docker") {
    return { url: `ws://127.0.0.1:${config.listenPort}${suffix}`, warning: "" }
  }
  let inspection: Record<string, unknown> = {}
  try {
    inspection = await dockerInspect(resolveDockerBin(config.dockerBin), config.containerName)
  } catch {
    const host = platform === "linux" ? "127.0.0.1" : "host.docker.internal"
    return {
      url: `ws://${host}:${config.listenPort}${suffix}`,
      warning: platform === "linux" ? "无法检查容器网络；Linux 请使用 host 网络模式" : "",
    }
  }
  const hostConfig = inspection.HostConfig && typeof inspection.HostConfig === "object" ? inspection.HostConfig as Record<string, unknown> : {}
  const networkMode = String(hostConfig.NetworkMode || "")
  if (networkMode === "host") {
    return { url: `ws://127.0.0.1:${config.listenPort}${suffix}`, warning: "" }
  }
  const wildcardListener = config.listenHost === "0.0.0.0" || config.listenHost === "::"
  if (platform === "linux" && !wildcardListener) {
    return {
      url: `ws://127.0.0.1:${config.listenPort}${suffix}`,
      warning: "Linux 默认桥接网络无法访问 Amitia 的 127.0.0.1 监听，请为容器使用 host 网络模式",
    }
  }
  const extraHosts = Array.isArray(hostConfig.ExtraHosts) ? hostConfig.ExtraHosts.map(String) : []
  if (extraHosts.some((entry) => entry.startsWith("host.docker.internal:")) || platform !== "linux") {
    return { url: `ws://host.docker.internal:${config.listenPort}${suffix}`, warning: "" }
  }
  const networks = inspection.NetworkSettings && typeof inspection.NetworkSettings === "object"
    ? (inspection.NetworkSettings as Record<string, unknown>).Networks
    : null
  if (networks && typeof networks === "object" && !Array.isArray(networks)) {
    for (const entry of Object.values(networks)) {
      if (!entry || typeof entry !== "object" || Array.isArray(entry)) continue
      const gateway = String((entry as Record<string, unknown>).Gateway || "").trim()
      if (gateway) {
        return {
          url: `ws://${gateway}:${config.listenPort}${suffix}`,
          warning: "Linux 默认桥接网络无法访问 Amitia 的 127.0.0.1 监听，请为容器使用 host 网络模式",
        }
      }
    }
  }
  return {
    url: `ws://127.0.0.1:${config.listenPort}${suffix}`,
    warning: "Linux 默认桥接网络无法访问 Amitia 的 127.0.0.1 监听，请为容器使用 host 网络模式",
  }
}

function waitForPort(host: string, port: number, timeoutMs: number, child: ManagedChild): Promise<void> {
  return new Promise((resolve, reject) => {
    const deadline = Date.now() + timeoutMs
    const overall = setTimeout(() => reject(new Error("等待 SnowLuma WebUI 启动超时")), timeoutMs)
    overall.unref()
    const tryConnect = () => {
      if (child.exitCode !== null) {
        clearTimeout(overall)
        reject(new Error("SnowLuma 进程已退出"))
        return
      }
      const socket = net.createConnection({ host, port })
      const fail = () => {
        socket.destroy()
        if (Date.now() >= deadline) {
          clearTimeout(overall)
          reject(new Error("等待 SnowLuma WebUI 启动超时"))
          return
        }
        setTimeout(tryConnect, 300)
      }
      socket.once("connect", () => {
        clearTimeout(overall)
        socket.destroy()
        resolve()
      })
      socket.once("error", fail)
    }
    tryConnect()
  })
}

export class SnowLumaRuntimeManager {
  private child: ManagedChild | null = null
  private mode: SnowLumaRuntimeMode = "docker"
  private startedAt = ""
  private error = ""
  private lastLines: string[] = []
  private bootstrapPassword = ""
  private stopping = false
  private fingerprint = ""
  private readonly allowedRoot: string
  private readonly installer: SnowLumaInstaller

  constructor(allowedRoot = moduleDataDir()) {
    this.allowedRoot = path.resolve(allowedRoot)
    this.installer = new SnowLumaInstaller(this.allowedRoot)
  }

  nativeRoot(config: QQPersonalConfig): string {
    return this.installer.installedRoot() || effectiveSnowLumaRoot(config.snowlumaRoot, this.allowedRoot)
  }

  installState(): SnowLumaInstallStatus {
    return this.installer.installState()
  }

  async install(config: QQPersonalConfig): Promise<SnowLumaInstallStatus> {
    if (this.installer.installState().running) return this.installState()
    void (async () => {
      await this.stop()
      const result = await this.installer.install(config.snowlumaRoot)
      if (result.phase === "failed") {
        this.recordError("native", result.error)
        return
      }
      try {
        await this.start(config)
      } catch (error) {
        const failure = error instanceof Error ? error.message : String(error)
        const rolledBack = this.installer.rollback()
        if (rolledBack) {
          try {
            await this.start(config)
            this.recordError("native", `${failure}，已回滚到上一版本`)
            return
          } catch {
            this.recordError("native", failure)
            return
          }
        }
        this.recordError("native", failure)
      }
    })()
    return this.installState()
  }

  status(): SnowLumaRuntimeStatus {
    return {
      platform: process.platform,
      mode: this.mode,
      managed: this.mode === "native" && this.child !== null,
      running: this.child !== null && this.child.exitCode === null,
      pid: this.child?.pid || 0,
      startedAt: this.startedAt,
      error: this.error,
    }
  }

  webuiBootstrapPassword(): string {
    return this.bootstrapPassword
  }

  recentLines(): string[] {
    return [...this.lastLines]
  }

  recordError(mode: SnowLumaRuntimeMode, error: string): void {
    this.mode = mode
    this.error = sanitizeRuntimeLine(error)
  }

  async start(config: QQPersonalConfig): Promise<SnowLumaRuntimeStatus> {
    this.mode = config.runtimeMode
    if (!config.enabled || config.runtimeMode !== "native") {
      await this.stop()
      this.mode = config.runtimeMode
      this.error = ""
      return this.status()
    }
    const nextFingerprint = JSON.stringify({
      root: this.nativeRoot(config),
      token: config.token,
      listenPort: config.listenPort,
      websocketPath: config.websocketPath,
      webuiPort: config.webuiPort,
    })
    if (this.child && this.child.exitCode === null && this.fingerprint === nextFingerprint) return this.status()
    await this.stop()
    const root = this.nativeRoot(config)
    const { nodeBin, entryPoint } = validateNativeSnowLuma(root, this.allowedRoot)
    const websocketPath = `${config.websocketPath.startsWith("/") ? "" : "/"}${config.websocketPath}`
    prepareSnowLumaOneBotConfig(root, `ws://127.0.0.1:${config.listenPort}${websocketPath}`, config.token)
    const shutdownBridge = writeShutdownBridge(this.allowedRoot)
    const child = spawn(nodeBin, ["--import", pathToFileURL(shutdownBridge).href, entryPoint], {
      cwd: root,
      env: {
        ...process.env,
        NO_COLOR: "1",
        SNOWLUMA_HOOK_AUTOLOAD: "1",
        SNOWLUMA_UPDATE_ROOT: root,
        SNOWLUMA_LOG_DIR: path.join(root, "logs"),
        SNOWLUMA_WEBUI_PORT: String(config.webuiPort),
        SNOWLUMA_ONEBOT_HTTP_PORT: String(DEFAULT_NATIVE_HTTP_PORT),
        SNOWLUMA_ONEBOT_WS_PORT: String(DEFAULT_NATIVE_WS_PORT),
      },
      stdio: ["ignore", "pipe", "pipe", "ipc"],
      windowsHide: true,
    })
    this.child = child
    this.stopping = false
    this.fingerprint = nextFingerprint
    this.startedAt = new Date().toISOString()
    this.error = ""
    this.bootstrapPassword = ""
    this.lastLines = []
    const collect = (chunk: Buffer | string, source: "stdout" | "stderr") => {
      const text = String(chunk || "")
      const credentials = extractLatestCredentials(text)
      if (credentials) this.bootstrapPassword = credentials.password
      for (const line of text.split(/\r?\n/).map(sanitizeRuntimeLine).filter(Boolean)) {
        this.lastLines.push(line)
        if (this.lastLines.length > 80) this.lastLines.shift()
      }
      if (source === "stderr" && child.exitCode === null) this.error = this.lastLines[this.lastLines.length - 1] || this.error
    }
    child.stdout?.on("data", (chunk) => collect(chunk, "stdout"))
    child.stderr?.on("data", (chunk) => collect(chunk, "stderr"))
    child.once("error", (failure) => {
      this.error = failure.message
      this.child = null
    })
    child.once("exit", (code, signal) => {
      if (!this.stopping) {
        this.error = `SnowLuma 已退出（${signal || `code ${code ?? "unknown"}`}）${this.lastLines.length ? `：${this.lastLines[this.lastLines.length - 1]}` : ""}`
      }
      this.child = null
    })
    try {
      await waitForPort("127.0.0.1", config.webuiPort, 12000, child)
    } catch (failure) {
      this.error = failure instanceof Error ? failure.message : String(failure)
    }
    return this.status()
  }

  async stop(): Promise<void> {
    const child = this.child
    this.child = null
    this.startedAt = ""
    this.fingerprint = ""
    this.stopping = true
    if (!child || child.exitCode !== null) return
    const exited = new Promise<boolean>((resolve) => {
      const timer = setTimeout(() => resolve(false), 5000)
      child.once("exit", () => {
        clearTimeout(timer)
        resolve(true)
      })
    })
    if (child.connected) {
      child.send(SHUTDOWN_MESSAGE, (error) => {
        if (!error) return
        try {
          child.kill("SIGTERM")
        } catch {
          return
        }
      })
    } else {
      try {
        child.kill("SIGTERM")
      } catch {
        return
      }
    }
    const stopped = await exited
    if (!stopped && child.exitCode === null) {
      try {
        child.kill("SIGKILL")
      } catch {
        this.error = "SnowLuma 进程未能退出"
      }
    }
  }
}
