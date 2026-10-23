import { test } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import Fastify from "fastify";
import { registerWechatContractRoutes } from "./channel-contract.js";

function createManager(initial: Record<string, unknown> = {}) {
    const calls: Array<[string, unknown?]> = [];
    const state = {
        status: "idle",
        accountId: null,
        qrCodeUrl: "",
        sessionKey: "",
        message: "",
        messageCount: 0,
        replyCount: 0,
        baseUrl: "https://ilinkai.weixin.qq.com",
        startedAt: null,
        lastError: null,
        ...initial,
    };
    return {
        calls,
        getState: () => ({ ...state }),
        startLogin: async (options?: { force?: boolean }) => {
            calls.push(["startLogin", options]);
            state.status = "qr_ready";
            state.qrCodeUrl = "https://example.test/qr";
            state.sessionKey = "session-1";
            return {
                qrCodeUrl: state.qrCodeUrl,
                qrImageUrl: "data:image/png;base64,AA==",
                sessionKey: state.sessionKey,
            };
        },
        waitForScan: async () => {
            calls.push(["waitForScan"]);
            return { connected: false, message: "pending" };
        },
        startPolling: async () => {
            calls.push(["startPolling"]);
        },
        stopPolling: async () => {
            calls.push(["stopPolling"]);
        },
        resetLogin: async () => {
            calls.push(["resetLogin"]);
            state.status = "idle";
            state.accountId = null;
            state.qrCodeUrl = "";
            state.sessionKey = "";
        },
    } as any;
}

async function createApp(manager: any) {
    const app = Fastify();
    registerWechatContractRoutes(app, { manager, config: { port: 19876 } });
    await app.ready();
    return app;
}

test("channel contract routes are registered", async () => {
    const app = await createApp(createManager());
    const checks: Array<[string, string]> = [
        ["GET", "/api/config"],
        ["POST", "/api/connect"],
        ["POST", "/api/disconnect"],
        ["POST", "/api/reconnect"],
        ["POST", "/api/action"],
    ];
    for (const [method, url] of checks) {
        const response = await app.inject({
            method: method as "GET" | "POST",
            url,
            payload: method === "POST" ? {} : undefined,
        });
        assert.notEqual(response.statusCode, 404, `${method} ${url}`);
    }
    await app.close();
});

test("manifest declares the channel contract paths used by the host", () => {
    const manifest = JSON.parse(fs.readFileSync(new URL("../../amitia-extension.json", import.meta.url), "utf8"));
    const service = manifest.modules.find((item: any) => item.type === "service");
    const sidecar = service.provider.metadata.sidecar;
    assert.equal(sidecar.connectPath, "/api/connect");
    assert.equal(sidecar.disconnectPath, "/api/disconnect");
    assert.equal(sidecar.configPath, "/api/config");
    assert.equal(sidecar.actionPath, "/api/action");
    assert.equal(sidecar.statusPath, "/api/status");
    assert.equal(sidecar.sendPath, "/api/send");
    assert.equal(sidecar.imagePath, "/api/send-image");
    assert.equal(sidecar.voicePath, "/api/send-voice");
});

test("connect starts login and returns QR data when disconnected", async () => {
    const manager = createManager();
    const app = await createApp(manager);
    const response = await app.inject({ method: "POST", url: "/api/connect", payload: {} });
    assert.equal(response.statusCode, 200);
    const body = response.json();
    assert.equal(body.success, true);
    assert.equal(body.data.status, "qr_ready");
    assert.equal(body.data.qrCodeUrl, "https://example.test/qr");
    assert.deepEqual(manager.calls[0], ["startLogin", { force: false }]);
    await app.close();
});

test("connect returns the existing account when already connected", async () => {
    const manager = createManager({ status: "connected", accountId: "account-1" });
    const app = await createApp(manager);
    const response = await app.inject({ method: "POST", url: "/api/connect", payload: {} });
    assert.equal(response.statusCode, 200);
    assert.deepEqual(response.json().data, { status: "connected", accountId: "account-1" });
    assert.equal(manager.calls.some((call: any[]) => call[0] === "startLogin"), false);
    await app.close();
});

test("connect with force clears the old login before starting login", async () => {
    const manager = createManager({ status: "connected", accountId: "account-1" });
    const app = await createApp(manager);
    const response = await app.inject({ method: "POST", url: "/api/connect", payload: { force: true } });
    assert.equal(response.statusCode, 200);
    assert.deepEqual(manager.calls.slice(0, 2), [["resetLogin"], ["startLogin", { force: true }]]);
    await app.close();
});

test("connect accepts the nested config used by the contribution bridge", async () => {
    const manager = createManager({ status: "connected", accountId: "account-1" });
    const app = await createApp(manager);
    const response = await app.inject({ method: "POST", url: "/api/connect", payload: { config: { force: true } } });
    assert.equal(response.statusCode, 200);
    assert.deepEqual(manager.calls.slice(0, 2), [["resetLogin"], ["startLogin", { force: true }]]);
    await app.close();
});

test("connect with reconnect restarts polling without a new login", async () => {
    const manager = createManager({ status: "connected", accountId: "account-1" });
    const app = await createApp(manager);
    const response = await app.inject({ method: "POST", url: "/api/connect", payload: { reconnect: true } });
    assert.equal(response.statusCode, 200);
    assert.deepEqual(manager.calls, [["stopPolling"], ["startPolling"]]);
    await app.close();
});

test("disconnect clears the current login", async () => {
    const manager = createManager({ status: "connected", accountId: "account-1" });
    const app = await createApp(manager);
    const response = await app.inject({ method: "POST", url: "/api/disconnect", payload: {} });
    assert.equal(response.statusCode, 200);
    assert.deepEqual(manager.calls, [["resetLogin"]]);
    assert.equal(response.json().data.status, "idle");
    await app.close();
});

test("config exposes the provider contract information", async () => {
    const app = await createApp(createManager({ status: "connected", accountId: "account-1" }));
    const response = await app.inject({ method: "GET", url: "/api/config" });
    assert.equal(response.statusCode, 200);
    assert.equal(response.json().mode, "openclaw");
    assert.equal(response.json().sidecarPort, 19876);
    assert.equal(response.json().data.accountId, "account-1");
    await app.close();
});

test("action dispatches connect, disconnect and status", async () => {
    const manager = createManager();
    const app = await createApp(manager);
    const status = await app.inject({ method: "POST", url: "/api/action", payload: { action: "status" } });
    assert.equal(status.statusCode, 200);
    const connect = await app.inject({ method: "POST", url: "/api/action", payload: { action: "connect", input: {} } });
    assert.equal(connect.statusCode, 200);
    const disconnect = await app.inject({ method: "POST", url: "/api/action", payload: { action: "disconnect", input: {} } });
    assert.equal(disconnect.statusCode, 200);
    assert.equal(manager.calls.some((call: any[]) => call[0] === "startLogin"), true);
    assert.equal(manager.calls.some((call: any[]) => call[0] === "resetLogin"), true);
    await app.close();
});

test("unknown action returns a clear error instead of a missing route", async () => {
    const app = await createApp(createManager());
    const response = await app.inject({ method: "POST", url: "/api/action", payload: { action: "unknown" } });
    assert.equal(response.statusCode, 400);
    assert.match(response.json().message, /Unsupported action/);
    await app.close();
});
