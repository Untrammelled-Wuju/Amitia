import { execFile, spawn } from "node:child_process"
import fs from "node:fs"
import path from "node:path"
import { dockerCliEnv, resolveDockerBin } from "./snowluma-info.js"

export interface DockerServiceStatus {
  available: boolean
  version: string
  error: string
}

export interface DockerServiceStartResult extends DockerServiceStatus {
  alreadyRunning: boolean
  started: boolean
  containerName: string
  containerStarted: boolean
  containerRunning: boolean
  message: string
}

interface StartDockerOptions {
  dockerBin?: string
  containerName?: string
  launchDocker?: boolean
  startTimeoutMs?: number
  readyTimeoutMs?: number
  pollIntervalMs?: number
}

interface ContainerState {
  exists: boolean
  running: boolean
  status: string
  error: string
}

function runCommand(file: string, args: string[], timeoutMs: number): Promise<string> {
  return new Promise((resolve, reject) => {
    execFile(file, args, {
      encoding: "utf8",
      timeout: timeoutMs,
      maxBuffer: 8 * 1024 * 1024,
      windowsHide: true,
      env: dockerCliEnv(),
    }, (error, stdout, stderr) => {
      if (error) {
        const failure = new Error(String(stderr || stdout || error.message).trim() || "Docker 命令执行失败")
        const code = "code" in error ? String(error.code || "") : ""
        if (code) Object.assign(failure, { code })
        reject(failure)
        return
      }
      resolve(String(stdout || "").trim())
    })
  })
}

function describeDockerError(error: unknown): string {
  const message = error instanceof Error ? error.message : String(error || "")
  if (/ENOENT|not found|无法将.+识别为|不是内部或外部命令/i.test(message)) {
    return "未找到 Docker Desktop，请先安装 Docker"
  }
  if (/Cannot connect to the Docker daemon|Is the docker daemon running|docker\.sock|failed to connect to the docker API/i.test(message)) {
    return "Docker 未运行"
  }
  if (/unknown command|not a docker command|is not a docker command/i.test(message)) {
    return "当前 Docker 版本不支持一键启动，请打开 Docker Desktop"
  }
  if (/operation not permitted|failed to fire hook|unable to create a log file|LSOpenURLsWithCompletionHandler|EACCES|EPERM/i.test(message)) {
    return "Docker Desktop 启动权限受限，请检查系统权限"
  }
  return message.trim() || "Docker 服务不可用"
}

function prepareDockerDesktopPlugin(dockerBin: string): void {
  const env = dockerCliEnv()
  const configDir = String(env.DOCKER_CONFIG || "").trim()
  if (!configDir) return
  const pluginName = process.platform === "win32" ? "docker-desktop.exe" : "docker-desktop"
  const candidates = [
    path.resolve(path.dirname(dockerBin), "..", "cli-plugins", pluginName),
    process.platform === "darwin" ? path.join("/Applications/Docker.app/Contents/Resources/cli-plugins", pluginName) : "",
  ].filter(Boolean)
  const source = candidates.find((candidate) => {
    try {
      fs.accessSync(candidate, fs.constants.X_OK)
      return true
    } catch {
      return false
    }
  })
  if (!source) return
  const pluginDir = path.join(configDir, "cli-plugins")
  const target = path.join(pluginDir, pluginName)
  fs.mkdirSync(pluginDir, { recursive: true, mode: 0o700 })
  try {
    fs.lstatSync(target)
    return
  } catch {
    try {
      fs.symlinkSync(source, target, "file")
    } catch {
      if (process.platform === "win32") fs.copyFileSync(source, target)
    }
  }
}

export async function inspectDockerService(dockerBin = resolveDockerBin(), timeoutMs = 5000): Promise<DockerServiceStatus> {
  try {
    const version = await runCommand(dockerBin, ["info", "--format", "{{.ServerVersion}}"], timeoutMs)
    return { available: true, version, error: "" }
  } catch (error) {
    return { available: false, version: "", error: describeDockerError(error) }
  }
}

function parseContainerState(raw: string): ContainerState {
  try {
    const parsed = JSON.parse(raw) as Record<string, unknown>
    return {
      exists: true,
      running: parsed.Running === true && parsed.Restarting !== true && parsed.Dead !== true,
      status: String(parsed.Status || ""),
      error: String(parsed.Error || ""),
    }
  } catch {
    return { exists: true, running: false, status: "", error: "容器状态无法解析" }
  }
}

