import { activeInvocation, HandlerRegistry, ToolHandler, InvocationContext } from "./handler-registry";
import { RpcConnection } from "./rpc";

export interface ExtensionHandlers {
  bindTool(name: string, handler: ToolHandler): void;
}

export interface HostApi {
  call(method: string, params?: any): Promise<any>;
}

export interface LogApi {
  write(level: string, message: string, fields?: Record<string, any>): void;
  info(message: string, fields?: Record<string, any>): void;
  warn(message: string, fields?: Record<string, any>): void;
  error(message: string, fields?: Record<string, any>): void;
}

export interface ContributionRegistrationRequest {
  id: string;
  kind: string;
  adapter: string;
  contractVersion: number;
  schema?: Record<string, unknown>;
  name: Record<string, string>;
  description?: Record<string, string>;
  version?: string;
  spec?: Record<string, unknown>;
  requiredPermissions?: string[];
  requiredScope?: string[];
  metadata?: Record<string, unknown>;
}

export interface ContributionRegistration {
  id: string;
  dispose(): Promise<void>;
}

export interface ServiceOffer {
  serviceId: string;
  version: string;
  contractHash?: string;
  scopes: Array<"global" | "character" | "conversation" | "device" | "plugin">;
  placement: "core" | "device" | "cloud" | "hybrid";
  priority?: number;
}

export interface ServiceRequirement {
  serviceId: string;
  versionRange?: string;
  contractHash?: string;
  optional?: boolean;
  selection?: "single" | "all" | "exclusive";
  reason?: string;
}

export interface ServiceLease {
  serviceId: string;
  providerIds: string[];
  release(): Promise<void>;
}

export interface ContributionsApi {
  register(request: ContributionRegistrationRequest): Promise<ContributionRegistration>;
  unregister(id: string): Promise<void>;
}

export interface ServicesApi {
  provide(offer: ServiceOffer): Promise<() => Promise<void>>;
  require(requirement: ServiceRequirement): Promise<ServiceLease>;
}

export interface AdapterInvocationRequest {
  invocationId?: string;
  contributionId?: string;
  operation: string;
  input?: unknown;
}

export interface AdapterApi {
  invoke(adapterId: string, contractVersion: number, request: AdapterInvocationRequest): Promise<{ invocationId: string; output?: unknown }>;
  cancel(adapterId: string, contractVersion: number, invocationId: string, reason?: string): Promise<void>;
}

export interface ExtensionContext {
  handlers: ExtensionHandlers;
  host: HostApi;
  log: LogApi;
  contributions: ContributionsApi;
  services: ServicesApi;
  adapters: AdapterApi;
}

export function createRuntimeContext(rpc: RpcConnection, registry: HandlerRegistry): ExtensionContext {
  const callHost = (method: string, input: any) => {
    return rpc.sendRequest("host.call", {
      method,
      version: 1,
      input,
      invocationId: activeInvocation.getStore()?.invocationId ?? "",
    });
  };
  return {
    handlers: {
      bindTool: (name: string, handler: ToolHandler) => {
        registry.bindTool(name, handler);
      },
    },
    host: {
      call: (method: string, params?: any) => callHost(method, params ?? {}),
    },
    contributions: {
      async register(request) {
        const result = await callHost("plugin.contribution.register", request);
        return {
          id: String(result.id ?? request.id),
          async dispose() {
            await callHost("plugin.contribution.unregister", { id: request.id });
          },
        };
      },
      async unregister(id) {
        await callHost("plugin.contribution.unregister", { id });
      },
    },
    services: {
      async provide(offer) {
        await callHost("plugin.service.provide", offer);
        return async () => {
          await callHost("plugin.service.unprovide", { serviceId: offer.serviceId });
        };
      },
      async require(requirement) {
        const result = await callHost("plugin.service.require", requirement);
        return {
          serviceId: requirement.serviceId,
          providerIds: Array.isArray(result.providerIds) ? result.providerIds.map(String) : [],
          async release() {
            await callHost("plugin.service.release", { serviceId: requirement.serviceId });
          },
        };
      },
    },
    adapters: {
      async invoke(adapterId, contractVersion, request) {
        return await callHost("plugin.adapter.invoke", {
          adapterId,
          contractVersion,
          ...request,
        });
      },
      async cancel(adapterId, contractVersion, invocationId, reason) {
        await callHost("plugin.adapter.cancel", {
          adapterId,
          contractVersion,
          invocationId,
          reason,
        });
      },
    },
    log: {
      write: (level: string, message: string, fields?: Record<string, any>) => {
        rpc.sendNotification("log.write", { level, message, fields });
      },
      info: (message: string, fields?: Record<string, any>) => {
        rpc.sendNotification("log.write", { level: "info", message, fields });
      },
      warn: (message: string, fields?: Record<string, any>) => {
        rpc.sendNotification("log.write", { level: "warn", message, fields });
      },
      error: (message: string, fields?: Record<string, any>) => {
        rpc.sendNotification("log.write", { level: "error", message, fields });
      },
    },
  };
}

export { InvocationContext };
