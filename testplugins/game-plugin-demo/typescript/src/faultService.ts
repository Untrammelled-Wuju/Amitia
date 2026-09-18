import type { Envelope, Client, SecretQueryResult } from '@amitia/game-plugin-sdk';
import { querySecretLease } from '@amitia/game-plugin-sdk';

interface CrashPayload {
  exitCode?: number;
  delayMs?: number;
}

interface HandshakeDelayPayload {
  delayMs?: number;
  dropCount?: number;
}

interface HeartbeatPausePayload {
  pauseMs?: number;
}

interface HeartbeatDropPayload {
  count?: number;
}

interface RPCStallPayload {
  stallMs?: number;
}

interface QueueFillPayload {
  count?: number;
}

interface IgnoreShutdownPayload {
  ignoreMs?: number;
}

export class FaultInjectionService {
  private client: Client;
  private emergencyNotified: boolean = false;
  private emergencyMethod: string = '';
  private emergencyPayload: unknown = null;
  private handshakeDelayMs: number = 0;
  private handshakeDropCount: number = 0;
  private heartbeatPaused: boolean = false;
  private heartbeatPauseTimer: NodeJS.Timeout | null = null;
  private heartbeatDropRemaining: number = 0;
  private shutdownIgnoreUntil: number = 0;
  private shutdownIgnoreTimer: NodeJS.Timeout | null = null;
  private activeHandleCount: number = 0;
  private activeTimerCount: number = 0;
  private activeListenerCount: number = 0;

  constructor(client: Client) {
    this.client = client;
  }

  async handleCrash(_request: Envelope): Promise<unknown> {
    const payload = (_request.payload as CrashPayload) || {};
    const exitCode = payload.exitCode ?? 1;
    const delayMs = payload.delayMs ?? 0;
    if (delayMs > 0) {
      this.activeTimerCount++;
      const timer = setTimeout(() => {
        this.activeTimerCount--;
        process.exit(exitCode);
      }, delayMs);
      if (timer.unref) {
        timer.unref();
      }
    } else {
      process.exit(exitCode);
    }
    return { crashing: true, exitCode, delayMs };
  }

  async handleHandshakeDelay(_request: Envelope): Promise<unknown> {
    const payload = (_request.payload as HandshakeDelayPayload) || {};
    if (payload.delayMs !== undefined) {
      this.handshakeDelayMs = payload.delayMs;
    }
    if (payload.dropCount !== undefined) {
      this.handshakeDropCount = payload.dropCount;
    }
    return {
      handshakeDelayMs: this.handshakeDelayMs,
      handshakeDropCount: this.handshakeDropCount,
    };
  }

  async handleHeartbeatPause(_request: Envelope): Promise<unknown> {
    const payload = (_request.payload as HeartbeatPausePayload) || {};
    const pauseMs = payload.pauseMs ?? 0;
    this.heartbeatPaused = true;
    if (pauseMs > 0) {
      if (this.heartbeatPauseTimer) {
        clearTimeout(this.heartbeatPauseTimer);
        this.activeTimerCount--;
        this.heartbeatPauseTimer = null;
      }
      this.activeTimerCount++;
      this.heartbeatPauseTimer = setTimeout(() => {
        this.activeTimerCount--;
        this.heartbeatPaused = false;
        this.heartbeatPauseTimer = null;
      }, pauseMs);
      this.heartbeatPauseTimer?.unref?.();
    }
    return { paused: true, pauseMs };
  }

  async handleHeartbeatResume(): Promise<unknown> {
    this.heartbeatPaused = false;
    if (this.heartbeatPauseTimer) {
      clearTimeout(this.heartbeatPauseTimer);
      this.activeTimerCount--;
      this.heartbeatPauseTimer = null;
    }
    return { paused: false };
  }

  async handleHeartbeatDrop(_request: Envelope): Promise<unknown> {
    const payload = (_request.payload as HeartbeatDropPayload) || {};
    const count = payload.count ?? 1;
    this.heartbeatDropRemaining += count;
    return { dropped: count, remaining: this.heartbeatDropRemaining };
  }

