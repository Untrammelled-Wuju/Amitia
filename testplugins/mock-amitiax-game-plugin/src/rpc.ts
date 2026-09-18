import type { Envelope } from '@amitia/game-plugin-sdk';
import { FakeGameRuntime } from './state.js';

export class MockGameRPCService {
  private runtime: FakeGameRuntime;
  private echoCount: number = 0;

  constructor(runtime: FakeGameRuntime) {
    this.runtime = runtime;
  }

  async handleEcho(request: Envelope): Promise<unknown> {
    const payload = (request.payload as { message?: string }) || {};
    this.echoCount++;
    return {
      message: payload.message || '',
      count: this.echoCount,
      timestamp: Date.now(),
    };
  }

  async handleStatus(request: Envelope): Promise<unknown> {
    const _payload = (request.payload as { detail?: string }) || {};
    const state = this.runtime.getState();
    return {
      state,
      pluginVersion: '1.0.0',
      uptime: process.uptime(),
      status: 'ok',
    };
  }

  async handleCommand(request: Envelope): Promise<unknown> {
    const payload = (request.payload as { action?: string; value?: number }) || {};
    const action = payload.action || 'noop';

    switch (action) {
      case 'start':
        this.runtime.start();
        return { action, result: 'started' };
      case 'stop':
        this.runtime.stop();
        return { action, result: 'stopped' };
      case 'damage':
        this.runtime.applyDamage(payload.value || 10);
        return { action, result: 'damaged', health: this.runtime.getState().health };
      case 'heal':
        this.runtime.heal(payload.value || 10);
        return { action, result: 'healed', health: this.runtime.getState().health };
      case 'move':
        return { action, result: 'moved', position: { x: payload.value || 0, y: payload.value || 0 } };
      case 'reset':
        this.runtime.reset();
        return { action, result: 'reset' };
      default:
        return { action, result: 'unknown_command' };
    }
  }

  async handleLongTask(request: Envelope): Promise<unknown> {
    const payload = (request.payload as { durationMs?: number }) || {};
    const duration = payload.durationMs || 100;
    const start = Date.now();
    while (Date.now() - start < duration) {
      // busy wait
    }
    return { completed: true, duration };
  }

  async handleFail(request: Envelope): Promise<unknown> {
    const payload = (request.payload as { errorType?: string; message?: string }) || {};
    throw new Error(payload.message || `intentional failure: ${payload.errorType || 'generic'}`);
  }

  async handleBinaryConsume(request: Envelope): Promise<unknown> {
    const payload = (request.payload as { binaryId?: string; size?: number }) || {};
    return {
      consumed: true,
      binaryId: payload.binaryId || 'unknown',
      size: payload.size || 0,
      checksum: 'sha256:placeholder',
    };
  }

  getEchoCount(): number {
    return this.echoCount;
  }
}
