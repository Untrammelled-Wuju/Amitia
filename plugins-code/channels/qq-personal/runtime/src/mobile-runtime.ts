import fs from "node:fs"
import path from "node:path"
import type { QQPersonalConfig } from "./config.js"
import { MobileDesktop } from "./mobile-desktop.js"
import { prepareMobileEnvironment, mobileEnvironmentReady } from "./mobile-environment.js"
import type { SnowLumaRuntimeManager } from "./snowluma-runtime.js"
import { linkMobileEnvironment } from "./mobile-links.js"
import { MOBILE_QQ_VERSION } from "./mobile-qq-package.js"
import { SNOWLUMA_VERSION } from "./snowluma-installer.js"

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
  private hookTask: Promise<void> | null = null

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
      desktopHooked: this.desktop.usesSnowLumaHook(),
      hookTargetDiagnostics: this.desktop.targetDiagnostics(),
    }
  }

  async hookProcessIds(): Promise<number[]> {
    return this.desktop.hookProcessIds()
  }

  async inspectHookTargets(): Promise<string> {
    return this.desktop.inspectHookTargets()
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
    fs.writeFileSync(path.join(this.root, "prepared.json"), JSON.stringify({ schemaVersion: 1, qqVersion: MOBILE_QQ_VERSION }), { mode: 0o600 })
    await this.ensureDesktop(config)
    this.setState({ running: true, phase: "snowluma", detail: "安装并启动 SnowLuma", error: "" })
    const existing = this.snowluma.installState()
    const needsInstall = !existing.installed || existing.installedVersion !== SNOWLUMA_VERSION
    if (needsInstall) await this.snowluma.install(config)
    else await this.snowluma.start(config)
    const deadline = Date.now() + 20 * 60 * 1000
    while (Date.now() < deadline) {
      const install = this.snowluma.installState()
      if (needsInstall && install.phase === "failed") throw new Error(install.error)
      if ((!needsInstall || install.phase === "completed") && this.snowluma.status().running) {
        try {
          const response = await fetch(`http://127.0.0.1:${config.webuiPort}/`, { signal: AbortSignal.timeout(2000) })
          await response.body?.cancel()
          if (response.ok || response.status === 401 || response.status === 403) {
            await this.attachSnowLumaHook(config)
            this.markWaitingLogin()
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
      this.snowluma.setEnvironment({ ...this.desktop.snowlumaEnvironment(), SNOWLUMA_HOOK_AUTOLOAD: "0" })
    } catch (error) {
      this.recordError(error)
      throw error
    }
  }

  async attachSnowLumaHook(config: QQPersonalConfig): Promise<void> {
    if (config.runtimeMode !== "native" || !this.snowluma.status().running) return
    if (!this.desktop.usesSnowLumaHook()) await this.restartDesktopWithHook(config)
    this.armSnowLumaHook()
  }

  markWaitingLogin(): void {
    const detail = this.desktop.usesSnowLumaHook()
      ? "等待 QQ 登录，服务就绪后自动加载 SnowLuma"
      : "运行环境已启动，请打开 QQ 桌面登录"
    this.setState({ running: false, phase: "waiting_login", detail, error: "" })
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
    this.snowluma.setEnvironment({ ...this.desktop.snowlumaEnvironment(), SNOWLUMA_HOOK_AUTOLOAD: "0" })
  }

  private armSnowLumaHook(): void {
    if (this.hookTask) return
    this.hookTask = this.desktop.startSnowLumaHook()
      .catch(error => {
        this.setState({
          running: false,
          phase: "failed",
          detail: "SnowLuma Hook 加载失败",
          error: error instanceof Error ? error.message : String(error),
        })
      })
      .finally(() => {
        this.hookTask = null
      })
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