  async handleRPCStall(_request: Envelope): Promise<unknown> {
    const payload = (_request.payload as RPCStallPayload) || {};
    const stallMs = payload.stallMs ?? 0;
    const start = Date.now();
    while (Date.now() - start < stallMs) {
      // synchronous busy-wait
    }
    return { stalled: stallMs };
  }

  async handleQueueFill(_request: Envelope): Promise<unknown> {
    const payload = (_request.payload as QueueFillPayload) || {};
    const count = payload.count ?? 1;
    let sent = 0;
    this.activeHandleCount += count;
    for (let i = 0; i < count; i++) {
      try {
        await this.client.sendRequest(`mock.fault.queue.fill.${i}`, {
          index: i,
          fault: true,
        });
      } catch {
        // ignore send errors
      }
      sent++;
    }
    this.activeHandleCount -= count;
    return { sent, requested: count };
  }

  async handleSecretProbe(request: Envelope): Promise<SecretQueryResult | Record<string, unknown>> {
    const payload = (request.payload as { leaseId?: string }) || {};
    if (!payload.leaseId) {
      return {
        granted: false,
        valid: false,
        reason: 'lease_id_required_for_query',
      };
    }
    return querySecretLease(this.client, { leaseId: payload.leaseId });
  }

  async handleControlIgnoreShutdown(_request: Envelope): Promise<unknown> {
    const payload = (_request.payload as IgnoreShutdownPayload) || {};
    const ignoreMs = payload.ignoreMs ?? 0;
    this.shutdownIgnoreUntil = Date.now() + ignoreMs;
    if (this.shutdownIgnoreTimer) {
      clearTimeout(this.shutdownIgnoreTimer);
      this.activeTimerCount--;
      this.shutdownIgnoreTimer = null;
    }
    if (ignoreMs > 0) {
      this.activeTimerCount++;
      this.shutdownIgnoreTimer = setTimeout(() => {
        this.activeTimerCount--;
        this.shutdownIgnoreUntil = 0;
        this.shutdownIgnoreTimer = null;
      }, ignoreMs);
      this.shutdownIgnoreTimer?.unref?.();
    }
    return { ignoring: true, ignoreMs, until: this.shutdownIgnoreUntil };
  }

  handleEmergencyNotification(notification: Envelope): void {
    this.emergencyNotified = true;
    this.emergencyMethod = notification.method || '';
    this.emergencyPayload = notification.payload;
  }

  async handleResidueCounts(): Promise<unknown> {
    return {
      activeHandles: this.activeHandleCount,
      timers: this.activeTimerCount,
      listeners: this.activeListenerCount,
    };
  }

  clearAll(): void {
    this.emergencyNotified = false;
    this.emergencyMethod = '';
    this.emergencyPayload = null;
    this.handshakeDelayMs = 0;
    this.handshakeDropCount = 0;
    this.heartbeatPaused = false;
    this.heartbeatDropRemaining = 0;
    this.shutdownIgnoreUntil = 0;
    if (this.heartbeatPauseTimer) {
      clearTimeout(this.heartbeatPauseTimer);
      this.heartbeatPauseTimer = null;
    }
    if (this.shutdownIgnoreTimer) {
      clearTimeout(this.shutdownIgnoreTimer);
      this.shutdownIgnoreTimer = null;
    }
    this.activeHandleCount = 0;
    this.activeTimerCount = 0;
    this.activeListenerCount = 0;
  }
}

export async function handleNotification(
  service: FaultInjectionService,
  notification: Envelope,
): Promise<void> {
  const method = notification.method;
  if (
    method === 'host.authority.emergency' ||
    method === 'host.authority.emergy' ||
    method === 'control.graceful_shutdown'
  ) {
    service.handleEmergencyNotification(notification);
  }
  await Promise.resolve();
}
