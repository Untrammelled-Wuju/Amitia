import { execFile, spawn, type ChildProcess } from "node:child_process"
import fs from "node:fs/promises"
import type { FileHandle } from "node:fs/promises"
import path from "node:path"
import net from "node:net"
import { randomBytes } from "node:crypto"
import { fileURLToPath } from "node:url"

const identityCompatLibrary = path.join(path.dirname(fileURLToPath(import.meta.url)), "native", "libamitia-identity.so")
const snowlumaHookLoaderLibrary = path.join(path.dirname(fileURLToPath(import.meta.url)), "native", "libamitia-snowluma-loader.so")
export const MOBILE_QQ_INSPECT_PORT = 9231

export function snowlumaHookRuntimeDirectory(uid = process.geteuid?.() ?? 0): string {
  return `/tmp/snowluma-${uid}`
}

export interface MobileDesktopOptions {
  snowlumaHookLibrary?: string
  nativeInjection?: boolean
}

function shellQuote(value: string): string {
  return `'${value.replace(/'/g, `'\\''`)}'`
}

export function mobileQQWrapperScript(realBinary: string, loaderLibrary: string, hookLibrary: string, runtimeDirectory: string): string {
  return [
    "#!/bin/sh",
    "set -eu",
    'if [ "${AMITIA_SNOWLUMA_WRAPPER_DISABLE:-0}" = "1" ]; then',
    "  unset AMITIA_SNOWLUMA_HOOK_LIBRARY AMITIA_SNOWLUMA_QQ_REAL_PATH AMITIA_SNOWLUMA_QQ_WRAPPER_PATH",
    `  exec ${shellQuote(realBinary)} "$@"`,
    "fi",
    `export AMITIA_SNOWLUMA_HOOK_LIBRARY=${shellQuote(hookLibrary)}`,
    `export SNOWLUMA_HOOK_RUNTIME_DIR=${shellQuote(runtimeDirectory)}`,
    `export AMITIA_SNOWLUMA_QQ_REAL_PATH=${shellQuote(realBinary)}`,
    `export AMITIA_SNOWLUMA_QQ_WRAPPER_PATH=${shellQuote(`${realBinary.slice(0, -".amitia-real".length)}`)}`,
    "unset SNOWLUMA_HOOK_SERVICE_MODE",
    "export AMITIA_SNOWLUMA_HOOK_AUTOSTART=1",
    "export AMITIA_SNOWLUMA_HOOK_START_DELAY_MS=0",
    "export AMITIA_SNOWLUMA_HOOK_START_ON_WRAPPER=0",
    "export AMITIA_SNOWLUMA_PIPE_BRIDGE=1",
    "export AMITIA_SNOWLUMA_LOGIN_STATE_DELAY_MS=5000",
    "export AMITIA_SNOWLUMA_PROC_MEM=1",
    "export AMITIA_SNOWLUMA_RECEIVE_PROBE=0",
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

export function mobileQQArguments(inspectPort = MOBILE_QQ_INSPECT_PORT): string[] {
  return [`--inspect=127.0.0.1:${inspectPort}`, "--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage"]
}

export class MobileDesktop {
  private children: ChildProcess[] = []
  private password = ""
  private failure = ""
  private stopping = false
  private display = ":1"
  private snowlumaHookLibrary = ""
  private hookTargetDiagnostics = ""

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

  snowlumaEnvironment(): NodeJS.ProcessEnv {
    return {
      ...this.environment(),
      LD_PRELOAD: [snowlumaHookLoaderLibrary, process.env.LD_PRELOAD].filter(Boolean).join(":"),
      SNOWLUMA_LOG_LEVEL: "trace",
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

  targetDiagnostics(): string { return this.hookTargetDiagnostics }

  async inspectHookTargets(): Promise<string> {
    const [pid] = [...await this.hookProcessIds(), ...await this.qqProcessIds()]
    if (!pid) return "QQ Hook 尚未启动"
    const result = await this.resolveHookTargets(pid)
    this.hookTargetDiagnostics = result.diagnostics
    return result.diagnostics
  }

  private async resolveHookTargets(pid: number): Promise<{ ready: boolean; serviceReady: boolean; diagnostics: string }> {
    const addon = this.snowlumaHookLibrary.replace(/\.so$/, ".node")
    const script = `
const mod = { exports: {} };
process.dlopen(mod, process.argv[1]);
const pid = Number(process.argv[2]);
const encode = value => JSON.stringify(value, (_key, item) => typeof item === "bigint" ? "0x" + item.toString(16) : item);
const readWords = async address => {
  try {
    const fs = await import("node:fs");
    const fd = fs.openSync(\`/proc/\${pid}/mem\`, "r");
    const buffer = Buffer.alloc(512);
    fs.readSync(fd, buffer, 0, buffer.length, Number(BigInt(address)));
    fs.closeSync(fd);
    return Array.from({ length: 64 }, (_item, index) => "0x" + buffer.readBigUInt64LE(index * 8).toString(16));
  } catch (error) {
    return [String(error?.code || error?.message || error)];
  }
};
(async () => {
  try {
    const [value, inspection] = await Promise.all([
      mod.exports.resolveRuntimeTargetsAsync(pid),
      typeof mod.exports.inspectProcessAsync === "function" ? mod.exports.inspectProcessAsync(pid) : null,
    ]);
    const arrays = Object.fromEntries(Object.entries(value).filter(([_key, item]) => Array.isArray(item)).map(([key, item]) => [key, item.slice(0, 32).map(String)]));
    const functions = Object.fromEntries(await Promise.all(Object.entries({
      sendSsoInternal: value.sendSsoInternal,
      msfDidRecvResponseData: value.msfDidRecvResponseData,
      msfGetStatus: value.msfGetStatus,
      selfInfoOnLoginSuccess: value.selfInfoOnLoginSuccess,
      onMSFPacketState: BigInt(value.wrapperBase) + 0x13a3e80n,
    }).map(async ([key, address]) => [key, await readWords(address)])));
    const memory = Object.fromEntries(await Promise.all(Object.entries(arrays).map(async ([key, addresses]) => [
      key,
      Object.fromEntries(await Promise.all(addresses.map(async address => [address, await readWords(address)]))),
    ])));
    console.log(encode({
      resolved: true,
      targets: Object.keys(value),
      candidates: Object.fromEntries(Object.entries(value).filter(([_key, item]) => Array.isArray(item)).map(([key, item]) => [key, item.length])),
      arrays,
      functions,
      memory,
      details: {
        pid: String(value.pid),
        modulePath: String(value.modulePath),
        wrapperBase: String(value.wrapperBase),
        sendSsoInternal: String(value.sendSsoInternal),
        msfDidRecvResponseData: String(value.msfDidRecvResponseData),
        msfGetStatus: String(value.msfGetStatus),
        selfInfoOnLoginSuccess: String(value.selfInfoOnLoginSuccess),
        msfServiceTypeinfoName: String(value.msfServiceTypeinfoName),
        msfServiceTypeinfo: String(value.msfServiceTypeinfo),
        msfCoreTypeinfo: String(value.msfCoreTypeinfo),
        msfCoreCallbackOffset: String(value.msfCoreCallbackOffset),
      },
      inspection,
    }));
  } catch (error) {
    console.log(JSON.stringify({ resolved: false, error: error.message }));
    process.exitCode = 1;
  }
})();
`
    return new Promise(resolve => {
      execFile(process.execPath, ["--input-type=module", "--eval", script, addon, String(pid)], {
        env: this.snowlumaEnvironment(), timeout: 30000, maxBuffer: 4 * 1024 * 1024,
      }, (error, stdout) => {
        const rawDiagnostics = stdout.trim()
        let diagnostics = rawDiagnostics || error?.message || "诊断未返回结果"
        let ready = false
        let serviceReady = false
        try {
          const parsed = JSON.parse(rawDiagnostics) as {
            resolved?: boolean
            targets?: string[]
            candidates?: Record<string, number>
            arrays?: Record<string, string[]>
            functions?: Record<string, string[]>
            memory?: Record<string, Record<string, string[]>>
            details?: Record<string, string>
            inspection?: Record<string, unknown>
          }
          serviceReady = parsed.resolved === true && Number(parsed.candidates?.msfServiceCandidates || 0) > 0
          ready = parsed.resolved === true
            && Number(parsed.candidates?.msfServiceCandidates || 0) > 0
            && Number(parsed.candidates?.msfCoreCandidates || 0) > 0
          const symbols = Array.isArray(parsed.inspection?.dynamicSymbols)
            ? parsed.inspection.dynamicSymbols.filter((item) => {
              const value = item && typeof item === "object" ? item as Record<string, unknown> : {}
              return /MSF|TaskManager|Packet|SessionInit/i.test(`${value.name || ""} ${value.demangled || ""}`)
            })
            : []
          diagnostics = JSON.stringify({
            resolved: parsed.resolved,
            targets: parsed.targets,
            candidates: parsed.candidates,
            arrays: parsed.arrays,
            functions: parsed.functions,
            memory: parsed.memory,
            details: parsed.details,
            inspection: parsed.inspection ? {
              keys: Object.keys(parsed.inspection),
              dynamicSymbols: symbols,
            } : null,
          })
        } catch {}
        resolve({ ready, serviceReady, diagnostics })
      })
    })
  }

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

  async qqProcessIds(): Promise<number[]> {
    const entries = await fs.readdir("/proc").catch(() => [])
    const pids: number[] = []
    for (const entry of entries) {
      if (!/^\d+$/.test(entry)) continue
      const cmdline = await fs.readFile(`/proc/${entry}/cmdline`, "utf8").catch(() => "")
      if (isMobileQQMainCommandLine(cmdline)) pids.push(Number(entry))
    }
    return pids.sort((left, right) => left - right)
  }

  async startSnowLumaHook(): Promise<void> {
    const deadline = Date.now() + 2 * 60 * 1000
    while (Date.now() < deadline) {
      this.assertAlive()
      const [hookPid] = await this.hookProcessIds()
      if (hookPid) return
      const [qqPid] = await this.qqProcessIds()
      if (!qqPid) throw new Error("手机 QQ 主进程已退出")
      await this.waitForHookSocket(qqPid)
      return
    }
    throw new Error("等待 SnowLuma Hook 启动超时")
  }

  async start(port: number, options: MobileDesktopOptions = {}): Promise<void> {
    if (this.children.length) return
    this.failure = ""
    this.hookTargetDiagnostics = ""
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
      if (!this.snowlumaHookLibrary || options.nativeInjection) qqEnvironment.AMITIA_SNOWLUMA_WRAPPER_DISABLE = "1"
      if (options.nativeInjection) qqEnvironment.LD_PRELOAD = snowlumaHookLoaderLibrary
      const qq = this.launch(qqExecutable, mobileQQArguments(), true, qqEnvironment)
      if (this.snowlumaHookLibrary && options.nativeInjection) {
        if (!qq.pid) throw new Error("手机 QQ 进程启动失败")
        await this.injectOfficialHook(qq.pid)
        await this.waitForHookSocket(qq.pid)
      }
      await new Promise(resolve => setTimeout(resolve, 1500))
      this.assertAlive()
    } catch (error) {
      await this.stop()
      throw error
    }
  }

  private async injectOfficialHook(pid: number): Promise<void> {
    const deadline = Date.now() + 30000
    let mapped = false
    while (Date.now() < deadline) {
      this.assertAlive()
      const maps = await fs.readFile(`/proc/${pid}/maps`, "utf8")
      if (maps.includes("wrapper.node")) { mapped = true; break }
      await new Promise(resolve => setTimeout(resolve, 200))
    }
    if (!mapped) throw new Error("等待 QQ 原生模块加载超时")
    const script = 'const mod = { exports: {} }; process.dlopen(mod, process.argv[1]); await mod.exports.loadModuleManualAsync(Number(process.argv[2]), process.argv[3]); console.log("official injection completed")'
    await new Promise<void>((resolve, reject) => {
      execFile(process.execPath, ["--input-type=module", "--eval", script, this.snowlumaHookLibrary.replace(/\.so$/, ".node"), String(pid), this.snowlumaHookLibrary], {
        env: { ...this.snowlumaEnvironment(), AMITIA_PTRACE_SEIZE_COMPAT: "1", AMITIA_SNOWLUMA_PROC_MEM: "1" },
        timeout: 30000, maxBuffer: 16000,
      }, (error, stdout, stderr) => {
        this.hookTargetDiagnostics = `${stdout}\n${stderr}`.trim().slice(-4000)
        if (error) reject(new Error(`官方 Hook 加载失败：${this.hookTargetDiagnostics || error.message}`))
        else resolve()
      })
    })
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
    const deadline = Date.now() + 45000
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
