import assert from "node:assert/strict"
import fs from "node:fs"
import os from "node:os"
import path from "node:path"
import test from "node:test"
import { napcatWebUIConfigCandidates, readNapCatWebUIInfo } from "./napcat-webui.js"

test("reads the NapCat WebUI token from a mounted config", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "qq-personal-webui-"))
  const file = path.join(root, "webui.json")
  fs.writeFileSync(file, JSON.stringify({ token: "webui-token" }))
  try {
    assert.deepEqual(readNapCatWebUIInfo([file]), { token: "webui-token", error: "" })
  } finally {
    fs.rmSync(root, { recursive: true, force: true })
  }
})

test("reports a missing WebUI config", () => {
  const result = readNapCatWebUIInfo(["/path/that/does/not/exist/webui.json"])
  assert.equal(result.token, "")
  assert.match(result.error, /未找到/)
})

test("prefers explicit configuration paths and expands home", () => {
  const home = "/tmp/qq-personal-home"
  const candidates = napcatWebUIConfigCandidates(
    { QQ_PERSONAL_NAPCAT_WEBUI_CONFIG: "~/custom/webui.json" },
    home,
  )
  assert.equal(candidates[0], path.join(home, "custom/webui.json"))
  assert.ok(candidates.includes(path.join(home, "napcat-docker/data/config/webui.json")))
})
