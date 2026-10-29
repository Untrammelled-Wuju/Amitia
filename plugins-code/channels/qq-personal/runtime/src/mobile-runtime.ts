import fs from "node:fs"
import path from "node:path"
import type { QQPersonalConfig } from "./config.js"
import { MobileDesktop } from "./mobile-desktop.js"
import { prepareMobileEnvironment, mobileEnvironmentReady } from "./mobile-environment.js"
import type { SnowLumaRuntimeManager } from "./snowluma-runtime.js"
import { linkMobileEnvironment } from "./mobile-links.js"

interface MobileRuntimeState {
  running: boolean
  phase: string
  detail: string
  error: string
  updatedAt: string
}

const INITIAL_STATE: MobileRuntimeState = {
  running: false,
  phase: "idle",
  detail: "",
  error: "",
  updatedAt: "",
}

export class MobileQQRuntime {
  private readonly root: string
  private readonly stateFile: string
  private readonly desktop: MobileDesktop
  private state: MobileRuntimeState

  constructor(dataRoot: string, private readonly snowluma: SnowLumaRuntimeManager) {
    this.root = path.join(dataRoot, "mobile")
    this.stateFile = path.join(this.root, "status.json")
    this.desktop = new MobileDesktop(this.root)
    this.state = this.readState()
    if (this.state.running) {
      this.setState({
        running: false,
        phase: "interrupted",
        detail: "上次安装被中断",
        error: "插件服务在安装完成前退出",
      })
    }
  }

  status() {
    return {
      ...this.state,
      installed: fs.existsSync(path.join(this.root, "prepared.json")),
      desktopPassword: this.desktop.credentials(),
      desktopError: this.desktop.error(),
    }
  }

  summary() {
    return {
      running: this.state.running,
      phase: this.state.phase,
      detail: this.state.detail,
      error: this.state.error,
      installed: fs.existsSync(path.join(this.root, "prepared.json")),
      updatedAt: this.state.updatedAt,
      desktopPassword: this.desktop.credentials(),
      desktopError: this.desktop.error(),
    }
  }

  install(config: QQPersonalConfig): void {
    if (this.state.running) return
    this.setState({ running: true, phase: "preparing", detail: "准备手机 QQ 运行环境", error: "" })
    void this.performInstall(config).catch(error => {
      const desktopError = this.desktop.error()
      this.setState({
        running: false,
        phase: "failed",
        detail: "安装失败",
        error: [error instanceof Error ? error.message : String(error), desktopError].filter(Boolean).join("："),
      })
      console.error("[QQ-Personal] 手机运行环境安装失败", error)
    })
  }

  private async performInstall(config: QQPersonalConfig): Promise<void> {
    await this.snowluma.stop()
    await this.desktop.stop()
    if (!await mobileEnvironmentReady(this.root)) {
      await prepareMobileEnvironment(this.root, (phase, detail) => {
        this.setState({ running: true, phase, detail, error: "" })
      })
    }
    await linkMobileEnvironment(this.root)
    fs.writeFileSync(path.join(this.root, "prepared.json"), JSON.stringify({ schemaVersion: 1 }), { mode: 0o600 })
    await this.ensureDesktop(config)
    this.setState({ running: true, phase: "snowluma", detail: "安装并启动 SnowLuma", error: "" })
    await this.snowluma.install(config)
    const deadline = Date.now() + 20 * 60 * 1000
    while (Date.now() < deadline) {
      const install = this.snowluma.installState()
      if (install.phase === "failed") throw new Error(install.error)
      if (install.phase === "completed" && this.snowluma.status().running) {
        try {
          const response = await fetch(`http://127.0.0.1:${config.webuiPort}/`, { signal: AbortSignal.timeout(2000) })
          await response.body?.cancel()
          if (response.ok || response.status === 401 || response.status === 403) {
            await this.attachSnowLumaHook(config)
            this.setState({ running: false, phase: "waiting_login", detail: "运行环境已启动，请打开 QQ 桌面登录", error: "" })
            return
          }
        } catch {}
      }
      await new Promise(resolve => setTimeout(resolve, 500))
    }
    throw new Error("等待 SnowLuma 安装启动超时")
  }

  async ensureDesktop(config: QQPersonalConfig): Promise<void> {
    if (config.runtimeMode !== "native" || !fs.existsSync(path.join(this.root, "prepared.json"))) return
    try {
      await this.desktop.start(config.novncPort)
      this.snowluma.setEnvironment(this.desktop.environment())
    } catch (error) {
      this.recordError(error)
      throw error
    }
  }

  async attachSnowLumaHook(config: QQPersonalConfig): Promise<void> {
    if (config.runtimeMode !== "native" || !this.snowluma.status().running || this.desktop.usesSnowLumaHook()) return
    await this.restartDesktopWithHook(config)
  }

  markWaitingLogin(): void {
    this.setState({ running: false, phase: "waiting_login", detail: "运行环境已启动，请打开 QQ 桌面登录", error: "" })
  }

  private snowlumaHookLibrary(config: QQPersonalConfig): string {
    const candidate = path.join(this.snowluma.nativeRoot(config), "native", `snowluma-${process.platform}-${process.arch}.so`)
    return fs.existsSync(candidate) ? candidate : ""
  }

  private async restartDesktopWithHook(config: QQPersonalConfig): Promise<void> {
    const hookLibrary = this.snowlumaHookLibrary(config)
    if (!hookLibrary) return
    await this.desktop.stop()
    await this.desktop.start(config.novncPort, { snowlumaHookLibrary: hookLibrary })
    this.snowluma.setEnvironment(this.desktop.environment())
  }

  recordError(error: unknown): void {
    const desktopError = this.desktop.error()
    this.setState({
      running: false,
      phase: "failed",
      detail: "手机 QQ 桌面启动失败",
      error: [error instanceof Error ? error.message : String(error), desktopError].filter(Boolean).join("："),
    })
  }

  async stop(): Promise<void> {
    await this.snowluma.stop()
    await this.desktop.stop()
  }

  private readState(): MobileRuntimeState {
    try {
      const parsed = JSON.parse(fs.readFileSync(this.stateFile, "utf8")) as Partial<MobileRuntimeState>
      return {
        running: parsed.running === true,
        phase: typeof parsed.phase === "string" && parsed.phase ? parsed.phase : INITIAL_STATE.phase,
        detail: typeof parsed.detail === "string" ? parsed.detail : "",
        error: typeof parsed.error === "string" ? parsed.error : "",
        updatedAt: typeof parsed.updatedAt === "string" ? parsed.updatedAt : "",
      }
    } catch {
      return { ...INITIAL_STATE }
    }
  }

  private setState(next: Omit<MobileRuntimeState, "updatedAt">): void {
    this.state = { ...next, updatedAt: new Date().toISOString() }
    fs.mkdirSync(this.root, { recursive: true, mode: 0o700 })
    const temporary = `${this.stateFile}.tmp`
    fs.writeFileSync(temporary, `${JSON.stringify(this.state, null, 2)}\n`, { mode: 0o600 })
    fs.renameSync(temporary, this.stateFile)
    try {
      fs.chmodSync(this.stateFile, 0o600)
    } catch {
      return
    }
  }
}
