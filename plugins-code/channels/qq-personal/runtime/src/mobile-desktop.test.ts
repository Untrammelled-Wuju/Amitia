import assert from "node:assert/strict"
import fs from "node:fs/promises"
import os from "node:os"
import path from "node:path"
import net from "node:net"
import { once } from "node:events"
import { createRequire } from "node:module"
import test from "node:test"
import { MobileDesktop, isMobileQQMainCommandLine, isMobileQQWrapper, mobileQQArguments, mobileQQWrapperScript, mobileVncArguments, snowlumaHookRuntimeDirectory } from "./mobile-desktop.js"

const require = createRequire(import.meta.url)
const procLocks = require("../../native/linux-arm64/amitia-proc-locks.cjs") as {
  deviceInodeKey(device: number, inode: number): string
  syntheticProcessEnvironment(file: string): string | null
}

test("mobile VNC disables unavailable shared memory transport", () => {
  const args = mobileVncArguments(":0", "12345678")
  assert.ok(args.includes("-noshm"))
  assert.ok(args.includes("-passwd"))
  assert.ok(args.includes("12345678"))
})

test("mobile QQ exposes only the loopback inspector", () => {
  assert.ok(mobileQQArguments().includes("--inspect=127.0.0.1:9231"))
})

test("Xvfb receives the current identity compatibility layer", () => {
  const desktop = new MobileDesktop("/tmp/qq-display-env")
  const environment = desktop as unknown as { displayEnvironment(): NodeJS.ProcessEnv }
  assert.match(String(environment.displayEnvironment().LD_PRELOAD), /native\/libamitia-identity\.so/)
})

test("SnowLuma receives the process memory compatibility layer", () => {
  const desktop = new MobileDesktop("/tmp/qq-snowluma-env")
  assert.match(String(desktop.snowlumaEnvironment().LD_PRELOAD), /native\/libamitia-snowluma-loader\.so/)
})

test("QQ wrapper reloads the SnowLuma hook for every client process", () => {
  const script = mobileQQWrapperScript(
    "/opt/QQ/qq.amitia-real",
    "/service/native/libamitia-snowluma-loader.so",
    "/service/snowluma/native/snowluma-linux-arm64.so",
    "/tmp/snowluma-10233",
  )
  assert.match(script, /export LD_PRELOAD='\/service\/native\/libamitia-snowluma-loader\.so'/)
  assert.match(script, /export AMITIA_SNOWLUMA_HOOK_LIBRARY='\/service\/snowluma\/native\/snowluma-linux-arm64\.so'/)
  assert.match(script, /export SNOWLUMA_HOOK_RUNTIME_DIR='\/tmp\/snowluma-10233'/)
  assert.match(script, /export AMITIA_SNOWLUMA_QQ_REAL_PATH='\/opt\/QQ\/qq\.amitia-real'/)
  assert.match(script, /export AMITIA_SNOWLUMA_QQ_WRAPPER_PATH='\/opt\/QQ\/qq'/)
  assert.match(script, /unset SNOWLUMA_HOOK_SERVICE_MODE/)
  assert.match(script, /export AMITIA_SNOWLUMA_PROC_MEM=1/)
  assert.match(script, /export AMITIA_SNOWLUMA_HOOK_AUTOSTART=1/)
  assert.match(script, /export AMITIA_SNOWLUMA_HOOK_START_DELAY_MS=0/)
  assert.match(script, /export AMITIA_SNOWLUMA_HOOK_START_ON_WRAPPER=0/)
  assert.match(script, /export AMITIA_SNOWLUMA_PIPE_BRIDGE=1/)
  assert.match(script, /export AMITIA_SNOWLUMA_LOGIN_STATE_DELAY_MS=5000/)
  assert.match(script, /export AMITIA_SNOWLUMA_RECEIVE_PROBE=0/)
  assert.match(script, /AMITIA_SNOWLUMA_WRAPPER_DISABLE:-0/)
  assert.match(script, /exec '\/opt\/QQ\/qq\.amitia-real' "\$@"/)
  assert.equal(isMobileQQWrapper(script), true)
  assert.equal(isMobileQQWrapper("\u007fELF"), false)
})

test("QQ hook detection ignores Electron child processes", () => {
  assert.equal(isMobileQQMainCommandLine("/opt/QQ/qq\u0000--no-sandbox\u0000"), true)
  assert.equal(isMobileQQMainCommandLine("/opt/QQ/qq\u0000--type=gpu-process\u0000"), false)
  assert.equal(isMobileQQMainCommandLine("/usr/bin/not-qq\u0000"), false)
})

test("proc locks compatibility key matches the Linux device and inode format", () => {
  assert.equal(procLocks.deviceInodeKey(0x1234, 77), "12:34:77")
})

test("proc locks compatibility scopes process environment to the QQ home", () => {
  const previous = process.env.HOME
  process.env.HOME = "/home/amitia"
  try {
    assert.equal(procLocks.syntheticProcessEnvironment("/proc/123/environ"), "HOME=/home/amitia\0XDG_CONFIG_HOME=/home/amitia/.config\0")
    assert.equal(procLocks.syntheticProcessEnvironment("/proc/123/status"), null)
  } finally {
    if (previous === undefined) delete process.env.HOME
    else process.env.HOME = previous
  }
})

test("SnowLuma hook socket path is scoped to the runtime user", () => {
  assert.equal(snowlumaHookRuntimeDirectory(10233), "/tmp/snowluma-10233")
})

test("QQ identity hints cannot synthesize protocol login events", async t => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), "qq-hint-"))
  const socketPath = path.join(root, "mojo.123.control.sock")
  const server = net.createServer(socket => socket.on("data", () => {}))
  server.listen(socketPath)
  await once(server, "listening")
  const socket = net.createConnection(socketPath)
  t.after(async () => {
    socket.destroy()
    await new Promise<void>(resolve => server.close(() => resolve()))
    await fs.rm(root, { recursive: true, force: true })
  })
  await once(socket, "connect")
  const received: Buffer[] = []
  socket.on("data", data => received.push(Buffer.from(data)))
  const hint = Buffer.alloc(40)
  hint.writeUInt32LE(0x31504851, 0)
  hint.writeUInt16LE(1, 4)
  hint.writeUInt16LE(17, 6)
  hint.writeBigUInt64LE(123456789n, 32)
  t.mock.timers.enable({ apis: ["setTimeout"] })
  socket.write(hint)
  t.mock.timers.tick(11000)
  assert.equal(received.length, 0)
  t.mock.timers.reset()
})

test("Xvfb display zero is retained from its readiness pipe", async t => {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), "qq-display-"))
  const executable = path.join(directory, "Xvfb")
  await fs.writeFile(executable, "#!/bin/sh\nprintf '0\\n'\nexec sleep 2\n", { mode: 0o700 })
  const desktop = new MobileDesktop(directory)
  t.after(async () => { await desktop.stop(); await fs.rm(directory, { recursive: true, force: true }) })
  const launcher = desktop as unknown as { launchDisplayServer(command: string): Promise<number> }
  assert.equal(await launcher.launchDisplayServer(executable), 0)
})
