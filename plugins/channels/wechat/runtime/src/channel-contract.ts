import type { FastifyInstance } from "fastify";
import type { OpenClawWechatManager } from "./openclaw-wechat.js";

interface WechatContractOptions {
    manager: OpenClawWechatManager;
    config: {
        port: number;
    };
}

function backgroundScan(manager: OpenClawWechatManager): void {
    manager.waitForScan(120000).then((result) => {
        if (result.connected) {
            manager.startPolling().catch((err) => console.error("[Sidecar] Auto-polling start failed:", err));
        }
    }).catch((err) => console.error("[Sidecar] Background waitForScan failed:", err));
}

function connectInput(value: unknown): Record<string, unknown> {
    if (!value || typeof value !== "object") return {};
    const record = value as Record<string, unknown>;
    if (record.config && typeof record.config === "object") {
        return record.config as Record<string, unknown>;
    }
    return record;
}

function providerConfig(manager: OpenClawWechatManager, port: number) {
    const state = manager.getState();
    return {
        mode: "openclaw",
        sidecarPort: port,
        success: true,
        data: {
            mode: "openclaw",
            sidecarPort: port,
            status: state.status,
            accountId: state.accountId,
            baseUrl: state.baseUrl,
        },
    };
}

async function startLoginChannel(manager: OpenClawWechatManager, input: Record<string, unknown>) {
    if (input.force === true) {
        await manager.resetLogin();
    }
    const result = await manager.startLogin({ force: input.force === true });
    backgroundScan(manager);
    return {
        success: true,
        message: "QR code generated",
        data: {
            qrCodeUrl: result.qrCodeUrl,
            qrImageUrl: result.qrImageUrl,
            sessionKey: result.sessionKey,
            status: "qr_ready",
        },
    };
}

async function connectChannel(manager: OpenClawWechatManager, input: Record<string, unknown>) {
    if (input.reconnect === true) {
        await manager.stopPolling();
        await manager.startPolling();
        return { success: true, data: manager.getState() };
    }
    const state = manager.getState();
    if (input.force !== true && state.status === "connected") {
        return { success: true, data: { status: "connected", accountId: state.accountId } };
    }
    return startLoginChannel(manager, input);
}

function errorMessage(err: unknown): string {
    return err instanceof Error ? err.message : String(err || "服务器错误");
}

export function registerWechatContractRoutes(app: FastifyInstance, options: WechatContractOptions): void {
    const { manager, config } = options;

    app.get("/api/config", async (_req, reply) => {
        return reply.send(providerConfig(manager, config.port));
    });

    app.post("/api/connect", async (req, reply) => {
        try {
            return reply.send(await connectChannel(manager, connectInput(req.body)));
        } catch (err) {
            return reply.status(500).send({ success: false, message: errorMessage(err) });
        }
    });

    app.post("/api/disconnect", async (_req, reply) => {
        try {
            await manager.resetLogin();
            return reply.send({ success: true, data: manager.getState() });
        } catch (err) {
            return reply.status(500).send({ success: false, message: errorMessage(err) });
        }
    });

    app.post("/api/reconnect", async (_req, reply) => {
        try {
            await manager.stopPolling();
            await manager.startPolling();
            return reply.send({ success: true, data: manager.getState() });
        } catch (err) {
            return reply.status(500).send({ success: false, message: errorMessage(err) });
        }
    });

    app.post("/api/action", async (req, reply) => {
        const body = (req.body || {}) as {
            action?: string;
            input?: Record<string, unknown>;
        };
        const action = String(body.action || "").trim();
        const input = body.input && typeof body.input === "object" ? body.input : {};
        try {
            if (action === "status") {
                return reply.send({ success: true, data: manager.getState() });
            }
            if (action === "config") {
                return reply.send(providerConfig(manager, config.port));
            }
            if (action === "connect" || action === "login.start") {
                return reply.send(await connectChannel(manager, input));
            }
            if (action === "login.rescan") {
                return reply.send(await startLoginChannel(manager, { ...input, force: true }));
            }
            if (action === "reconnect" || action === "login.reconnect") {
                await manager.stopPolling();
                await manager.startPolling();
                return reply.send({ success: true, data: manager.getState() });
            }
            if (action === "disconnect") {
                await manager.resetLogin();
                return reply.send({ success: true, data: manager.getState() });
            }
            return reply.status(400).send({
                success: false,
                message: `Unsupported action: ${action || "(empty)"}`,
            });
        } catch (err) {
            return reply.status(500).send({ success: false, message: errorMessage(err) });
        }
    });
}
