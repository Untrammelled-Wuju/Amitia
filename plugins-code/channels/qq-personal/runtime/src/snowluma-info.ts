import { execFile } from "node:child_process"
import fs from "node:fs"
import os from "node:os"
import path from "node:path"
import { moduleDataDir } from "./config.js"

export type WebUIPasswordState = "stored" | "temporary" | "changed" | "unavailable"

export interface SnowLumaInfo {
  webuiUrl: string
  novncUrl: string
  webuiUser: string
  webuiPassword: string
  webuiPasswordState: WebUIPasswordState
  novncPassword: string
  error: string
}

export interface SnowLumaTarget {
  runtimeMode?: "native" | "docker"
  snowlumaRoot?: string
  bootstrapPassword?: string
  containerName: string
  webuiPort: number
  novncPort: number
  dockerBin?: string
  host?: string
  timeoutMs?: number
}

export interface PasswordRule {
  id: string
  label: string
  ok: boolean
}

export interface PasswordEvaluation {
  valid: boolean
  rules: PasswordRule[]
}

export interface ChangePasswordResult {
  ok: boolean
  message: string
}

const WEBUI_CONTAINER_PATH = "/app/data/config/webui.json"
const VNC_CONTAINER_PATH = "/app/data/config/vnc-password"
const DOCKER_CANDIDATES = [
  "/usr/local/bin/docker",
  "/opt/homebrew/bin/docker",
  "/usr/bin/docker",
  "/Applications/Docker.app/Contents/Resources/bin/docker",
]
const WINDOWS_DOCKER_CANDIDATES = [
  "C:\\Program Files\\Docker\\Docker\\resources\\bin\\docker.exe",
  "C:\\ProgramData\\DockerDesktop\\version-bin\\docker.exe",
]

const PASSWORD_RULES: Array<{ id: string; label: string; test: (value: string) => boolean }> = [
  { id: "length", label: "长度不少于 10 位", test: (value) => value.length >= 10 },
  { id: "lower", label: "包含小写字母", test: (value) => /[a-z]/.test(value) },
  { id: "upper", label: "包含大写字母", test: (value) => /[A-Z]/.test(value) },
  { id: "special", label: "包含特殊符号 (!@#$%…)", test: (value) => /[^A-Za-z0-9\s]/.test(value) },
  { id: "no-space", label: "不包含空格", test: (value) => value.length > 0 && !/\s/.test(value) },
]

export function evaluatePassword(password: string): PasswordEvaluation {
  const value = String(password || "")
  const rules = PASSWORD_RULES.map((rule) => ({ id: rule.id, label: rule.label, ok: rule.test(value) }))
  return { valid: rules.every((rule) => rule.ok), rules }
}

export function resolveDockerBin(configured = "", home = os.homedir(), platform = process.platform, env: NodeJS.ProcessEnv = process.env): string {
  const explicit = String(configured || "").trim()
  if (explicit) return explicit
  if (platform === "win32") {
    const root = String(env.ProgramFiles || "C:\\Program Files")
    const data = String(env.ProgramData || "C:\\ProgramData")
    const candidates = [
      path.win32.join(root, "Docker", "Docker", "resources", "bin", "docker.exe"),
      path.win32.join(data, "DockerDesktop", "version-bin", "docker.exe"),
      ...WINDOWS_DOCKER_CANDIDATES,
      path.win32.join(home, ".docker", "bin", "docker.exe"),
    ]
    for (const candidate of [...new Set(candidates)]) {
      try {
        fs.accessSync(candidate, fs.constants.F_OK)
        return candidate
      } catch {
        continue
      }
    }
    return "docker.exe"
  }
  const candidates = [...DOCKER_CANDIDATES, path.join(home, ".docker", "bin", "docker")]
  for (const candidate of candidates) {
    try {
      fs.accessSync(candidate, fs.constants.X_OK)
      return candidate
    } catch {
      continue
    }
  }
  return "docker"
}

export function dockerCliEnv(home = os.homedir(), platform = process.platform, env: NodeJS.ProcessEnv = process.env): NodeJS.ProcessEnv {
  const configDir = path.join(moduleDataDir(), ".docker-cli")
  fs.mkdirSync(configDir, { recursive: true, mode: 0o700 })
  const next: NodeJS.ProcessEnv = { ...env, DOCKER_CONFIG: configDir }
  if (!next.DOCKER_HOST && platform === "darwin") {
    next.DOCKER_HOST = `unix://${path.join(home, ".docker", "run", "docker.sock")}`
  }
  return next
}

