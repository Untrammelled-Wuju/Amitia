import { RpcConnection } from "./rpc";
import { loadExtension, LoadedExtension } from "./module-loader";
import { createRuntimeContext } from "./runtime-context";
import { HandlerRegistry } from "./handler-registry";

export interface BootstrapSpec {
  entry: string;
  extensionId?: string;
  moduleId?: string;
  generation?: number;
  traceId?: string;
  resourceId?: string;
  resourceLimits?: {
    maxHeapSizeMb?: number;
    timeoutMs?: number;
  };
}

export async function bootstrap(
  spec: BootstrapSpec,
  rpc: RpcConnection,
  registry: HandlerRegistry
): Promise<LoadedExtension> {
  if (!spec || !spec.entry) {
    throw new Error("Bootstrap spec is missing entry path");
  }
  const context = createRuntimeContext(rpc, registry);
  const extension = loadExtension(spec.entry, {
    networkDisabled: process.env.AMITIA_NETWORK_DISABLED === "1",
    filesystemDisabled: process.env.AMITIA_HOST_FILESYSTEM_MODE !== "read" && process.env.AMITIA_HOST_FILESYSTEM_MODE !== "read_write",
    processDisabled: process.env.AMITIA_HOST_PROCESS_MODE !== "direct",
  });
  if (typeof extension.activate !== "function") {
    throw new Error(
      `Extension activate is not a function: type=${typeof extension} activateType=${typeof (extension as any).activate} keys=${Object.keys(extension).join(",")} entry=${spec.entry}`
    );
  }
  await extension.activate(context);
  bindSdkTools(extension, registry, spec);
  return extension;
}

function bindSdkTools(extension: LoadedExtension, registry: HandlerRegistry, spec: BootstrapSpec): void {
  const tools = extension.sdk?.listTools?.();
  if (!Array.isArray(tools)) {
    return;
  }
  for (const tool of tools) {
    const definition = tool?.definition;
    const configuredName = definition?.metadata?.handlerName;
    const toolId = String(definition?.toolId ?? "");
    const entryName = String(configuredName ?? toolId.split("/").pop() ?? "").trim();
    if (!entryName || registry.has(entryName)) {
      continue;
    }
    registry.bindTool(entryName, async (input, invocation) => {
      const result = await tool.handler(input, {
        runtime: {
          extensionId: spec.extensionId ?? "",
          moduleId: spec.moduleId ?? "",
          generation: spec.generation ?? 0,
          platform: process.platform,
          locale: "default",
          traceId: spec.traceId ?? "",
          scope: { scope: "global" },
        },
        toolCallId: invocation.invocationId,
      });
      if (result && typeof result === "object" && "ok" in result) {
        if (!result.ok) {
          const error = result.error ?? {};
          throw new Error(String(error.message ?? error.code ?? "tool invocation failed"));
        }
        return result.result;
      }
      return result;
    });
  }
}
