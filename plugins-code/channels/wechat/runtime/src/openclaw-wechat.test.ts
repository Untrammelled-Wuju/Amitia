import { test } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { OpenClawWechatManager } from "./openclaw-wechat.js";

test("keeps outbound context and delivery idempotency when rebuilding the service", async () => {
    const directory = fs.mkdtempSync(path.join(os.tmpdir(), "amitia-wechat-test-"));
    const previous = process.env.OPENCLAW_STATE_DIR;
    process.env.OPENCLAW_STATE_DIR = directory;
    try {
        const manager = new OpenClawWechatManager() as any;
        manager.token = "test-token";
        manager.state.accountId = "test-account";
        const messages: any[] = [];
        manager.sendMessageChecked = async (body: any) => { messages.push(body); };
        await manager.sendTextMessage("test-peer", "测试回复", "test-context", "delivery-test");
        const duplicate = await manager.sendTextMessage("test-peer", "测试回复", "test-context", "delivery-test");
        assert.equal(messages.length, 1);
        assert.equal(messages[0].msg.context_token, "test-context");
        assert.equal(messages[0].msg.to_user_id, "test-peer");
        assert.equal(messages[0].msg.item_list[0].text_item.text, "测试回复");
        assert.equal(duplicate.duplicate, true);
        assert.equal(manager.getState().replyCount, 1);
        await assert.rejects(manager.sendTextMessage("unknown-peer", "无上下文"), /context_token missing/);
    } finally {
        if (previous === undefined) delete process.env.OPENCLAW_STATE_DIR;
        else process.env.OPENCLAW_STATE_DIR = previous;
        fs.rmSync(directory, { recursive: true, force: true });
    }
});

test("does not treat a WeChat business error as a successful send", async () => {
    const manager = new OpenClawWechatManager() as any;
    manager.token = "test-token";
    const previous = globalThis.fetch;
    globalThis.fetch = async () => new Response(JSON.stringify({ ret: 0, errcode: 10013, errmsg: "invalid context" }));
    try {
        await assert.rejects(manager.sendMessageChecked({ msg: { client_id: "test-client" } }), /10013/);
    } finally {
        globalThis.fetch = previous;
    }
});

test("reset login does not wait for an in-flight WeChat long poll", async () => {
    const manager = new OpenClawWechatManager() as any;
    const controller = new AbortController();
    manager.polling = true;
    manager.pollGeneration = 1;
    manager.pollAbort = controller;
    manager.pollPromise = new Promise(() => undefined);
    const startedAt = Date.now();
    await manager.resetLogin();
    assert.ok(Date.now() - startedAt < 1000);
    assert.equal(controller.signal.aborted, true);
    assert.equal(manager.getState().status, "idle");
    assert.equal(manager.pollPromise, null);
});
