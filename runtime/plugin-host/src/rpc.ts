import * as readline from "readline";

export interface JsonRpcMessage {
  jsonrpc: "2.0";
  id?: string | number;
  method?: string;
  params?: any;
  result?: any;
  error?: RpcErrorData;
}

export interface RpcErrorData {
  code: number;
  message: string;
  data?: any;
}

export type MessageHandler = (message: JsonRpcMessage) => void;

export const DEFAULT_REQUEST_TIMEOUT_MS = 30000;

interface PendingCall {
  resolve: (value: any) => void;
  reject: (error: Error) => void;
  timer: ReturnType<typeof setTimeout>;
}

export class RpcError extends Error {
  public code: number;
  public data: any;

  constructor(code: number, message: string, data?: any) {
    super(message);
    this.code = code;
    this.data = data;
  }
}

export class RpcConnection {
  private rl: readline.Interface;
  private handlers: Map<string, MessageHandler> = new Map();
  private pending: Map<number, PendingCall> = new Map();
  private nextId: number = 1;
  private closed: boolean = false;

  constructor() {
    this.rl = readline.createInterface({ input: process.stdin, terminal: false });
    this.rl.on("line", (line: string) => this.handleLine(line));
    this.rl.on("close", () => {
      this.closed = true;
      this.rejectAllPending(new Error("rpc connection closed"));
    });
  }

  public on(method: string, handler: MessageHandler): void {
    this.handlers.set(method, handler);
  }

  public sendNotification(method: string, params?: any): void {
    const msg: any = { jsonrpc: "2.0", method };
    if (params !== undefined) {
      msg.params = params;
    }
    this.write(msg);
  }

  public sendRequest(method: string, params?: any, timeoutMs: number = DEFAULT_REQUEST_TIMEOUT_MS): Promise<any> {
    return new Promise((resolve, reject) => {
      const id = this.nextId++;
      const timer = setTimeout(() => {
        if (!this.pending.delete(id)) {
          return;
        }
        reject(new Error(`rpc request timeout: ${method} (${timeoutMs}ms)`));
      }, timeoutMs);
      if (typeof (timer as any).unref === "function") {
        (timer as any).unref();
      }
      this.pending.set(id, { resolve, reject, timer });
      const msg: any = { jsonrpc: "2.0", id, method };
      if (params !== undefined) {
        msg.params = params;
      }
      this.write(msg);
    });
  }

  public sendResult(id: string | number, result: any): void {
    this.write({ jsonrpc: "2.0", id, result });
  }

  public sendError(id: string | number, code: number, message: string, data?: any): void {
    const error: any = { code, message };
    if (data !== undefined) {
      error.data = data;
    }
    this.write({ jsonrpc: "2.0", id, error });
  }

  public isClosed(): boolean {
    return this.closed;
  }

  public close(): void {
    this.rl.close();
    this.rejectAllPending(new Error("rpc connection closed"));
  }

  private rejectAllPending(error: Error): void {
    for (const [id, pending] of this.pending) {
      clearTimeout(pending.timer);
      this.pending.delete(id);
      try {
        pending.reject(error);
      } catch (e) {
      }
    }
  }

  private write(message: any): void {
    if (this.closed) {
      return;
    }
    try {
      process.stdout.write(JSON.stringify(message) + "\n");
    } catch (e) {
      this.closed = true;
    }
  }

  private handleLine(line: string): void {
    const trimmed = line.trim();
    if (!trimmed) {
      return;
    }
    let message: JsonRpcMessage;
    try {
      message = JSON.parse(trimmed);
    } catch (e) {
      return;
    }
    if (!message || message.jsonrpc !== "2.0") {
      return;
    }
    if (message.id !== undefined && message.method === undefined && (message.result !== undefined || message.error !== undefined)) {
      const pending = this.pending.get(message.id as number);
      if (pending) {
        this.pending.delete(message.id as number);
        clearTimeout(pending.timer);
        if (message.error) {
          pending.reject(new RpcError(message.error.code, message.error.message, message.error.data));
        } else {
          pending.resolve(message.result);
        }
      }
      return;
    }
    if (message.method) {
      const handler = this.handlers.get(message.method);
      if (handler) {
        try {
          handler(message);
        } catch (e) {
          if (message.id !== undefined) {
            this.sendError(message.id, -32603, (e as Error).message || "Internal error");
          }
        }
      } else if (message.id !== undefined) {
        this.sendError(message.id, -32601, "Method not found: " + message.method);
      }
    }
  }
}
