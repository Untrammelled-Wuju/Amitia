import fs from "node:fs"
import path from "node:path"
import { randomBytes } from "node:crypto"

export type GroupTrigger = "mention" | "all" | "prefix"

export interface QQPersonalConfig {
  enabled: boolean
  listenHost: string
  listenPort: number
  websocketPath: string
  token: string
  groupTrigger: GroupTrigger
  triggerPrefix: string
  ignoredUserIds: string[]
}

const dataDir = process.env.AMITIA_MODULE_DATA_DIR
  ? path.resolve(process.env.AMITIA_MODULE_DATA_DIR)
  : path.resolve(process.cwd(), "data")
const configPath = path.join(dataDir, "qq-personal-config.json")

function readPort(raw: unknown, fallback: number): number {
  const parsed = Number(raw)
  return Number.isInteger(parsed) && parsed > 0 && parsed <= 65535 ? parsed : fallback
}

function normalizeTrigger(raw: unknown): GroupTrigger {
  return raw === "all" || raw === "prefix" || raw === "mention" ? raw : "mention"
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
