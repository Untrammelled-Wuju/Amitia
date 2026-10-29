import { spawn, type ChildProcess } from "node:child_process"
import fs from "node:fs/promises"
import path from "node:path"
import net from "node:net"
import { randomBytes } from "node:crypto"

export class MobileDesktop {
  private children: ChildProcess[] = []
  private password = ""
  private failure = ""
  private stopping = false
  private display = ":1"

  constructor(private readonly root: string) {}

  environment(): NodeJS.ProcessEnv {
    const prefix = path.join(this.root, "root")
    return {
      ...process.env,
      HOME: path.join(this.root, "home"),
      DISPLAY: this.display,
      LD_LIBRARY_PATH: [path.join(prefix, "usr/lib/aarch64-linux-gnu"), path.join(prefix, "lib/aarch64-linux-gnu"), process.env.LD_LIBRARY_PATH].filter(Boolean).join(":"),
      XKB_CONFIG_ROOT: path.join(prefix, "usr/share/X11/xkb"),
      FONTCONFIG_PATH: path.join(prefix, "etc/fonts"),
      FONTCONFIG_FILE: path.join(this.root, "home", "fonts.conf"),
      PYTHONHOME: path.join(prefix, "usr"),
      PYTHONPATH: path.join(prefix, "usr/lib/python3/dist-packages"),
    }
  }

  credentials(): string { return this.password }

  error(): string { return this.failure }

  async start(port: number): Promise<void> {
    if (this.children.length) return
    this.failure = ""
    this.stopping = false
    const prefix = path.join(this.root, "root")
    const home = path.join(this.root, "home")
    await fs.mkdir(home, { recursive: true, mode: 0o700 })
    const xml = (value: string) => value.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
    await fs.writeFile(path.join(home, "fonts.conf"), `<fontconfig><dir>${xml(path.join(prefix, "usr/share/fonts"))}</dir><cachedir>${xml(path.join(home, "font-cache"))}</cachedir></fontconfig>`, { mode: 0o600 })
    this.password = randomBytes(6).toString("hex").slice(0, 8)
    const passwordFile = path.join(home, ".vnc-password")
    try {
      const vnc = path.join(prefix, "usr/bin/x11vnc")
      await new Promise<void>((resolve, reject) => {
        const child = spawn(vnc, ["-storepasswd", this.password, passwordFile], { env: this.environment(), stdio: "ignore" })
        child.once("error", reject)
        child.once("exit", code => code === 0 ? resolve() : reject(new Error("创建手机 QQ 桌面凭据失败")))
      })
      await fs.chmod(passwordFile, 0o600)
      const displayNumber = await this.launchDisplayServer(path.join(prefix, "usr/bin/Xvfb"))
      this.display = `:${displayNumber}`
      await this.waitForFile(`/tmp/.X11-unix/X${displayNumber}`)
      const fluxboxConfig = path.join(home, "fluxbox-init")
      await fs.writeFile(fluxboxConfig, "session.screen0.toolbar.visible: false\nsession.screen0.workspaces: 1\n", { mode: 0o600 })
      this.launch(path.join(prefix, "usr/bin/fluxbox"), ["-display", this.display, "-rc", fluxboxConfig])
      this.launch(vnc, ["-display", this.display, "-rfbauth", passwordFile, "-localhost", "-rfbport", "5900", "-forever", "-shared"])
      await this.waitForPort(5900)
      this.launch(path.join(prefix, "usr/bin/python3"), [path.join(prefix, "usr/bin/websockify"), "--web", path.join(prefix, "usr/share/novnc"), `127.0.0.1:${port}`, "127.0.0.1:5900"])
      await this.waitForPort(port)
      this.launch(path.join(prefix, "opt/QQ/qq"), ["--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage"])
      await new Promise(resolve => setTimeout(resolve, 1500))
      this.assertAlive()
    } catch (error) {
      await this.stop()
      throw error
    }
  }

  private launch(command: string, args: string[], recordStdout = true): ChildProcess {
    const label = path.basename(command)
    const child = spawn(command, args, { env: this.environment(), stdio: ["ignore", "pipe", "pipe"] })
    const record = (source: string) => (data: unknown) => {
      const text = String(data || "").trim()
      if (text) this.failure = `${label} ${source}: ${text}`.slice(-2000)
    }
    if (recordStdout) child.stdout?.on("data", record("stdout"))
    child.stderr?.on("data", record("stderr"))
    child.on("error", error => { this.failure = `${label}: ${error.message}` })
    child.once("exit", (code, signal) => {
      if (!this.stopping) this.failure = `${this.failure ? `${this.failure}；` : ""}${label} 已退出（${signal || `code ${code ?? "unknown"}`}）`.slice(-2000)
    })
    this.children.push(child)
    return child
  }

  private launchDisplayServer(command: string): Promise<number> {
    const child = this.launch(command, ["-displayfd", "1", "-screen", "0", "960x720x24", "-nolisten", "tcp"], false)
    return new Promise((resolve, reject) => {
      let output = ""
      let settled = false
      const finish = (error?: Error, display?: number) => {
        if (settled) return
        settled = true
        clearTimeout(timer)
        child.stdout?.removeListener("data", onData)
        if (error) reject(error)
        else resolve(display || 1)
      }
      const onData = (data: unknown) => {
        output = `${output}${String(data || "")}`.slice(-2000)
        for (const line of output.split(/\r?\n/)) {
          const value = line.trim()
          if (/^\d+$/.test(value)) finish(undefined, Number(value))
        }
      }
      const timer = setTimeout(() => finish(new Error("手机 QQ 图形显示服务启动超时")), 15000)
      child.stdout?.on("data", onData)
      child.once("error", error => finish(error))
      child.once("exit", (code, signal) => finish(new Error(`${path.basename(command)} 已退出（${signal || `code ${code ?? "unknown"}`}）${output.trim() ? `：${output.trim()}` : ""}`)))
    })
  }

  private async waitForFile(file: string): Promise<void> {
    const deadline = Date.now() + 15000
    while (Date.now() < deadline) {
      this.assertAlive()
      try { await fs.access(file); return } catch {}
      await new Promise(resolve => setTimeout(resolve, 200))
    }
    throw new Error("手机 QQ 图形服务启动超时")
  }

  private async waitForPort(port: number): Promise<void> {
    const deadline = Date.now() + 15000
    while (Date.now() < deadline) {
      this.assertAlive()
      const ready = await new Promise<boolean>(resolve => {
        const socket = net.createConnection({ host: "127.0.0.1", port })
        socket.setTimeout(500)
        socket.once("connect", () => { socket.destroy(); resolve(true) })
        socket.once("error", () => { socket.destroy(); resolve(false) })
        socket.once("timeout", () => { socket.destroy(); resolve(false) })
      })
      if (ready) return
      await new Promise(resolve => setTimeout(resolve, 200))
    }
    throw new Error(`手机 QQ 服务 ${port} 启动超时`)
  }

  private assertAlive(): void {
    if (this.children.some(child => !child.pid || child.exitCode !== null)) throw new Error(`手机 QQ 图形服务提前退出：${this.failure}`)
  }

  async stop(): Promise<void> {
    this.stopping = true
    for (const child of this.children.splice(0).reverse()) {
      if (child.exitCode !== null) continue
      const exited = new Promise<void>(resolve => {
        const timer = setTimeout(() => { child.kill("SIGKILL"); resolve() }, 2000)
        child.once("exit", () => { clearTimeout(timer); resolve() })
      })
      child.kill("SIGTERM")
      await exited
    }
    this.password = ""
    this.stopping = false
  }
}
