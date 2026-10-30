import assert from "node:assert/strict"
import fs from "node:fs/promises"
import os from "node:os"
import path from "node:path"
import test from "node:test"
import { MOBILE_QQ_VERSION } from "./mobile-qq-package.js"
import {
  mobileAptOptions,
  mobileEnvironmentReady,
  mobileRuntimePackages,
  prepareMobileEnvironment,
  runWithReceipt,
  type ExtractDebPackage,
  type RunCommand,
} from "./mobile-environment.js"

test("mobile runtime package list includes the Node ARM64 dependency closure", () => {
  assert.ok(mobileRuntimePackages.includes("libatomic1"))
  assert.ok(mobileRuntimePackages.includes("xvfb"))
  assert.ok(mobileRuntimePackages.includes("xkb-data"))
  assert.ok(mobileRuntimePackages.includes("x11-xkb-utils"))
  assert.ok(mobileRuntimePackages.includes("novnc"))
})

test("mobile process receipts execute the command instead of treating the script as a file", async t => {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), "qq-personal-receipt-"))
  t.after(() => fs.rm(directory, { recursive: true, force: true }))
  const receipt = path.join(directory, "status")
  await runWithReceipt("/usr/bin/true", [], receipt)
  assert.equal(await fs.readFile(receipt, "utf8"), "0")
})

test("mobile preparation extracts dependencies and QQ without dpkg-deb extract", async t => {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), "qq-personal-mobile-"))
  t.after(() => fs.rm(directory, { recursive: true, force: true }))
  const prefix = path.join(directory, "root")
  await fs.mkdir(path.join(directory, "apt", "archives"), { recursive: true })
  await fs.mkdir(path.join(prefix, "opt", "QQ"), { recursive: true })
  await fs.mkdir(path.join(prefix, "usr", "bin"), { recursive: true })
  await fs.mkdir(path.join(prefix, "usr", "lib", "aarch64-linux-gnu"), { recursive: true })
  await fs.writeFile(path.join(directory, "apt", "archives", "runtime.deb"), "runtime")
  await fs.writeFile(path.join(prefix, "opt", "QQ", "qq"), "")
  await fs.writeFile(path.join(prefix, "usr", "bin", "Xvfb"), "")
  await fs.writeFile(path.join(prefix, "usr", "bin", "x11vnc"), "")
  await fs.writeFile(path.join(prefix, "usr", "lib", "aarch64-linux-gnu", "libatomic.so.1"), "")
  await Promise.all([
    fs.chmod(path.join(prefix, "opt", "QQ", "qq"), 0o755),
    fs.chmod(path.join(prefix, "usr", "bin", "Xvfb"), 0o755),
    fs.chmod(path.join(prefix, "usr", "bin", "x11vnc"), 0o755),
  ])
  const commands: string[][] = []
  const run: RunCommand = async (command, args) => {
    commands.push([command, ...args])
  }
  const extract: ExtractDebPackage = async () => {}
  const qqArchive = path.join(directory, "qq.deb")
  const progress: string[] = []
  const output = await prepareMobileEnvironment(
    directory,
    (_phase, detail) => { progress.push(detail) },
    run,
    async (_directory, _signal, onProgress) => {
      onProgress?.(1048576, 2097152)
      return qqArchive
    },
    extract,
    "linux",
    "arm64",
  )
  assert.equal(output, prefix)
  assert.ok(progress.some(detail => detail.includes("1.0 MB / 2.0 MB") && detail.includes("50%")))
  assert.equal(commands.filter(command => command[0] === "/usr/bin/apt-get").length, 2)
  const options = mobileAptOptions(directory)
  assert.ok(options.includes(path.join(directory, "apt", "config")))
})

test("prepared runtime is reusable only with its required keyboard files", async t => {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), "qq-prepared-"))
  t.after(() => fs.rm(directory, { recursive: true, force: true }))
  await fs.writeFile(path.join(directory, "prepared.json"), JSON.stringify({ schemaVersion: 1 }))
  assert.equal(await mobileEnvironmentReady(directory), false)
  for (const relative of ["root/opt/QQ/qq", "root/usr/bin/Xvfb", "root/usr/bin/x11vnc", "root/usr/bin/xkbcomp", "root/usr/share/X11/xkb/rules/evdev", "root/usr/share/X11/xkb/symbols/us"]) {
    const file = path.join(directory, relative)
    await fs.mkdir(path.dirname(file), { recursive: true })
    await fs.writeFile(file, "test")
  }
  assert.equal(await mobileEnvironmentReady(directory), false)
  await fs.writeFile(path.join(directory, "prepared.json"), JSON.stringify({ schemaVersion: 1, qqVersion: MOBILE_QQ_VERSION }))
  assert.equal(await mobileEnvironmentReady(directory), true)
  await fs.rm(path.join(directory, "root/usr/bin/xkbcomp"))
  assert.equal(await mobileEnvironmentReady(directory), false)
})
