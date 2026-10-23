export interface InstallationOperationState {
  operationId?: string;
  installationId?: string;
  status: string;
  stage?: string;
  errorCode?: string;
  errorMessage?: string;
}

interface TrackInstallationOperationOptions {
  attempts?: number;
  intervalMs?: number;
  sleep?: (milliseconds: number) => Promise<void>;
}

const activeStatuses = new Set([
  "created",
  "queued",
  "running",
  "waiting_runtime_ack",
  "failed_retryable",
  "cancel_requested",
]);

function operationFailure(operation: InstallationOperationState): Error {
  const code = operation.errorCode?.trim() || operation.status;
  const message = operation.errorMessage?.trim();
  return new Error(message || `桌宠操作失败：${code}`);
}

export async function trackInstallationOperation(
  submission: InstallationOperationState,
  load: (operationId: string) => Promise<InstallationOperationState | null>,
  options: TrackInstallationOperationOptions = {},
): Promise<InstallationOperationState> {
  const attempts = Math.max(1, options.attempts ?? 60);
  const intervalMs = Math.max(0, options.intervalMs ?? 250);
  const sleep =
    options.sleep ??
    ((milliseconds: number) =>
      new Promise<void>((resolve) => window.setTimeout(resolve, milliseconds)));
  let current = submission;
  for (let attempt = 0; attempt < attempts; attempt += 1) {
    if (current.status === "failed_terminal" || current.status === "cancelled") {
      throw operationFailure(current);
    }
    if (!current.operationId || !activeStatuses.has(current.status)) {
      return current;
    }
    await sleep(intervalMs);
    const next = await load(current.operationId);
    if (!next) return current;
    current = next;
  }
  return current;
}
