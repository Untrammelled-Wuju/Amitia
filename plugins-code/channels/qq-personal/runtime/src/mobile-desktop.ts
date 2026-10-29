import { spawn, type ChildProcess } from "node:child_process"
import fs from "node:fs/promises"
import type { FileHandle } from "node:fs/promises"
import path from "node:path"
import net from "node:net"
import { randomBytes } from "node:crypto"
import { fileURLToPath } from "node:url"

const identityCompatLibrary = path.join(path.dirname(fileURLToPath(import.meta.url)), "native", "libamitia-identity.so")
const snowlumaHookLoaderLibrary = path.join(path.dirname(fileURLToPath(import.meta.url)), "native", "libamitia-snowluma-loader.so")

export function snowlumaHookRuntimeDirectory(uid = process.geteuid?.() ?? 0): string {
  return `/tmp/snowluma-${uid}`
}

export interface MobileDesktopOptions {
  snowlumaHookLibrary?: string
}

function shellQuote(value: string): string {
  return `'${value.replace(/'/g, `'\\''`)}'`
}

export function mobileQQWrapperScript(realBinary: string, loaderLibrary: string, hookLibrary: string, runtimeDirectory: string): string {
  return [
    "#!/bin/sh",
    "set -eu",
    'if [ "${AMITIA_SNOWLUMA_WRAPPER_DISABLE:-0}" = "1" ]; then',
    "  unset LD_PRELOAD AMITIA_SNOWLUMA_HOOK_LIBRARY AMITIA_SNOWLUMA_QQ_REAL_PATH AMITIA_SNOWLUMA_QQ_WRAPPER_PATH",
    `  exec ${shellQuote(realBinary)} "$@"`,
    "fi",
    `export AMITIA_SNOWLUMA_HOOK_LIBRARY=${shellQuote(hookLibrary)}`,
    `export SNOWLUMA_HOOK_RUNTIME_DIR=${shellQuote(runtimeDirectory)}`,
    `export AMITIA_SNOWLUMA_QQ_REAL_PATH=${shellQuote(realBinary)}`,
    `export AMITIA_SNOWLUMA_QQ_WRAPPER_PATH=${shellQuote(`${realBinary.slice(0, -".amitia-real".length)}`)}`,
    `if [ -n "\${LD_PRELOAD:-}" ]; then export LD_PRELOAD=${shellQuote(loaderLibrary)}:"$LD_PRELOAD"; else export LD_PRELOAD=${shellQuote(loaderLibrary)}; fi`,
    `exec ${shellQuote(realBinary)} "$@"`,
    "",
  ].join("\n")
}

export function isMobileQQWrapper(content: string): boolean {
  return content.startsWith("#!/bin/sh") && content.includes("AMITIA_SNOWLUMA_HOOK_LIBRARY")
}

export function isMobileQQMainCommandLine(cmdline: string): boolean {
  return cmdline.includes("/opt/QQ/qq") && !cmdline.includes("--type=")
}

export function mobileVncArguments(display: string, password: string): string[] {
  return ["-display", display, "-passwd", password, "-localhost", "-rfbport", "5900", "-forever", "-shared", "-noshm"]
}

export class MobileDesktop {
  private children: ChildProcess[] = []
  private password = ""
  private failure = ""
  private stopping = false
  private display = ":1"
  private snowlumaHookLibrary = ""

  constructor(private readonly root: string) {}

  environment(): NodeJS.ProcessEnv {
    const prefix = path.join(this.root, "root")
    return {
      ...process.env,
      HOME: path.join(this.root, "home"),
      PATH: [path.join(prefix, "usr/bin"), process.env.PATH].filter(Boolean).join(":"),
      DISPLAY: this.display,
      LD_LIBRARY_PATH: [path.join(prefix, "usr/lib/aarch64-linux-gnu"), path.join(prefix, "lib/aarch64-linux-gnu"), process.env.LD_LIBRARY_PATH].filter(Boolean).join(":"),
      XKB_CONFIG_ROOT: path.join(prefix, "usr/share/X11/xkb"),
      FONTCONFIG_PATH: path.join(prefix, "etc/fonts"),
      FONTCONFIG_FILE: path.join(this.root, "home", "fonts.conf"),
      PYTHONHOME: path.join(prefix, "usr"),
      PYTHONPATH: path.join(prefix, "usr/lib/python3/dist-packages"),
      SNOWLUMA_HOOK_RUNTIME_DIR: snowlumaHookRuntimeDirectory(),
    }
  }

  private displayEnvironment(): NodeJS.ProcessEnv {
    return {
      ...this.environment(),
      LD_PRELOAD: [identityCompatLibrary, process.env.LD_PRELOAD].filter(Boolean).join(":"),
    }
  }

  credentials(): string { return this.password }

  error(): string { return this.failure }

  async hookProcessIds(): Promise<number[]> {
    if (!this.snowlumaHookLibrary) return []
    const entries = await fs.readdir("/proc").catch(() => [])
    const runtime = snowlumaHookRuntimeDirectory()
    const pids: number[] = []
    for (const entry of entries) {
      if (!/^\d+$/.test(entry)) continue
      const cmdline = await fs.readFile(`/proc/${entry}/cmdline`, "utf8").catch(() => "")
      if (!isMobileQQMainCommandLine(cmdline)) continue
      try {
        await fs.access(path.join(runtime, `mojo.${entry}.control.sock`))
        pids.push(Number(entry))
      } catch {}
    }
    return pids.sort((left, right) => left - right)
  }

  async start(port: number, options: MobileDesktopOptions = {}): Promise<void> {
    if (this.children.length) return
    this.failure = ""
    this.stopping = false
    this.snowlumaHookLibrary = String(options.snowlumaHookLibrary || "")
    const prefix = path.join(this.root, "root")
    const home = path.join(this.root, "home")
    await fs.mkdir(home, { recursive: true, mode: 0o700 })
    const xml = (value: string) => value.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
    await fs.writeFile(path.join(home, "fonts.conf"), `<fontconfig><dir>${xml(path.join(prefix, "usr/share/fonts"))}</dir><cachedir>${xml(path.join(home, "font-cache"))}</cachedir></fontconfig>`, { mode: 0o600 })
    this.password = randomBytes(6).toString("hex").slice(0, 8)
    try {
      const vnc = path.join(prefix, "usr/bin/x11vnc")
      await this.verifyProcessIdentity(prefix)
      await this.verifyKeyboard(prefix, home)
      const displayNumber = await this.launchDisplayServer(path.join(prefix, "usr/bin/Xvfb"))
      this.display = `:${displayNumber}`
      await this.waitForFile(`/tmp/.X11-unix/X${displayNumber}`)
      const fluxboxConfig = path.join(home, "fluxbox-init")
      await fs.writeFile(fluxboxConfig, "session.screen0.toolbar.visible: false\nsession.screen0.workspaces: 1\n", { mode: 0o600 })
      this.launch(path.join(prefix, "usr/bin/fluxbox"), ["-display", this.display, "-rc", fluxboxConfig])
      this.launch(vnc, mobileVncArguments(this.display, this.password))
      await this.waitForPort(5900)
      this.launch(path.join(prefix, "usr/bin/python3"), [path.join(prefix, "usr/bin/websockify"), "--web", path.join(prefix, "usr/share/novnc"), `127.0.0.1:${port}`, "127.0.0.1:5900"])
      await this.waitForPort(port)
      const qqBinary = path.join(prefix, "opt/QQ/qq")
      const wrapped = isMobileQQWrapper(await this.readExecutableHead(qqBinary))
      const qqExecutable = this.snowlumaHookLibrary || wrapped
        ? await this.prepareQQWrapper(qqBinary)
        : qqBinary
      const qqEnvironment = this.environment()
      if (!this.snowlumaHookLibrary) qqEnvironment.AMITIA_SNOWLUMA_WRAPPER_DISABLE = "1"
      const qq = this.launch(qqExecutable, ["--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage"], true, qqEnvironment)
      if (this.snowlumaHookLibrary) {
        if (!qq.pid) throw new Error("手机 QQ 进程启动失败")
        await this.waitForHookSocket(qq.pid)
      }
      await new Promise(resolve => setTimeout(resolve, 1500))
      this.assertAlive()
    } catch (error) {
      await this.stop()
      throw error
    }
  }

  private async verifyProcessIdentity(prefix: string): Promise<void> {
    const script = [
      "import os",
      "print(f'before uid={os.getuid()} euid={os.geteuid()} gid={os.getgid()} egid={os.getegid()}')",
      "os.setgid(os.getgid())",
      "os.setuid(os.getuid())",
      "print(f'after uid={os.getuid()} euid={os.geteuid()} gid={os.getgid()} egid={os.getegid()}')",
    ].join("\n")
    await new Promise<void>((resolve, reject) => {
      const child = spawn(path.join(prefix, "usr/bin/python3"), ["-c", script], { env: this.displayEnvironment(), stdio: ["ignore", "pipe", "pipe"] })
      let detail = ""
      const record = (data: unknown) => { detail = `${detail}${String(data || "")}`.trim().slice(-4000) }
      child.stdout?.on("data", record)
      child.stderr?.on("data", record)
      const timer = setTimeout(() => { child.kill("SIGKILL"); reject(new Error(`进程身份检查超时：${detail}`)) }, 15000)
      child.once("error", error => { clearTimeout(timer); reject(error) })
      child.once("exit", code => {
        clearTimeout(timer)
        if (code === 0) resolve()
        else reject(new Error(`进程身份切换失败（${code}）：${detail}`))
      })
    })
  }

  private async verifyKeyboard(prefix: string, home: string): Promise<void> {
    const source = path.join(home, "keyboard.xkb")
    const output = path.join(home, "keyboard.xkm")
    await fs.writeFile(source, 'xkb_keymap { xkb_keycodes { include "evdev+aliases(qwerty)" }; xkb_types { include "complete" }; xkb_compatibility { include "complete" }; xkb_symbols { include "pc+us+inet(evdev)" }; xkb_geometry { include "pc(pc105)" }; };', { mode: 0o600 })
    const keymap = await fs.readFile(source)
    await new Promise<void>((resolve, reject) => {
      const child = spawn("/bin/sh", ["-c", 'exec "$1" -w 1 "-R$2" -xkm - "$3"', "amitia-xkb-check", "/usr/bin/xkbcomp", path.join(prefix, "usr/share/X11/xkb"), output], { env: this.environment(), stdio: ["pipe", "pipe", "pipe"] })
      child.stdin?.on("error", () => {})
      child.stdin?.end(keymap)
      let detail = ""
      const record = (data: unknown) => { detail = `${detail}${String(data || "")}`.slice(-4000) }
      child.stdout?.on("data", record)
      child.stderr?.on("data", record)
      const timer = setTimeout(() => { child.kill("SIGKILL"); reject(new Error("键盘映射编译检查超时")) }, 15000)
      child.once("error", error => { clearTimeout(timer); reject(error) })
      child.once("exit", code => { clearTimeout(timer); if (code === 0) resolve(); else reject(new Error(`键盘映射编译失败（${code}）：${detail}`)) })
    })
    await fs.access(output)
  }

  private launch(command: string, args: string[], recordStdout = true, environment = this.environment()): ChildProcess {
    const label = path.basename(command)
    const child = spawn(command, args, { env: environment, stdio: ["ignore", "pipe", "pipe"] })
    const record = (source: string) => (data: unknown) => {
      const text = String(data || "").trim()
      if (text) this.failure = `${this.failure}\n${label} ${source}: ${text}`.trim().slice(-6000)
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
    const child = this.launch(command, ["-xkbdir", path.join(this.root, "root/usr/share/X11/xkb"), "-displayfd", "1", "-screen", "0", "960x720x24", "-nolisten", "tcp"], false, this.displayEnvironment())
    return new Promise((resolve, reject) => {
      let output = ""
      let settled = false
      const finish = (error?: Error, display?: number) => {
        if (settled) return
        settled = true
        clearTimeout(timer)
        child.stdout?.removeListener("data", onData)
        if (error) reject(error)
        else resolve(display ?? 1)
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

  private async waitForHookSocket(pid: number): Promise<void> {
    const socket = path.join(snowlumaHookRuntimeDirectory(), `mojo.${pid}.control.sock`)
    const deadline = Date.now() + 20000
    while (Date.now() < deadline) {
      this.assertAlive()
      try {
        await fs.access(socket)
        return
      } catch {}
      await new Promise(resolve => setTimeout(resolve, 200))
    }
    throw new Error(`SnowLuma Hook 启动超时：${this.failure || socket}`)
  }

  private async prepareQQWrapper(qqBinary: string): Promise<string> {
    const realBinary = `${qqBinary}.amitia-real`
    const content = await this.readExecutableHead(qqBinary)
    if (!isMobileQQWrapper(content)) {
      await fs.copyFile(qqBinary, realBinary)
      const source = await fs.stat(qqBinary)
      await fs.chmod(realBinary, source.mode & 0o777)
    }
    const temporary = `${qqBinary}.amitia-tmp`
    await fs.writeFile(temporary, mobileQQWrapperScript(realBinary, snowlumaHookLoaderLibrary, this.snowlumaHookLibrary, snowlumaHookRuntimeDirectory()), { mode: 0o700 })
    await fs.rename(temporary, qqBinary)
    return qqBinary
  }

  private async readExecutableHead(file: string): Promise<string> {
    let handle: FileHandle | undefined
    try {
      handle = await fs.open(file, "r")
      const buffer = Buffer.alloc(256)
      const { bytesRead } = await handle.read(buffer, 0, buffer.length, 0)
      return buffer.subarray(0, bytesRead).toString("utf8")
    } catch {
      return ""
    } finally {
      await handle?.close().catch(() => {})
    }
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
    this.snowlumaHookLibrary = ""
    this.stopping = false
  }

  usesSnowLumaHook(): boolean {
    return Boolean(this.snowlumaHookLibrary)
  }
}
