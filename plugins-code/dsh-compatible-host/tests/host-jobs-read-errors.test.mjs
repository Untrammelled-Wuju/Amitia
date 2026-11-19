import assert from "node:assert/strict";
import test from "node:test";
import { createAmitiaDshHostServices } from "../src/runtime/host-services.mjs";

function createHost() {
  return {
    async call() {
      return {};
    },
  };
}

test("jobs.read 把 source 读取失败记入 readErrors 而不是静默跳过", async () => {
  const services = createAmitiaDshHostServices(createHost());
  const original = new Error("source read failed");
  services.jobs.start({
    id: "job-broken",
    output: [{ channel: "stdout", read: () => { throw original; } }],
    run: async () => "done",
  });
  await new Promise((resolve) => setTimeout(resolve, 10));
  const result = services.jobs.read("job-broken");
  assert.equal(result.job.readErrors.length, 1);
  assert.equal(result.job.readErrors[0].channel, "stdout");
  assert.equal(result.job.readErrors[0].cursor, 0);
  assert.equal(result.job.readErrors[0].message, original.message);
  assert.ok(Array.isArray(result.chunks));
});

test("jobs.read 正常读取时不输出 readErrors 字段", async () => {
  const services = createAmitiaDshHostServices(createHost());
  let offset = 0;
  services.jobs.start({
    id: "job-ok",
    output: [{ channel: "stdout", read: (cursor) => {
      if (cursor > 0) return null;
      return { text: "hello\n", nextOffset: 6 };
    } }],
    run: async () => "done",
  });
  await new Promise((resolve) => setTimeout(resolve, 10));
  const result = services.jobs.read("job-ok");
  assert.equal(Object.hasOwn(result.job, "readErrors"), false);
  assert.ok(result.chunks.some((chunk) => chunk.text === "hello"));
});
