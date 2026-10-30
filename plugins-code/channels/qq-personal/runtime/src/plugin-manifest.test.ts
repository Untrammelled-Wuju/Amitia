import assert from "node:assert/strict"
import fs from "node:fs"
import path from "node:path"
import test from "node:test"
import { fileURLToPath } from "node:url"

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..")
const manifest = JSON.parse(fs.readFileSync(path.join(root, "amitia-extension.json"), "utf8"))
const modules = (manifest.modules ?? []) as Array<Record<string, unknown>>
const ui = modules.find(module => module.id === "qq-personal-ui") as Record<string, unknown> | undefined
const contributions = ((ui?.contributions as Array<Record<string, unknown>> | undefined) ?? [])

test("QQ personal package exposes the desktop plugin page", () => {
  const desktop = contributions.find(contribution => contribution.id === "qq-personal-page")
  const mobile = contributions.find(contribution => contribution.id === "qq-personal-mobile-page")
  assert.ok(desktop)
  assert.equal(mobile, undefined)
  const desktopEntry = (((desktop.spec as Record<string, unknown>).entry as Record<string, unknown>))
  assert.equal(desktopEntry.type, "client_module")
  assert.deepEqual(manifest.compatibility.platforms, ["windows", "linux", "macos"])
})

test("desktop workspace tabs only target desktop entries", () => {
  const workspaceTabs = contributions.filter(contribution => {
    const spec = contribution.spec as Record<string, unknown>
    return spec.capability === "workspace.tab"
  })
  assert.equal(workspaceTabs.length, 2)
  const tabIds = new Set<string>()
  for (const tab of workspaceTabs) {
    const spec = tab.spec as Record<string, unknown>
    const entries = spec.entries as Record<string, Record<string, unknown>>
    const metadata = spec.metadata as Record<string, unknown>
    tabIds.add(String(metadata.tabId))
    assert.equal(entries.mobile, undefined)
    for (const platform of ["electron_windows", "electron_linux", "electron_macos"]) {
      assert.equal(entries[platform].type, "external_web_ui")
    }
  }
  assert.deepEqual([...tabIds].sort(), ["novnc-desktop", "snowluma-webui"])
  for (const tab of workspaceTabs) {
    const spec = tab.spec as Record<string, unknown>
    if ((spec.metadata as Record<string, unknown>).tabId !== "novnc-desktop") continue
    const linux = (spec.entries as Record<string, Record<string, unknown>>).electron_linux
    assert.match(String(linux.url), /\/vnc\.html$/)
  }
})
