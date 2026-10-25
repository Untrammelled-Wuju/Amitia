import assert from "node:assert/strict"
import test from "node:test"
import { IdempotencyStore } from "./idempotency.js"

test("相同幂等键和内容只执行一次", async () => {
  const store = new IdempotencyStore(1000)
  let executions = 0
  const run = () => store.execute("same", store.fingerprint({ text: "hello" }), async () => {
    executions++
    return { messageId: executions }
  })
  assert.deepEqual(await run(), { messageId: 1 })
  assert.deepEqual(await run(), { messageId: 1 })
  assert.equal(executions, 1)
})

test("相同幂等键对应不同内容时拒绝", async () => {
  const store = new IdempotencyStore(1000)
  await store.execute("same", store.fingerprint({ text: "one" }), async () => "ok")
  await assert.rejects(
    store.execute("same", store.fingerprint({ text: "two" }), async () => "wrong"),
    /不同的发送内容/,
  )
})

test("失败请求允许重试", async () => {
  const store = new IdempotencyStore(1000)
  let attempts = 0
  const fingerprint = store.fingerprint({ text: "retry" })
  await assert.rejects(store.execute("retry", fingerprint, async () => {
    attempts++
    throw new Error("temporary")
  }))
  assert.equal(await store.execute("retry", fingerprint, async () => {
    attempts++
    return "ok"
  }), "ok")
  assert.equal(attempts, 2)
})
