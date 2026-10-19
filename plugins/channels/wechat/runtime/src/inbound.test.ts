import { test } from "node:test";
import assert from "node:assert/strict";
import { buildInboundRequest, createInboundClient } from "./inbound.js";

const config = { coreUrl: "http://127.0.0.1:18899", extensionId: "com.amitia/channel-wechat", moduleId: "wechat-channel-service", serviceAuthToken: "test-service-credential" };
const message = { fromUserId: "test-peer", messageId: "message-1", text: "你好" };

test("forwards the declared provider identity and current inbound schema", async () => {
    let actualUrl = "";
    let actual: RequestInit | undefined;
    const client = createInboundClient(config, (async (url, request) => {
        actualUrl = String(url);
        actual = request;
        return new Response(JSON.stringify({ code: 200, data: { requestId: "request-1", outcome: "completed" } }));
    }) as typeof fetch);
    assert.equal((await client([message]))?.requestId, "request-1");
    assert.equal(actualUrl, config.coreUrl + "/api/channels/inbound");
    const headers = new Headers(actual?.headers);
    assert.equal(headers.get("X-Amitia-Extension-ID"), config.extensionId);
    assert.equal(headers.get("X-Amitia-Module-ID"), config.moduleId);
    assert.equal(headers.get("Authorization"), "Bearer " + config.serviceAuthToken);
    assert.deepEqual(JSON.parse(String(actual?.body)), { channelId: "wechat", accountId: "openclaw-wechat", conversationId: "conv-test-peer", peerId: "test-peer", messageId: "message-1", contentType: "text", text: "你好" });
});

test("keeps failures visible and rejects missing runtime credentials", async () => {
    assert.throws(() => createInboundClient({ ...config, serviceAuthToken: "" }), /身份或服务凭据/);
    const client = createInboundClient(config, (async () => new Response(JSON.stringify({ code: 401, msg: "invalid channel provider credential" }), { status: 401 })) as typeof fetch);
    await assert.rejects(client([message]), /401.*invalid channel provider credential/);
    const businessFailure = createInboundClient(config, (async () => new Response(JSON.stringify({ code: 502, msg: "模型不可用" }))) as typeof fetch);
    await assert.rejects(businessFailure([message]), /模型不可用/);
});

test("preserves merged content and supplies media in the current schema", () => {
    assert.equal(buildInboundRequest([message, { ...message, messageId: "message-2", text: "在吗" }]).text, "你好\n在吗");
    const voice = buildInboundRequest([{ ...message, audioBase64: "YXVkaW8=" }]);
    assert.equal(voice.contentType, "audio");
    assert.equal(voice.audioUrl, "data:audio/wav;base64,YXVkaW8=");
    assert.equal(buildInboundRequest([message], "data:image/png;base64,aW1hZ2U=").contentType, "image");
});
