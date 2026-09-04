import type { RuntimeEnvelope } from "../../desktop-pet/runtime/protocol-v2";

export type CommandExecutionStatus =
  | "accepted"
  | "applied"
  | "rejected"
  | "failed"
  | "duplicate"
  | "expired"
  | "cancelled";

export interface RuntimeCommandExecutionResult {
  commandId: string;
  status: CommandExecutionStatus;
  errorCode: string;
  errorMessage: string;
  appliedRevision: number;
  actualState?: unknown;
  acceptedAction?: string;
  playbackRequestId?: string;
}

export type RuntimeCommandExecutor = (
  command: unknown,
  envelope: RuntimeEnvelope,
) => Promise<RuntimeCommandExecutionResult>;

export interface RuntimeV2CommandAdapterOptions {
  executor: RuntimeCommandExecutor;
  onError?: (err: Error, commandId: string) => void;
}

export class RuntimeV2CommandAdapter {
  private executor: RuntimeCommandExecutor;
  private onError?: (err: Error, commandId: string) => void;

  constructor(options: RuntimeV2CommandAdapterOptions) {
    this.executor = options.executor;
    this.onError = options.onError;
  }

  async handleCommand(
    command: unknown,
    envelope: RuntimeEnvelope,
  ): Promise<RuntimeCommandExecutionResult> {
    const cmd = command as { commandId?: string } | undefined;
    const commandId = cmd?.commandId ?? "";

    try {
      const result = await this.executor(command, envelope);
      return result;
    } catch (err) {
      const error = err instanceof Error ? err : new Error(String(err));
      this.onError?.(error, commandId);
      return {
        commandId,
        status: "failed",
        errorCode: "COMMAND_EXECUTION_ERROR",
        errorMessage: error.message,
        appliedRevision: 0,
      };
    }
  }
}

export function createCommandAdapter(
  executor: RuntimeCommandExecutor,
  onError?: (err: Error, commandId: string) => void,
): RuntimeV2CommandAdapter {
  return new RuntimeV2CommandAdapter({ executor, onError });
}
