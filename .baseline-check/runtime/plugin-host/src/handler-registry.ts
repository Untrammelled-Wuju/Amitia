export interface InvocationContext {
  invocationId: string;
  isCanceled: boolean;
  onCancel: (handler: () => void) => void;
}

export type ToolHandler = (input: any, invocation: InvocationContext) => Promise<any>;

export class HandlerRegistry {
  private tools: Map<string, ToolHandler> = new Map();

  public bindTool(name: string, handler: ToolHandler): void {
    if (!name || typeof name !== "string") {
      throw new Error("Tool name must be a non-empty string");
    }
    if (typeof handler !== "function") {
      throw new Error("Tool handler must be a function");
    }
    this.tools.set(name, handler);
  }

  public has(name: string): boolean {
    return this.tools.has(name);
  }

  public get(name: string): ToolHandler | undefined {
    return this.tools.get(name);
  }

  public list(): string[] {
    return Array.from(this.tools.keys());
  }

  public clear(): void {
    this.tools.clear();
  }
}
