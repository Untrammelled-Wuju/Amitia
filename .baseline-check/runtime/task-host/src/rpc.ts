import { randomUUID } from "node:crypto";

export interface JsonRpcRequest {
  jsonrpc: "2.0";
  id?: string | number;
  method: string;
  params?: unknown;
}

export interface JsonRpcResponse {
  jsonrpc: "2.0";
  id: string | number;
  result?: unknown;
  error?: JsonRpcError;
}

export interface JsonRpcError {
  code: number;
  message: string;
  data?: unknown;
}

export interface JsonRpcNotification {
  jsonrpc: "2.0";
  method: string;
  params?: unknown;
}

export type JsonRpcMessage = JsonRpcRequest | JsonRpcResponse | JsonRpcNotification;

export interface RpcTransport {
  write(data: string): void;
  onMessage(handler: (message: JsonRpcMessage) => void): void;
  onClose(handler: () => void): void;
}

export class StdioRpcTransport implements RpcTransport {
  private buffer: string = "";
  private readonly messageHandlers: Array<(message: JsonRpcMessage) => void> = [];
  private readonly closeHandlers: Array<() => void> = [];
  private closed: boolean = false;
  private started: boolean = false;

  constructor(
    private readonly input: NodeJS.ReadStream = process.stdin,
    private readonly output: NodeJS.WriteStream = process.stdout,
  ) {}

  start(): void {
    if (this.started) return;
    this.started = true;
    this.input.setEncoding("utf-8");
    this.input.on("data", (chunk: string) => this.onData(chunk));
    this.input.on("close", () => this.handleClose());
    this.input.on("end", () => this.handleClose());
    this.input.on("error", () => this.handleClose());
    this.input.resume();
  }

  private onData(chunk: string): void {
    this.buffer += chunk;
    let newlineIndex: number;
    while ((newlineIndex = this.buffer.indexOf("\n")) >= 0) {
      const line = this.buffer.slice(0, newlineIndex).trim();
      this.buffer = this.buffer.slice(newlineIndex + 1);
      if (!line) continue;
      try {
        const message = JSON.parse(line) as JsonRpcMessage;
        for (const handler of this.messageHandlers) {
          handler(message);
        }
      } catch {
      }
    }
  }

  private handleClose(): void {
    if (this.closed) return;
    this.closed = true;
    for (const handler of this.closeHandlers) {
      handler();
    }
  }

  write(data: string): void {
    this.output.write(data + "\n");
  }

  onMessage(handler: (message: JsonRpcMessage) => void): void {
    this.messageHandlers.push(handler);
  }

  onClose(handler: () => void): void {
    this.closeHandlers.push(handler);
  }

  isClosed(): boolean {
    return this.closed;
  }
}

interface PendingCall {
  resolve: (value: unknown) => void;
  reject: (error: Error) => void;
  timer: ReturnType<typeof setTimeout>;
}

export class RpcClient {
  private readonly pending: Map<string | number, PendingCall> = new Map();
  private readonly notificationHandlers: Map<string, Array<(params: unknown) => void>> = new Map();
  private readonly requestHandlers: Map<string, (params: unknown) => Promise<unknown>> = new Map();
  private closed: boolean = false;

  constructor(private readonly transport: RpcTransport) {
    this.transport.onMessage((message) => this.handleMessage(message));
    this.transport.onClose(() => this.handleClose());
  }

  private isResponse(message: JsonRpcMessage): message is JsonRpcResponse {
    return "id" in message && message.id !== undefined && !("method" in message);
  }

  private isRequest(message: JsonRpcMessage): message is JsonRpcRequest {
    return "method" in message && "id" in message && message.id !== undefined;
  }

  private isNotification(message: JsonRpcMessage): message is JsonRpcNotification {
    return "method" in message && (!("id" in message) || message.id === undefined);
  }

  private handleMessage(message: JsonRpcMessage): void {
    if (this.isResponse(message)) {
      const pending = this.pending.get(message.id);
      if (pending) {
        clearTimeout(pending.timer);
        this.pending.delete(message.id);
        if (message.error) {
          pending.reject(new Error(message.error.message));
        } else {
          pending.resolve(message.result);
        }
      }
      return;
    }

    if (this.isRequest(message)) {
      const handler = this.requestHandlers.get(message.method);
      const id = message.id!;
      if (handler) {
        handler(message.params)
          .then((result) => {
            this.sendResponse(id, result);
          })
          .catch((error: Error) => {
            this.sendErrorResponse(id, -32000, error.message);
          });
      } else {
        this.sendErrorResponse(id, -32601, `method not found: ${message.method}`);
      }
      return;
    }

    if (this.isNotification(message)) {
      const handlers = this.notificationHandlers.get(message.method);
      if (handlers) {
        for (const handler of handlers) {
          handler(message.params);
        }
      }
    }
  }

  private sendResponse(id: string | number, result: unknown): void {
    const response: JsonRpcResponse = { jsonrpc: "2.0", id, result };
    this.transport.write(JSON.stringify(response));
  }

  private sendErrorResponse(id: string | number, code: number, message: string): void {
    const response: JsonRpcResponse = {
      jsonrpc: "2.0",
      id,
      error: { code, message },
    };
    this.transport.write(JSON.stringify(response));
  }

  private handleClose(): void {
    if (this.closed) return;
    this.closed = true;
    for (const [, pending] of this.pending) {
      clearTimeout(pending.timer);
      pending.reject(new Error("rpc connection closed"));
    }
    this.pending.clear();
  }

  call<T = unknown>(method: string, params?: unknown, timeoutMs: number = 30000): Promise<T> {
    if (this.closed) {
      return Promise.reject(new Error("rpc connection closed"));
    }
    const id = randomUUID();
    const request: JsonRpcRequest = { jsonrpc: "2.0", id, method, params };
    return new Promise<T>((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new Error(`rpc call timeout: ${method}`));
      }, timeoutMs);
      this.pending.set(id, {
        resolve: (value: unknown) => resolve(value as T),
        reject,
        timer,
      });
      this.transport.write(JSON.stringify(request));
    });
  }

  notify(method: string, params?: unknown): void {
    if (this.closed) return;
    const notification: JsonRpcNotification = { jsonrpc: "2.0", method, params };
    this.transport.write(JSON.stringify(notification));
  }

  onNotification(method: string, handler: (params: unknown) => void): void {
    let handlers = this.notificationHandlers.get(method);
    if (!handlers) {
      handlers = [];
      this.notificationHandlers.set(method, handlers);
    }
    handlers.push(handler);
  }

  onRequest(method: string, handler: (params: unknown) => Promise<unknown>): void {
    this.requestHandlers.set(method, handler);
  }

  onceNotification(method: string, timeoutMs: number = 60000): Promise<unknown> {
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.removeNotificationHandler(method, wrappedHandler);
        reject(new Error(`timeout waiting for notification: ${method}`));
      }, timeoutMs);

      const wrappedHandler = (params: unknown) => {
        clearTimeout(timer);
        this.removeNotificationHandler(method, wrappedHandler);
        resolve(params);
      };

      let handlers = this.notificationHandlers.get(method);
      if (!handlers) {
        handlers = [];
        this.notificationHandlers.set(method, handlers);
      }
      handlers.push(wrappedHandler);
    });
  }

  private removeNotificationHandler(method: string, handler: (params: unknown) => void): void {
    const handlers = this.notificationHandlers.get(method);
    if (!handlers) return;
    const idx = handlers.indexOf(handler);
    if (idx >= 0) {
      handlers.splice(idx, 1);
    }
  }

  isClosed(): boolean {
    return this.closed;
  }
}
