import assert from "node:assert/strict"
import fs from "node:fs/promises"
import os from "node:os"
import path from "node:path"
import test from "node:test"
import { MobileDesktop, mobileVncArguments } from "./mobile-desktop.js"

test("mobile VNC disables unavailable shared memory transport", () => {
  const args = mobileVncArguments(":0", "/tmp/pass")
  assert.ok(args.includes("-noshm"))
  assert.ok(args.includes("-passwdfile"))
  assert.ok(args.includes("rm:/tmp/pass"))
})

test("Xvfb receives the current identity compatibility layer", () => {
  const desktop = new MobileDesktop("/tmp/qq-display-env")
  const environment = desktop as unknown as { displayEnvironment(): NodeJS.ProcessEnv }
  assert.match(String(environment.displayEnvironment().LD_PRELOAD), /native\/libamitia-identity\.so/)
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
