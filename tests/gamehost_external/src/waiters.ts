export interface WaitOptions {
  timeoutMs?: number;
  intervalMs?: number;
  signal?: AbortSignal;
}

export class TimeoutError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'TimeoutError';
  }
}

export class AbortError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'AbortError';
  }
}

export async function waitUntil(
  predicate: () => Promise<boolean>,
  options: WaitOptions = {}
): Promise<void> {
  const timeoutMs = options.timeoutMs ?? 10000;
  const intervalMs = options.intervalMs ?? 200;
  const start = Date.now();

  while (true) {
    if (options.signal?.aborted) {
      throw new AbortError('waitUntil aborted');
    }
    if (await predicate()) {
      return;
    }
    if (Date.now() - start >= timeoutMs) {
      throw new TimeoutError(`waitUntil timed out after ${timeoutMs}ms`);
    }
    await sleep(intervalMs);
  }
}

export async function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(resolve, ms);
    signal?.addEventListener('abort', () => {
      clearTimeout(timer);
      reject(new AbortError('sleep aborted'));
    }, { once: true });
  });
}

export async function waitForCondition<T>(
  fetcher: () => Promise<T>,
  predicate: (value: T) => boolean,
  options: WaitOptions = {}
): Promise<T> {
  const timeoutMs = options.timeoutMs ?? 10000;
  const intervalMs = options.intervalMs ?? 200;
  const start = Date.now();
  let lastValue: T | undefined;

  while (true) {
    if (options.signal?.aborted) {
      throw new AbortError('waitForCondition aborted');
    }
    try {
      lastValue = await fetcher();
      if (predicate(lastValue)) {
        return lastValue;
      }
    } catch {
      // ignore transient errors during wait
    }
    if (Date.now() - start >= timeoutMs) {
      throw new TimeoutError(`waitForCondition timed out after ${timeoutMs}ms`);
    }
    await sleep(intervalMs);
  }
}
