import { describe, it } from "node:test"
import assert from "node:assert"
import fs from "node:fs"
import path from "node:path"
import os from "node:os"
import { DeliveryIdempotencyStore } from "./delivery-idempotency.js"

describe("DeliveryIdempotencyStore", () => {
  function tmpDir(): string {
    return fs.mkdtempSync(path.join(os.tmpdir(), "delivery-idempotency-test-"))
  }

  it("same key sequential twice, sender called once", async () => {
    const dir = tmpDir()
    const filePath = path.join(dir, "ledger.json")
    const store = new DeliveryIdempotencyStore(filePath)
    let calls = 0
    const sender = async (_clientId: string) => { calls++ }
    const r1 = await store.execute("key-1", sender)
    assert.strictEqual(r1.duplicate, false)
    assert.strictEqual(calls, 1)
    const r2 = await store.execute("key-1", sender)
    assert.strictEqual(r2.duplicate, true)
    assert.strictEqual(calls, 1)
  })

  it("same key concurrent twice, sender called once", async () => {
    const dir = tmpDir()
    const filePath = path.join(dir, "ledger.json")
    const store = new DeliveryIdempotencyStore(filePath)
    let calls = 0
    const sender = async (_clientId: string) => {
      calls++
      await new Promise(r => setTimeout(r, 50))
    }
    const [r1, r2] = await Promise.all([
      store.execute("key-conc", sender),
      store.execute("key-conc", sender),
    ])
    assert.strictEqual(calls, 1)
    assert.ok(r1.duplicate || r2.duplicate)
    assert.ok(!r1.duplicate || !r2.duplicate)
  })

  it("persistence across instances", async () => {
    const dir = tmpDir()
    const filePath = path.join(dir, "ledger.json")
    let calls = 0
    const sender = async (_clientId: string) => { calls++ }
    const s1 = new DeliveryIdempotencyStore(filePath)
    await s1.execute("key-persist", sender)
    assert.strictEqual(calls, 1)
    const s2 = new DeliveryIdempotencyStore(filePath)
    await s2.execute("key-persist", sender)
    assert.strictEqual(calls, 1)
  })

  it("sender throws, retry uses same clientId", async () => {
    const dir = tmpDir()
    const filePath = path.join(dir, "ledger.json")
    const store = new DeliveryIdempotencyStore(filePath)
    let firstClientId = ""
    const sender1 = async (clientId: string) => {
      firstClientId = clientId
      throw new Error("fail")
    }
    await store.execute("key-retry", sender1).catch(() => {})
    let secondClientId = ""
    const sender2 = async (clientId: string) => {
      secondClientId = clientId
    }
    await store.execute("key-retry", sender2)
    assert.strictEqual(firstClientId, secondClientId)
  })

  it("different keys generate different clientIds", async () => {
    const store = new DeliveryIdempotencyStore(path.join(tmpDir(), "ledger.json"))
    const c1 = (await store.execute("k1", async () => {})).clientId
    const c2 = (await store.execute("k2", async () => {})).clientId
    assert.notStrictEqual(c1, c2)
  })

  it("corrupt JSON handled without crash", async () => {
    const dir = tmpDir()
    const filePath = path.join(dir, "ledger.json")
    fs.mkdirSync(path.dirname(filePath), { recursive: true })
    fs.writeFileSync(filePath, "not valid json {{{", "utf-8")
    const store = new DeliveryIdempotencyStore(filePath)
    let called = false
    await store.execute("after-corrupt", async () => { called = true })
    assert.strictEqual(called, true)
    const files = fs.readdirSync(path.dirname(filePath))
    const corrupt = files.find(f => f.includes(".corrupt"))
    assert.ok(corrupt, "should have created .corrupt backup")
  })
})
