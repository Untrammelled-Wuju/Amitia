import { HandlerRegistry, ToolHandler, InvocationContext } from "./handler-registry";
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

export interface ExtensionContext {
  handlers: ExtensionHandlers;
  host: HostApi;
  log: LogApi;
}

export function createRuntimeContext(rpc: RpcConnection, registry: HandlerRegistry): ExtensionContext {
  return {
    handlers: {
      bindTool: (name: string, handler: ToolHandler) => {
        registry.bindTool(name, handler);
      },
    },
    host: {
      call: (method: string, params?: any) => {
        return rpc.sendRequest("host.call", { method, params });
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
