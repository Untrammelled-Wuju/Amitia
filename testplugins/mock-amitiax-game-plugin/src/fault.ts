import type { Envelope } from '@amitia/game-plugin-sdk';

interface CrashPayload {
  exitCode?: number;
  delayMs?: number;
}

interface DelayPayload {
  delayMs?: number;
}

interface DropNextResponsePayload {
  count?: number;
}

interface HangPayload {
  hangMs?: number;
}

export class FaultInjectionService {
  private exitRequested: boolean = false;
  private exitCode: number = 0;
  private delayMs: number = 0;
  private dropNextResponseCount: number = 0;
  private hangMs: number = 0;
  private malformedFrameRequested: boolean = false;

  isExitRequested(): boolean {
    return this.exitRequested;
  }

  getExitCode(): number {
    return this.exitCode;
  }

  getDelayMs(): number {
    return this.delayMs;
  }

  shouldDropNextResponse(): boolean {
    if (this.dropNextResponseCount > 0) {
      this.dropNextResponseCount--;
      return true;
    }
    return false;
  }

  isMalformedFrameRequested(): boolean {
    return this.malformedFrameRequested;
  }

  async handleCrash(request: Envelope): Promise<unknown> {
    const payload = (request.payload as CrashPayload) || {};
    const code = payload.exitCode ?? 42;
    const delay = payload.delayMs ?? 0;
    this.exitRequested = true;
    this.exitCode = code;
    if (delay > 0) {
      const timer = setTimeout(() => {
        process.exit(code);
      }, delay);
      if (timer.unref) timer.unref();
    } else {
      process.exit(code);
    }
    return { crashing: true, exitCode: code, delayMs: delay };
  }

  async handleDelay(request: Envelope): Promise<unknown> {
    const payload = (request.payload as DelayPayload) || {};
    const delay = payload.delayMs ?? 100;
    this.delayMs = delay;
    const start = Date.now();
    while (Date.now() - start < delay) {
      // busy wait
    }
    return { delayed: true, delayMs: delay };
  }

  async handleMalformedFrame(): Promise<unknown> {
    this.malformedFrameRequested = true;
    return { malformed: true, note: 'next frame will be intentionally malformed' };
  }

  async handleDropNextResponse(request: Envelope): Promise<unknown> {
    const payload = (request.payload as DropNextResponsePayload) || {};
    const count = payload.count ?? 1;
    this.dropNextResponseCount = count;
    return { dropCount: count, note: `next ${count} response(s) will be dropped` };
  }

  async handleHang(request: Envelope): Promise<unknown> {
    const payload = (request.payload as HangPayload) || {};
    const hangMs = payload.hangMs ?? 5000;
    this.hangMs = hangMs;
    await new Promise<void>((resolve) => {
      const timer = setTimeout(resolve, hangMs);
      if (timer.unref) timer.unref();
    });
    return { hung: true, hangMs };
  }

  async handleReset(): Promise<{ reset: boolean }> {
    this.exitRequested = false;
    this.exitCode = 0;
    this.delayMs = 0;
    this.dropNextResponseCount = 0;
    this.hangMs = 0;
    this.malformedFrameRequested = false;
    return { reset: true };
  }

  async getStatus(): Promise<Record<string, unknown>> {
    return {
      exitRequested: this.exitRequested,
      exitCode: this.exitCode,
      delayMs: this.delayMs,
      dropNextResponseCount: this.dropNextResponseCount,
      hangMs: this.hangMs,
      malformedFrameRequested: this.malformedFrameRequested,
    };
  }
}

export async function handleHostNotification(
  service: FaultInjectionService,
  notification: Envelope,
): Promise<void> {
  const method = notification.method;
  if (method === 'host.authority.emergency') {
    service.handleReset();
  }
  await Promise.resolve();
}
