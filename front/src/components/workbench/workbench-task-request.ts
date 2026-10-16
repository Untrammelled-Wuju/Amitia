export async function withTaskRequestDeadline<T>(
  request: (signal: AbortSignal) => Promise<T>,
  signal: AbortSignal,
  timeoutMs = 25000,
): Promise<T> {
  const controller = new AbortController();
  const cancel = () => controller.abort();
  signal.addEventListener('abort', cancel, { once: true });
  if (signal.aborted) cancel();
  const timeout = setTimeout(cancel, timeoutMs);
  let onAbort: (() => void) | undefined;
  const cancellation = new Promise<never>((_, reject) => {
    onAbort = () => reject(new DOMException('任务请求已取消或超时', 'AbortError'));
    controller.signal.addEventListener('abort', onAbort, { once: true });
    if (controller.signal.aborted) onAbort();
  });
  try {
    return await Promise.race([controller.signal.aborted ? cancellation : request(controller.signal), cancellation]);
  } finally {
    clearTimeout(timeout);
    signal.removeEventListener('abort', cancel);
    if (onAbort) controller.signal.removeEventListener('abort', onAbort);
  }
}
