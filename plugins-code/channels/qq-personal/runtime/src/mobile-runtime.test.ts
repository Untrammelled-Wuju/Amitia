import assert from "node:assert/strict"
import fs from "node:fs/promises"
import os from "node:os"
import path from "node:path"
import test from "node:test"
import type { QQPersonalConfig } from "./config.js"
import { MobileQQRuntime } from "./mobile-runtime.js"
import type { SnowLumaRuntimeManager } from "./snowluma-runtime.js"

const config: QQPersonalConfig = {
  enabled: true,
  listenHost: "127.0.0.1",
  listenPort: 6199,
  websocketPath: "/ws",
  token: "token",
  groupTrigger: "mention",
  triggerPrefix: "Amitia",
  ignoredUserIds: [],
  runtimeMode: "native",
  snowlumaRoot: "",
  webuiPort: 5099,
  novncPort: 6081,
  containerName: "snowluma",
  dockerBin: "",
}

test("SnowLuma hook is attached only after SnowLuma is running", async t => {
  const dataRoot = await fs.mkdtemp(path.join(os.tmpdir(), "qq-mobile-runtime-"))
  const snowlumaRoot = path.join(dataRoot, "snowluma")
  const mobileRoot = path.join(dataRoot, "mobile")
  await fs.mkdir(path.join(snowlumaRoot, "native"), { recursive: true })
  await fs.mkdir(mobileRoot, { recursive: true })
  await fs.writeFile(path.join(mobileRoot, "prepared.json"), "{}\n")
  await fs.writeFile(path.join(snowlumaRoot, "native", `snowluma-${process.platform}-${process.arch}.so`), "")
  t.after(() => fs.rm(dataRoot, { recursive: true, force: true }))

  const calls: Array<{ operation: string; options?: unknown }> = []
  let hookAttached = false
  const desktop = {
    async start(_port: number, options?: unknown) {
      calls.push({ operation: "start", options })
      hookAttached = Boolean((options as { snowlumaHookLibrary?: string } | undefined)?.snowlumaHookLibrary)
    },
    async stop() {
      calls.push({ operation: "stop" })
      hookAttached = false
    },
    environment() {
      return { DISPLAY: ":0" }
    },
    credentials() {
      return ""
    },
    error() {
      return ""
    },
    usesSnowLumaHook() {
      return hookAttached
    },
  }
  const snowluma = {
    status() {
      return { running: true }
    },
    nativeRoot() {
      return snowlumaRoot
    },
    setEnvironment() {},
  } as unknown as SnowLumaRuntimeManager
  const runtime = new MobileQQRuntime(dataRoot, snowluma)
  Object.defineProperty(runtime, "desktop", { value: desktop })

  await runtime.ensureDesktop(config)
  assert.equal(calls[0].options, undefined)
  await runtime.attachSnowLumaHook(config)
  assert.deepEqual(calls.map((call) => call.operation), ["start", "stop", "start"])
  assert.match(String((calls[2].options as { snowlumaHookLibrary?: string }).snowlumaHookLibrary), /snowluma-.*\.so$/)
  runtime.markWaitingLogin()
  assert.equal(runtime.status().phase, "waiting_login")
  assert.equal(runtime.status().error, "")
  assert.match(runtime.status().detail, /Hook 已预加载/)
})