async function inspectContainer(dockerBin: string, containerName: string, timeoutMs = 5000): Promise<ContainerState> {
  try {
    const raw = await runCommand(dockerBin, ["inspect", containerName, "--format", "{{json .State}}"], timeoutMs)
    return parseContainerState(raw)
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error || "")
    if (/No such (object|container)|no such (object|container)/i.test(message)) {
      return { exists: false, running: false, status: "", error: "" }
    }
    return { exists: false, running: false, status: "", error: describeDockerError(error) }
  }
}

function wait(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

function launchMacDockerBackend(dockerBin: string): Promise<void> {
  const executable = path.resolve(path.dirname(dockerBin), "..", "..", "MacOS", "com.docker.backend")
  return new Promise((resolve, reject) => {
    const child = spawn(executable, [], {
      detached: true,
      stdio: "ignore",
    })
    const timer = setTimeout(() => reject(new Error("Docker Desktop 启动超时")), 10000)
    child.once("error", (error) => {
      clearTimeout(timer)
      reject(error)
    })
    child.once("spawn", () => {
      clearTimeout(timer)
      child.unref()
      resolve()
    })
  })
}

export async function startDockerService(options: StartDockerOptions = {}): Promise<DockerServiceStartResult> {
  const dockerBin = options.dockerBin || resolveDockerBin()
  const containerName = String(options.containerName || "").trim()
  const launchDocker = options.launchDocker !== false
  const startTimeoutMs = options.startTimeoutMs ?? 75000
  const readyTimeoutMs = options.readyTimeoutMs ?? 45000
  const pollIntervalMs = options.pollIntervalMs ?? 1200
  const initial = await inspectDockerService(dockerBin)
  const dockerAlreadyRunning = initial.available
  let dockerStatus = initial
  if (!dockerStatus.available) {
    if (dockerStatus.error === "未找到 Docker Desktop，请先安装 Docker") throw new Error(dockerStatus.error)
    if (launchDocker) {
      try {
        prepareDockerDesktopPlugin(dockerBin)
        await runCommand(dockerBin, ["desktop", "start"], startTimeoutMs)
      } catch (error) {
        const detail = describeDockerError(error)
        if (process.platform !== "darwin" || !/权限受限/.test(detail)) throw new Error(detail)
        try {
          await launchMacDockerBackend(dockerBin)
        } catch (fallbackError) {
          throw new Error(describeDockerError(fallbackError))
        }
      }
    }
    const dockerDeadline = Date.now() + readyTimeoutMs
    while (Date.now() < dockerDeadline) {
      dockerStatus = await inspectDockerService(dockerBin, 3000)
      if (dockerStatus.available) break
      await wait(pollIntervalMs)
    }
    if (!dockerStatus.available) throw new Error("Docker Desktop 已收到启动请求，但服务尚未就绪，请稍后刷新")
  }

  let containerStarted = false
  let containerState = containerName
    ? await inspectContainer(dockerBin, containerName)
    : { exists: false, running: false, status: "", error: "" }
  if (containerName) {
    if (!containerState.exists) {
      if (containerState.error) throw new Error(containerState.error)
      throw new Error(`未找到 SnowLuma 容器 ${containerName}，请检查容器名`)
    }
    if (!containerState.running) {
      try {
        await runCommand(dockerBin, ["start", containerName], 30000)
        containerStarted = true
      } catch (error) {
        throw new Error(`SnowLuma 容器启动失败：${describeDockerError(error)}`)
      }
      const containerDeadline = Date.now() + readyTimeoutMs
      while (Date.now() < containerDeadline) {
        containerState = await inspectContainer(dockerBin, containerName, 3000)
        if (containerState.running) break
        await wait(pollIntervalMs)
      }
      if (!containerState.running) {
        throw new Error(`SnowLuma 容器未能进入运行状态${containerState.status ? `：${containerState.status}` : ""}`)
      }
    }
  }
  const message = !containerName
    ? dockerAlreadyRunning ? "Docker 已在运行" : "Docker 已启动"
    : dockerAlreadyRunning && !containerStarted ? "Docker 与 SnowLuma 已在运行"
      : dockerAlreadyRunning ? "SnowLuma 容器已启动"
        : "Docker 与 SnowLuma 已启动"
  return {
    ...dockerStatus,
    alreadyRunning: dockerAlreadyRunning,
    started: !dockerAlreadyRunning,
    containerName,
    containerStarted,
    containerRunning: Boolean(containerName && containerState.running),
    message,
  }
}
