import assert from "node:assert/strict"
import fs from "node:fs"
import os from "node:os"
import path from "node:path"
import test from "node:test"
import { CredentialStore } from "./credential-store.js"

function tempDir(): string {
  return fs.mkdtempSync(path.join(os.tmpdir(), "qq-personal-cred-"))
}

function withKeyFile(prefix: string): string {
  const dir = tempDir()
  return path.join(dir, `${prefix}-credential`)
}

test("round trips a password and keeps it off disk in plain text", () => {
  const dir = tempDir()
  const store = new CredentialStore(dir)
  store.write("Passw0rd!demo")
  assert.equal(store.read(), "Passw0rd!demo")
  const raw = fs.readFileSync(path.join(dir, "webui-credential.json"), "utf8")
  assert.equal(raw.includes("Passw0rd!demo"), false)
  const mode = fs.statSync(path.join(dir, "webui-credential.json")).mode & 0o777
  assert.equal(mode, 0o600)
  const keyMode = fs.statSync(path.join(dir, "webui-credential.key")).mode & 0o777
  assert.equal(keyMode, 0o600)
})

test("reuses the local key so a new store instance can decrypt", () => {
  const dir = tempDir()
  new CredentialStore(dir).write("SecondPass!1")
  assert.equal(new CredentialStore(dir).read(), "SecondPass!1")
})

test("returns empty when the ciphertext is tampered with", () => {
  const dir = tempDir()
  const store = new CredentialStore(dir)
  store.write("TamperMe!1")
  const file = path.join(dir, "webui-credential.json")
  const record = JSON.parse(fs.readFileSync(file, "utf8")) as { data: string }
  record.data = Buffer.from("tampered").toString("base64")
  fs.writeFileSync(file, JSON.stringify(record))
  assert.equal(new CredentialStore(dir).read(), "")
})

test("clears the stored password", () => {
  const dir = tempDir()
  const store = new CredentialStore(dir)
  store.write("ClearMe!12")
  store.clear()
  assert.equal(store.read(), "")
  assert.equal(fs.existsSync(path.join(dir, "webui-credential.json")), false)
})

test("ignores an empty password", () => {
  const dir = tempDir()
  const store = new CredentialStore(dir)
  store.write("")
  assert.equal(store.read(), "")
})

test("derives the key from the host secret lease when provided", () => {
  const dir = tempDir()
  const previous = process.env.AMITIA_SECRET_LEASE
  process.env.AMITIA_SECRET_LEASE = "lease-from-host"
  try {
    new CredentialStore(dir).write("LeaseBound!1")
    assert.equal(new CredentialStore(dir).read(), "LeaseBound!1")
    assert.equal(fs.existsSync(path.join(dir, "webui-credential.key")), false)
  } finally {
    if (previous === undefined) delete process.env.AMITIA_SECRET_LEASE
    else process.env.AMITIA_SECRET_LEASE = previous
  }
})

test("withKeyFile helper keeps workspaces isolated", () => {
  const left = withKeyFile("left")
  const right = withKeyFile("right")
  new CredentialStore(left).write("LeftPass!12")
  assert.equal(new CredentialStore(right).read(), "")
})
