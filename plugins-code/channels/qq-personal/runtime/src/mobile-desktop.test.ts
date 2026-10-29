import assert from "node:assert/strict"
import fs from "node:fs/promises"
import os from "node:os"
import path from "node:path"
import test from "node:test"
import { MobileDesktop, isMobileQQMainCommandLine, isMobileQQWrapper, mobileQQWrapperScript, mobileVncArguments, snowlumaHookRuntimeDirectory } from "./mobile-desktop.js"

test("mobile VNC disables unavailable shared memory transport", () => {
  const args = mobileVncArguments(":0", "12345678")
  assert.ok(args.includes("-noshm"))
  assert.ok(args.includes("-passwd"))
  assert.ok(args.includes("12345678"))
})

test("Xvfb receives the current identity compatibility layer", () => {
  const desktop = new MobileDesktop("/tmp/qq-display-env")
  const environment = desktop as unknown as { displayEnvironment(): NodeJS.ProcessEnv }
  assert.match(String(environment.displayEnvironment().LD_PRELOAD), /native\/libamitia-identity\.so/)
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

test("SnowLuma hook socket path is scoped to the runtime user", () => {
  assert.equal(snowlumaHookRuntimeDirectory(10233), "/tmp/snowluma-10233")
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
