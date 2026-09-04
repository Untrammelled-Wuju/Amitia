import { randomBytes } from "crypto";
import { RpcConnection, JsonRpcMessage } from "./rpc";
import { bootstrap, BootstrapSpec } from "./bootstrap";
import { HandlerRegistry, InvocationContext } from "./handler-registry";
import { getState, setState, shutdown, onShutdown } from "./shutdown";
import { LoadedExtension } from "./module-loader";

function log(message: string): void {
  process.stderr.write("[plugin-host] " + message + "\n");
}

interface InvocationState {
  canceled: boolean;
  cancelHandlers: Array<() => void>;
}

async function main(): Promise<void> {
  const rpc = new RpcConnection();
  const registry = new HandlerRegistry();
  const invocations = new Map<string, InvocationState>();
  let extension: LoadedExtension | null = null;

  const instanceId = randomBytes(8).toString("hex");
  const nonce = randomBytes(8).toString("hex");

  setState("starting");
  rpc.sendNotification("runtime.hello", { instanceId, nonce });

  rpc.on("runtime.initialize", async (message: JsonRpcMessage) => {
    try {
      const spec: BootstrapSpec = (message.params || {}) as BootstrapSpec;
      extension = await bootstrap(spec, rpc, registry);
      setState("ready");
      rpc.sendNotification("runtime.ready", { instanceId });
    } catch (e) {
      setState("failed");
      rpc.sendNotification("log.write", {
        level: "error",
        message: "initialize failed: " + (e as Error).message,
      });
      log("initialize failed: " + (e as Error).message);
      process.exit(1);
    }
  });

  rpc.on("runtime.invoke", async (message: JsonRpcMessage) => {
    if (message.id === undefined) {
      return;
    }
    const params = message.params || {};
    const contributionId = params.contributionId;
    const input = params.input;
    const invocationId = String(message.id);
    const state: InvocationState = { canceled: false, cancelHandlers: [] };
    invocations.set(invocationId, state);

    const handler = registry.get(contributionId);
    if (!handler) {
      invocations.delete(invocationId);
      rpc.sendError(message.id, -32601, "Tool not found: " + contributionId);
      return;
    }

    const invocation: InvocationContext = {
      invocationId,
      get isCanceled() {
        return state.canceled;
      },
      onCancel: (h: () => void) => {
        state.cancelHandlers.push(h);
      },
    };

    try {
      const result = await handler(input, invocation);
      rpc.sendResult(message.id, result);
    } catch (e) {
      rpc.sendError(message.id, -32603, (e as Error).message || "Internal error");
    } finally {
      invocations.delete(invocationId);
    }
  });

  rpc.on("runtime.cancel", (message: JsonRpcMessage) => {
    const params = message.params || {};
    const invocationId = String(params.invocationId);
    const state = invocations.get(invocationId);
    if (state) {
      state.canceled = true;
      for (const h of state.cancelHandlers) {
        try {
          h();
        } catch (e) {
        }
      }
    }
  });

  rpc.on("runtime.shutdown", () => {
    shutdown("runtime.shutdown");
  });

  onShutdown(async () => {
    if (extension && typeof extension.deactivate === "function") {
      try {
        await extension.deactivate();
      } catch (e) {
      }
    }
    rpc.close();
  });

  process.on("SIGTERM", () => {
    shutdown("SIGTERM");
  });
  process.on("SIGINT", () => {
    shutdown("SIGINT");
  });

  process.on("uncaughtException", (err: Error) => {
    setState("crashed");
    log("uncaughtException: " + (err.stack || err.message));
    rpc.sendNotification("log.write", {
      level: "error",
      message: "crashed: " + err.message,
    });
    process.exit(1);
  });

  process.on("unhandledRejection", (reason: any) => {
    setState("crashed");
    const msg = reason && reason.message ? reason.message : String(reason);
    log("unhandledRejection: " + msg);
    rpc.sendNotification("log.write", {
      level: "error",
      message: "unhandledRejection: " + msg,
    });
    process.exit(1);
  });
}

main().catch((err: Error) => {
  process.stderr.write("[plugin-host] fatal: " + (err.stack || err.message) + "\n");
  process.exit(1);
});
