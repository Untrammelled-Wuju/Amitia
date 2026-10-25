import { test } from "node:test";
import assert from "node:assert/strict";
import { createChannelClient, createHostBridge, loginCode, statusLabel, unwrapResult } from "../src/session.mjs";

test("unwraps channel responses and preserves service failures", () => {
  assert.deepEqual(unwrapResult({ success: true, data: { data: { status: "connected" } } }), { status: "connected" });
  assert.throws(() => unwrapResult({ success: false, message: "微信连接超时" }), /微信连接超时/);
  assert.throws(() => unwrapResult({ code: 500, msg: "服务未启动" }), /服务未启动/);
});

test("treats the actual WeChat login page as QR content instead of an image", () => {
  const url = "https://liteapp.weixin.qq.com/q/example?qrcode=test&bot_type=3";
  assert.deepEqual(loginCode({ data: { qrCodeUrl: url } }), { text: url, image: "" });
  const image = "data:image/png;base64,test";
  assert.deepEqual(loginCode({ qrImageUrl: image }), { text: "", image });
  assert.throws(() => loginCode({ message: "登录超时" }), /登录超时/);
});

test("uses the existing host action contract and passes channel ownership", async () => {
  const calls = [];
  const client = createChannelClient({ invokeAction: async (...args) => { calls.push(args); return { data: { status: "waiting_scan" } }; } });
  assert.equal((await client("connect", { config: { reconnect: true } })).status, "waiting_scan");
  assert.deepEqual(calls, [["connect", { channelId: "wechat", config: { reconnect: true } }]]);
  assert.throws(() => createChannelClient(null), /尚未就绪/);
  assert.equal(statusLabel("waiting_confirm"), "请在微信中确认");
  assert.equal(statusLabel("qr_ready"), "等待微信扫码");
});

test("maps the same-page host guard onto the channel action contract", async () => {
  const calls = [];
  const bridge = createHostBridge({ status: async (input) => { calls.push(input); return { status: "connected" }; } }, null);
  assert.deepEqual(await bridge.invokeAction("status", { channelId: "wechat" }), { status: "connected" });
  assert.deepEqual(calls, [{ channelId: "wechat" }]);
  await assert.rejects(bridge.invokeAction("missing", {}), /宿主未提供操作/);
});

test("coalesces status refreshes and stays below the bridge action budget", async () => {
  let time = 0;
  let count = 0;
  const client = createChannelClient({ invokeAction: async () => { count++; return { status: "waiting_scan" }; } }, { now: () => time });
  await Promise.all(Array.from({ length: 30 }, () => client("status")));
  assert.equal(count, 1);
  for (time = 2000; time < 60000; time += 2000) await client("status");
  assert.ok(count <= 12);
  assert.equal(client.nextPollDelay(true), 5000);
  assert.equal(client.nextPollDelay(false), 15000);
});

test("backs off on host rate limits without issuing repeated requests", async () => {
  let time = 0;
  let count = 0;
  const client = createChannelClient({ invokeAction: async () => { count++; if (count === 1) throw new Error("host bridge rate limited"); return { status: "connected" }; } }, { now: () => time });
  await assert.rejects(client("status"), /稍后再试/);
  assert.equal(client.nextPollDelay(true), 60000);
  time = 5000;
  await assert.rejects(client("status"), /稍后再试/);
  assert.equal(count, 1);
  time = 60001;
  assert.equal((await client("status")).status, "connected");
});
