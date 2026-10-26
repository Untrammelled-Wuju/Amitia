import assert from "node:assert/strict"
import { createHash } from "node:crypto"
import fs from "node:fs"
import os from "node:os"
import path from "node:path"
import test from "node:test"
import { strToU8, zipSync } from "fflate"
import * as tar from "tar"
import {
  isSupportedProjectNode,
  readInstallPointer,
  selectReleaseAsset,
  SnowLumaInstaller,
} from "./snowluma-installer.js"

test("selects only official Lite assets for supported native targets", () => {
  const assets = [
    { name: "SnowLuma-v2.2.0-win-x64.zip", size: 1, digest: `sha256:${"1".repeat(64)}`, browser_download_url: "https://example.invalid/full.zip" },
    { name: "SnowLuma-v2.2.0-win-x64-lite.zip", size: 2, digest: `sha256:${"2".repeat(64)}`, browser_download_url: "https://example.invalid/lite.zip" },
    { name: "SnowLuma-v2.2.0-linux-arm64-lite.tar.gz", size: 3, digest: `sha256:${"3".repeat(64)}`, browser_download_url: "https://example.invalid/lite.tar.gz" },
  ]
  assert.equal(selectReleaseAsset({ assets }, "win32", "x64").name, "SnowLuma-v2.2.0-win-x64-lite.zip")
  assert.equal(selectReleaseAsset({ assets }, "linux", "arm64").name, "SnowLuma-v2.2.0-linux-arm64-lite.tar.gz")
  assert.throws(() => selectReleaseAsset({ assets }, "darwin", "arm64"), /不支持当前平台/)
  assert.throws(() => selectReleaseAsset({ assets }, "linux", "x64"), /缺少 linux-x64 Lite/)
})

test("enforces the SnowLuma project Node contract", () => {
  assert.equal(isSupportedProjectNode("24.19.0"), false)
  assert.equal(isSupportedProjectNode("24.22.1"), true)
  assert.equal(isSupportedProjectNode("25.9.0"), false)
  assert.equal(isSupportedProjectNode("26.12.1"), true)
})

test("installs a verified SnowLuma package with shared writable data", async () => {
  const work = fs.mkdtempSync(path.join(os.tmpdir(), "snowluma-installer-"))
  const packageRoot = path.join(work, "package", "SnowLuma-v2.2.0-linux-x64")
  fs.mkdirSync(path.join(packageRoot, "config"), { recursive: true })
  fs.mkdirSync(path.join(packageRoot, "data", "10001"), { recursive: true })
  fs.writeFileSync(path.join(packageRoot, "index.mjs"), "console.log('snowluma')\n")
  fs.writeFileSync(path.join(packageRoot, "config", "onebot.json"), "{\"networks\":{}}\n")
  fs.writeFileSync(path.join(packageRoot, "data", "10001", "messages.db"), "fixture\n")
  const archive = path.join(work, "SnowLuma-v2.2.0-linux-x64-lite.tar.gz")
  await tar.c({ gzip: true, file: archive, cwd: path.join(work, "package") }, ["SnowLuma-v2.2.0-linux-x64"])
  const bytes = fs.readFileSync(archive)
  const digest = createHash("sha256").update(bytes).digest("hex")
  const release = {
    tag_name: "v2.2.0",
    assets: [
      {
        name: path.basename(archive),
        size: bytes.length,
        digest: `sha256:${digest}`,
        browser_download_url: "https://example.invalid/snowluma.tar.gz",
      },
    ],
  }
  const fetchImpl = async (input: string | URL | Request): Promise<Response> => {
    const url = String(input)
    if (url.includes("/releases/tags/")) return new Response(JSON.stringify(release), { status: 200 })
    return new Response(bytes, { status: 200, headers: { "content-type": "application/gzip" } })
  }
  const allowedRoot = path.join(work, "plugin-data")
  const installer = new SnowLumaInstaller(allowedRoot)
  const status = await installer.install("", {
    version: "2.2.0",
    platform: "linux",
    arch: "x64",
    nodeVersion: "26.12.1",
    fetchImpl: fetchImpl as typeof fetch,
  })
  assert.equal(status.phase, "completed")
  assert.equal(status.percent, 100)
  const pointer = readInstallPointer(allowedRoot)
  assert(pointer)
  assert.equal(pointer.version, "2.2.0")
  assert.equal(fs.existsSync(path.join(pointer.installDir, "index.mjs")), true)
  assert.equal(fs.lstatSync(path.join(pointer.installDir, "config")).isSymbolicLink(), true)
  assert.equal(fs.readFileSync(path.join(allowedRoot, "snowluma", "shared", "config", "onebot.json"), "utf8"), "{\"networks\":{}}\n")
  assert.equal(fs.readFileSync(path.join(allowedRoot, "snowluma", "shared", "data", "10001", "messages.db"), "utf8"), "fixture\n")
})

test("installs a verified Windows ZIP package", async () => {
  const work = fs.mkdtempSync(path.join(os.tmpdir(), "snowluma-installer-win-"))
  const archive = path.join(work, "SnowLuma-v2.2.0-win-x64-lite.zip")
  const prefix = "SnowLuma-v2.2.0-win-x64/"
  fs.writeFileSync(archive, zipSync({
    [`${prefix}index.mjs`]: strToU8("console.log('snowluma')\n"),
    [`${prefix}config/onebot.json`]: strToU8("{\"networks\":{}}\n"),
    [`${prefix}data/`]: new Uint8Array(),
  }))
  const bytes = fs.readFileSync(archive)
  const digest = createHash("sha256").update(bytes).digest("hex")
  const release = {
    tag_name: "v2.2.0",
    assets: [{
      name: path.basename(archive),
      size: bytes.length,
      digest: `sha256:${digest}`,
      browser_download_url: "https://example.invalid/snowluma.zip",
    }],
  }
  const fetchImpl = async (input: string | URL | Request): Promise<Response> => {
    const url = String(input)
    if (url.includes("/releases/tags/")) return new Response(JSON.stringify(release), { status: 200 })
    return new Response(bytes, { status: 200, headers: { "content-type": "application/zip" } })
  }
  const allowedRoot = path.join(work, "plugin-data")
  const installer = new SnowLumaInstaller(allowedRoot)
  const status = await installer.install("", {
    version: "2.2.0",
    platform: "win32",
    arch: "x64",
    nodeVersion: "26.12.1",
    fetchImpl: fetchImpl as typeof fetch,
  })
  assert.equal(status.phase, "completed")
  const pointer = readInstallPointer(allowedRoot)
  assert(pointer)
  assert.equal(fs.existsSync(path.join(pointer.installDir, "index.mjs")), true)
  assert.equal(fs.readFileSync(path.join(allowedRoot, "snowluma", "shared", "config", "onebot.json"), "utf8"), "{\"networks\":{}}\n")
})
