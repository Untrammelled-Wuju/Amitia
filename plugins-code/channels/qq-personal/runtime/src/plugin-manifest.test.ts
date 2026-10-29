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

test("QQ personal package exposes desktop and mobile plugin pages", () => {
  const desktop = contributions.find(contribution => contribution.id === "qq-personal-page")
  const mobile = contributions.find(contribution => contribution.id === "qq-personal-mobile-page")
  assert.ok(desktop)
  assert.ok(mobile)
  const desktopEntry = (((desktop.spec as Record<string, unknown>).entry as Record<string, unknown>))
  const mobileEntry = (((mobile.spec as Record<string, unknown>).entry as Record<string, unknown>))
  assert.equal(desktopEntry.type, "client_module")
  assert.equal(mobileEntry.type, "schema_renderer")
  assert.equal(mobileEntry.schema_path, "modules/qq-personal-ui/mobile/schema.json")
})

test("mobile schema binds host command results without an extra data envelope", () => {
  const schema = JSON.parse(fs.readFileSync(path.join(root, "ui", "mobile", "schema.json"), "utf8"))
  const nodes = (schema.children ?? []) as Array<Record<string, unknown>>
  const page = nodes.find(node => node.id === "qq-personal-page") as Record<string, unknown>
  const collect = (node: Record<string, unknown>): Array<Record<string, unknown>> => [
    node,
    ...(((node.children ?? []) as Array<Record<string, unknown>>).flatMap(collect)),
  ]
  const children = collect(page)
  const bindings = children
    .flatMap(node => (node.bindings ?? []) as Array<Record<string, unknown>>)
    .map(binding => binding.sourcePath)
  assert.ok(bindings.includes("status.displayStatus.mobileDetail"))
  assert.ok(bindings.includes("status.mobileRuntimeSummary.running"))
  assert.ok(bindings.includes("status.displayStatus.installActionLabel"))
  assert.ok(Number(schema.lifecycle.refreshIntervalMs) >= 1000)
  assert.equal(schema.lifecycle.onRefresh[0].statePath, "status")
  assert.ok(bindings.includes("status.displayStatus.connectionLabel"))
  assert.ok(bindings.includes("status.displayStatus.mobileLabel"))
  assert.ok(bindings.includes("mobileConfig.display.reverseWebSocketUrl"))
  assert.equal(bindings.includes("status.data"), false)
  assert.equal(bindings.includes("status.mobileRuntime"), false)
})

test("desktop and mobile workspace tabs share the same provider package", () => {
  const workspaceTabs = contributions.filter(contribution => {
    const spec = contribution.spec as Record<string, unknown>
    return spec.capability === "workspace.tab"
  })
  assert.equal(workspaceTabs.length, 4)
  const tabIds = new Set<string>()
  for (const tab of workspaceTabs) {
    const spec = tab.spec as Record<string, unknown>
    const entries = spec.entries as Record<string, Record<string, unknown>>
    const metadata = spec.metadata as Record<string, unknown>
    tabIds.add(String(metadata.tabId))
    const desktop = entries.electron_windows
    const mobile = entries.mobile
    assert.ok(desktop || mobile)
    assert.equal((desktop || mobile).type, "external_web_ui")
  }
  assert.deepEqual([...tabIds].sort(), ["novnc-desktop", "snowluma-webui"])
  for (const tabId of tabIds) {
    const matches = workspaceTabs.filter(tab => {
      const spec = tab.spec as Record<string, unknown>
      const metadata = spec.metadata as Record<string, unknown>
      return metadata.tabId === tabId
    })
    const platforms = matches.flatMap(tab => {
      const spec = tab.spec as Record<string, unknown>
      return Object.keys(spec.entries as Record<string, unknown>)
    })
    assert.ok(platforms.includes("electron_windows"))
    assert.ok(platforms.includes("mobile"))
  }
  for (const tab of workspaceTabs) {
    const spec = tab.spec as Record<string, unknown>
    if ((spec.metadata as Record<string, unknown>).tabId !== "novnc-desktop") continue
    for (const entry of Object.values(spec.entries as Record<string, Record<string, unknown>>)) {
      assert.match(String(entry.url), /\/vnc\.html$/)
    }
  }
})
