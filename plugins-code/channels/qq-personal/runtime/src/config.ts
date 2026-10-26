import fs from "node:fs"
import path from "node:path"
import { randomBytes } from "node:crypto"

export type GroupTrigger = "mention" | "all" | "prefix"
export type SnowLumaRuntimeMode = "native" | "docker"

export interface QQPersonalConfig {
  enabled: boolean
  listenHost: string
  listenPort: number
  websocketPath: string
  token: string
  groupTrigger: GroupTrigger
  triggerPrefix: string
  ignoredUserIds: string[]
  runtimeMode: SnowLumaRuntimeMode
  snowlumaRoot: string
  webuiPort: number
  novncPort: number
  containerName: string
  dockerBin: string
}

const dataDir = process.env.AMITIA_MODULE_DATA_DIR
  ? path.resolve(process.env.AMITIA_MODULE_DATA_DIR)
  : path.resolve(process.cwd(), "data")
const configPath = path.join(dataDir, "qq-personal-config.json")

export function moduleDataDir(): string {
  return dataDir
}

function readPort(raw: unknown, fallback: number): number {
  const parsed = Number(raw)
  return Number.isInteger(parsed) && parsed > 0 && parsed <= 65535 ? parsed : fallback
}

function normalizeTrigger(raw: unknown): GroupTrigger {
  return raw === "all" || raw === "prefix" || raw === "mention" ? raw : "mention"
}

function normalizeText(raw: unknown, fallback: string): string {
  const value = String(raw ?? "").trim()
  return value || fallback
}

export function isRuntimeModeSupported(mode: string, platform = process.platform): mode is SnowLumaRuntimeMode {
  if (platform === "darwin") return mode === "docker"
  return mode === "docker" || (mode === "native" && (platform === "win32" || platform === "linux"))
}

export function normalizeRuntimeModeForPlatform(raw: unknown, fallback: SnowLumaRuntimeMode, platform = process.platform): SnowLumaRuntimeMode {
  if (typeof raw === "string" && isRuntimeModeSupported(raw, platform)) return raw
  if (isRuntimeModeSupported(fallback, platform)) return fallback
  return "docker"
}

function defaultRuntimeMode(): SnowLumaRuntimeMode {
  if (process.env.QQ_PERSONAL_RUNTIME_MODE && isRuntimeModeSupported(process.env.QQ_PERSONAL_RUNTIME_MODE)) return process.env.QQ_PERSONAL_RUNTIME_MODE
  return process.platform === "win32" ? "native" : "docker"
}

export function generateToken(): string {
  return randomBytes(32).toString("hex")
}

export function defaults(): QQPersonalConfig {
  const configuredPort = process.env.QQ_PERSONAL_PORT
  return {
    enabled: true,
    listenHost: process.env.QQ_PERSONAL_HOST || "127.0.0.1",
    listenPort: readPort(configuredPort, 6199),
    websocketPath: process.env.QQ_PERSONAL_PATH || "/ws",
    token: process.env.QQ_PERSONAL_TOKEN || generateToken(),
    groupTrigger: normalizeTrigger(process.env.QQ_PERSONAL_GROUP_TRIGGER),
    triggerPrefix: process.env.QQ_PERSONAL_TRIGGER_PREFIX || "Amitia",
    ignoredUserIds: ["2854196310"],
    runtimeMode: defaultRuntimeMode(),
    snowlumaRoot: "",
    webuiPort: readPort(process.env.QQ_PERSONAL_WEBUI_PORT, 5099),
    novncPort: readPort(process.env.QQ_PERSONAL_NOVNC_PORT, 6081),
    containerName: process.env.QQ_PERSONAL_CONTAINER || "snowluma",
    dockerBin: process.env.QQ_PERSONAL_DOCKER_BIN || "",
  }
}

export function normalizeConfig(input: Partial<QQPersonalConfig>, base = defaults()): QQPersonalConfig {
  return {
    enabled: input.enabled !== false,
    listenHost: base.listenHost,
    listenPort: base.listenPort,
    websocketPath: base.websocketPath,
    token: String(input.token || base.token).trim(),
    groupTrigger: normalizeTrigger(input.groupTrigger || base.groupTrigger),
    triggerPrefix: String(input.triggerPrefix ?? base.triggerPrefix).trim(),
    ignoredUserIds: Array.isArray(input.ignoredUserIds)
      ? input.ignoredUserIds.map(String).map((item) => item.trim()).filter(Boolean)
      : base.ignoredUserIds,
    runtimeMode: normalizeRuntimeModeForPlatform(input.runtimeMode, base.runtimeMode),
    snowlumaRoot: String(input.snowlumaRoot ?? base.snowlumaRoot).trim(),
    webuiPort: readPort(input.webuiPort, base.webuiPort),
    novncPort: readPort(input.novncPort, base.novncPort),
    containerName: normalizeText(input.containerName, base.containerName),
    dockerBin: String(input.dockerBin ?? base.dockerBin).trim(),
  }
}

function ensureDataDir(): void {
  fs.mkdirSync(dataDir, { recursive: true, mode: 0o700 })
}

export function loadConfig(): QQPersonalConfig {
  ensureDataDir()
  const fallback = defaults()
  try {
    const parsed = JSON.parse(fs.readFileSync(configPath, "utf8")) as Partial<QQPersonalConfig>
    const config = normalizeConfig(parsed, fallback)
    saveConfig(config)
    return config
  } catch {
    saveConfig(fallback)
    return fallback
  }
}

export function saveConfig(config: QQPersonalConfig): void {
  ensureDataDir()
  const temporary = `${configPath}.tmp`
  fs.writeFileSync(temporary, `${JSON.stringify(config, null, 2)}\n`, { mode: 0o600 })
  fs.renameSync(temporary, configPath)
  fs.chmodSync(configPath, 0o600)
}

export function dataFilePath(name: string): string {
  ensureDataDir()
  return path.join(dataDir, name)
}
