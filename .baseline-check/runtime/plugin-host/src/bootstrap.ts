import { RpcConnection } from "./rpc";
import { loadExtension, LoadedExtension } from "./module-loader";
import { createRuntimeContext } from "./runtime-context";
import { HandlerRegistry } from "./handler-registry";

export interface BootstrapSpec {
  entry: string;
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
  const extension = loadExtension(spec.entry);
  if (typeof extension.activate !== "function") {
    throw new Error("Extension activate is not a function");
  }
  await extension.activate(context);
  return extension;
}
