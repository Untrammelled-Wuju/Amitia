import assert from "node:assert/strict"
import test from "node:test"
import { isRuntimeModeSupported, normalizeRuntimeModeForPlatform } from "./config.js"

test("only allows Docker on macOS", () => {
  assert.equal(isRuntimeModeSupported("docker", "darwin"), true)
  assert.equal(isRuntimeModeSupported("native", "darwin"), false)
  assert.equal(normalizeRuntimeModeForPlatform("native", "native", "darwin"), "docker")
  assert.equal(normalizeRuntimeModeForPlatform("external", "native", "darwin"), "docker")
})

test("allows Docker and native mode on Windows and Linux", () => {
  for (const platform of ["win32", "linux"] as const) {
    assert.equal(isRuntimeModeSupported("docker", platform), true)
    assert.equal(isRuntimeModeSupported("native", platform), true)
    assert.equal(isRuntimeModeSupported("external", platform), false)
    assert.equal(normalizeRuntimeModeForPlatform("native", "docker", platform), "native")
  }
})