export function extractLatestCredentials(logs: string): { user: string; password: string } | null {
  const pattern = /initial credentials: user=(\S+) password=(\S+)/g
  let latest: { user: string; password: string } | null = null
  for (const match of String(logs || "").matchAll(pattern)) {
    latest = { user: match[1], password: match[2] }
  }
  return latest
}

function runDocker(bin: string, args: string[], timeoutMs: number): Promise<string> {
  return new Promise((resolve, reject) => {
    execFile(bin, args, { encoding: "utf8", timeout: timeoutMs, maxBuffer: 8 * 1024 * 1024, env: dockerCliEnv() }, (error, stdout) => {
      if (error) {
        reject(error)
        return
      }
      resolve(String(stdout || ""))
    })
  })
}

function describeFailure(error: unknown, container: string): string {
  const raw = error instanceof Error ? error.message : String(error || "")
  if (/ENOENT/.test(raw)) return process.platform === "win32"
    ? "未找到 docker.exe，请安装 Docker Desktop 或在下方填写可执行文件路径"
    : "未找到 docker 命令，请安装 Docker 或在下方填写可执行文件路径"
  if (/No such container|no such container/i.test(raw)) return `未找到容器 ${container}，请确认容器名`
  if (/Cannot connect to the Docker daemon|Is the docker daemon running/i.test(raw)) return "Docker 未运行，请先启动 Docker Desktop"
  if (/timed out|killed/i.test(raw)) return "读取 SnowLuma 信息超时"
  return raw.trim() || "读取 SnowLuma 信息失败"
}

export async function readSnowLumaInfo(target: SnowLumaTarget, storedPassword = ""): Promise<SnowLumaInfo> {
  const host = String(target.host || "127.0.0.1").trim() || "127.0.0.1"
  const container = String(target.containerName || "").trim() || "snowluma"
  const timeoutMs = target.timeoutMs ?? 4000
  const stored = String(storedPassword || "")
  const info: SnowLumaInfo = {
    webuiUrl: `http://${host}:${target.webuiPort}/`,
    novncUrl: `http://${host}:${target.novncPort}/`,
    webuiUser: "admin",
    webuiPassword: stored,
    webuiPasswordState: stored ? "stored" : "unavailable",
    novncPassword: "",
    error: "",
  }
  if (target.runtimeMode === "native") {
    return readNativeSnowLumaInfo(target, info, stored)
  }
  const bin = resolveDockerBin(target.dockerBin)
  let readFailure = ""
  try {
    const raw = await runDocker(bin, ["exec", container, "cat", WEBUI_CONTAINER_PATH], timeoutMs)
    const parsed = JSON.parse(raw) as Record<string, unknown>
    const configuredUser = String(parsed.username || parsed.user || "").trim()
    if (configuredUser) info.webuiUser = configuredUser
    if (!stored) {
      if (parsed.mustChangePassword === false) {
        info.webuiPasswordState = "changed"
      } else {
        const logs = await runDocker(bin, ["logs", "--tail", "4000", container], timeoutMs)
        const credentials = extractLatestCredentials(logs)
        if (credentials) {
          info.webuiUser = credentials.user
          info.webuiPassword = credentials.password
          info.webuiPasswordState = "temporary"
        } else {
          readFailure = "未在 SnowLuma 启动日志中找到初始密码"
        }
      }
    }
  } catch (error) {
    readFailure = describeFailure(error, container)
  }
  if (!stored && info.webuiPasswordState === "unavailable" && readFailure) info.error = readFailure
  try {
    const raw = await runDocker(bin, ["exec", container, "cat", VNC_CONTAINER_PATH], timeoutMs)
    info.novncPassword = String(raw || "").trim()
  } catch (error) {
    const detail = describeFailure(error, container)
    if (!info.error) info.error = detail
  }
  if (!info.novncPassword && !info.error) info.error = "未读取到 noVNC 密码"
  return info
}

