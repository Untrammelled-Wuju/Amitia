import assert from "node:assert/strict"
import fs from "node:fs/promises"
import os from "node:os"
import path from "node:path"
import test from "node:test"
import { MobileDesktop, mobileVncArguments, snowlumaHookRuntimeDirectory } from "./mobile-desktop.js"

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

test("QQ receives the SnowLuma startup hook environment without changing Xvfb", () => {
  const desktop = new MobileDesktop("/tmp/qq-display-env")
  const internals = desktop as unknown as {
    snowlumaHookLibrary: string
    qqEnvironment(): NodeJS.ProcessEnv
    displayEnvironment(): NodeJS.ProcessEnv
  }
  internals.snowlumaHookLibrary = "/opt/snowluma/native/snowluma-linux-arm64.so"
  const environment = internals.qqEnvironment()
  assert.match(String(environment.LD_PRELOAD), /native\/libamitia-snowluma-loader\.so/)
  assert.equal(environment.AMITIA_SNOWLUMA_HOOK_LIBRARY, internals.snowlumaHookLibrary)
  assert.equal(environment.AMITIA_SNOWLUMA_HOOK_PARENT_PID, String(process.pid))
  assert.match(String(environment.SNOWLUMA_HOOK_RUNTIME_DIR), /^\/tmp\/snowluma-\d+$/)
  assert.doesNotMatch(String(internals.displayEnvironment().LD_PRELOAD), /libamitia-snowluma-loader/)
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
