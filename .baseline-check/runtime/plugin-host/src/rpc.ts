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
  private pending: Map<number, (response: JsonRpcMessage) => void> = new Map();
  private nextId: number = 1;
  private closed: boolean = false;

  constructor() {
    this.rl = readline.createInterface({ input: process.stdin, terminal: false });
    this.rl.on("line", (line: string) => this.handleLine(line));
    this.rl.on("close", () => {
      this.closed = true;
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

  public sendRequest(method: string, params?: any): Promise<any> {
    return new Promise((resolve, reject) => {
      const id = this.nextId++;
      this.pending.set(id, (response: JsonRpcMessage) => {
        if (response.error) {
          reject(new RpcError(response.error.code, response.error.message, response.error.data));
        } else {
          resolve(response.result);
        }
      });
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
    if (message.id !== undefined && (message.result !== undefined || message.error !== undefined)) {
      const handler = this.pending.get(message.id as number);
      if (handler) {
        this.pending.delete(message.id as number);
        handler(message);
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