async function readNativeSnowLumaInfo(target: SnowLumaTarget, info: SnowLumaInfo, stored: string): Promise<SnowLumaInfo> {
  const root = String(target.snowlumaRoot || "").trim()
  info.novncUrl = ""
  if (!root) {
    info.error = "请填写 SnowLuma 解压目录"
    return info
  }
  const webuiPath = path.join(root, "config", "webui.json")
  try {
    const parsed = JSON.parse(fs.readFileSync(webuiPath, "utf8")) as Record<string, unknown>
    const configuredUser = String(parsed.username || parsed.user || "").trim()
    if (configuredUser) info.webuiUser = configuredUser
    if (!stored) {
      if (parsed.mustChangePassword === false) {
        info.webuiPasswordState = "changed"
      } else if (target.bootstrapPassword) {
        info.webuiPassword = String(target.bootstrapPassword)
        info.webuiPasswordState = "temporary"
      } else {
        info.webuiPasswordState = "unavailable"
        info.error = "未捕获首次启动密码，请登录 WebUI 后记录当前密码"
      }
    }
  } catch (error) {
    const raw = error instanceof Error ? error.message : String(error || "")
    if (/ENOENT/.test(raw)) {
      info.error = "SnowLuma 尚未生成 WebUI 凭据，请确认程序已启动"
    } else if (error instanceof SyntaxError) {
      info.error = "SnowLuma WebUI 凭据文件格式无效"
    } else {
      info.error = raw.trim() || "读取 SnowLuma WebUI 凭据失败"
    }
  }
  return info
}

interface JsonReply {
  ok: boolean
  status: number
  body: Record<string, unknown>
}

async function postJson(url: string, payload: Record<string, unknown>, timeoutMs: number, headers: Record<string, string> = {}): Promise<JsonReply> {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), timeoutMs)
  try {
    const response = await fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json", ...headers },
      body: JSON.stringify(payload),
      signal: controller.signal,
    })
    const body = (await response.json().catch(() => ({}))) as Record<string, unknown>
    return { ok: response.ok, status: response.status, body }
  } finally {
    clearTimeout(timer)
  }
}

export async function changeWebuiPassword(
  target: Pick<SnowLumaTarget, "host" | "webuiPort" | "timeoutMs">,
  oldPassword: string,
  newPassword: string,
  username = "admin",
): Promise<ChangePasswordResult> {
  const host = String(target.host || "127.0.0.1").trim() || "127.0.0.1"
  const base = `http://${host}:${target.webuiPort}`
  const timeoutMs = target.timeoutMs ?? 12000
  let login: JsonReply
  try {
    login = await postJson(`${base}/api/login`, { username, password: oldPassword }, timeoutMs)
  } catch (error) {
    const raw = error instanceof Error ? error.message : String(error || "")
    if (/abort|aborted/i.test(raw)) return { ok: false, message: "连接 SnowLuma WebUI 超时" }
    return { ok: false, message: `无法连接 SnowLuma WebUI：${raw || "未知错误"}` }
  }
  if (!login.ok) {
    if (login.status === 401) return { ok: false, message: "当前密码不正确" }
    const detail = String(login.body.message || "").trim()
    return { ok: false, message: detail ? `登录 SnowLuma 失败：${detail}` : `登录 SnowLuma 失败（HTTP ${login.status}）` }
  }
  const token = String(login.body.token || "").trim()
  if (!token) return { ok: false, message: "SnowLuma 未返回登录凭据" }
  let changed: JsonReply
  try {
    changed = await postJson(`${base}/api/auth/change-password`, { oldPassword, newPassword }, timeoutMs, { "x-snowluma-token": token })
  } catch (error) {
    const raw = error instanceof Error ? error.message : String(error || "")
    if (/abort|aborted/i.test(raw)) return { ok: false, message: "SnowLuma 改密响应超时" }
    return { ok: false, message: `改密请求失败：${raw || "未知错误"}` }
  }
  if (changed.ok && changed.body.success === true) return { ok: true, message: "密码已更新" }
  const message = String(changed.body.message || "").trim()
  if (message && message !== "Unauthorized") return { ok: false, message }
  if (changed.status === 401) return { ok: false, message: "当前密码不正确" }
  if (changed.status === 400) return { ok: false, message: "新密码不符合要求" }
  return { ok: false, message: `改密失败（HTTP ${changed.status}）` }
}
