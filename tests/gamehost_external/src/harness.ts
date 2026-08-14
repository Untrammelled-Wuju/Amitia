import { spawn, ChildProcess } from 'child_process';
import { EventEmitter } from 'events';

interface RequestEnvelope {
  type: 'request';
  id: string;
  method: string;
  payload?: unknown;
  serviceId?: string;
  pluginId?: string;
  runtimeId?: string;
  protocol?: string;
}

interface NotificationEnvelope {
  type: 'notification';
  id: string;
  method: string;
  payload?: unknown;
  serviceId?: string;
  pluginId?: string;
}

interface ResponseEnvelope {
  type: 'response';
  id: string;
  requestId: string;
  payload?: unknown;
  error?: { code: string; message: string };
  protocol?: string;
}

interface IncomingEnvelope {
  type: string;
  id?: string;
  requestId?: string;
  method?: string;
  payload?: unknown;
  error?: { code: string; message: string };
  protocol?: string;
  pluginId?: string;
  runtimeId?: string;
  serviceId?: string;
}

interface PendingRequest {
  resolve: (value: IncomingEnvelope) => void;
  reject: (reason: Error) => void;
  timer: NodeJS.Timeout;
}

export interface ResidueSnapshot {
  processRunning: boolean;
  handshakeDone: boolean;
  responseCount: number;
  pendingCount: number;
}

export class ExternalE2EHarness {
  private pluginPath: string;
  private proc: ChildProcess | null = null;
  private stdin: any = null;
  private stdout: any = null;
  private stderr: any = null;
  private buffer: Buffer = Buffer.alloc(0);
  private rpcSeq: number = 0;
  private pending: Map<string, PendingRequest> = new Map();
  private generation: number = 0;
  private exitCode: number | null = null;
  private exited: boolean = false;
  private emitter: EventEmitter = new EventEmitter();
  private running: boolean = false;
  private handshakeDone: boolean = false;
  private handshakeWaiters: Array<() => void> = [];
  private receivedNotifications: IncomingEnvelope[] = [];
  private responseCount: number = 0;

  constructor(pluginPath: string) {
    this.pluginPath = pluginPath;
    this.emitter.setMaxListeners(0);
  }

  async start(): Promise<void> {
    const isWindows = process.platform === 'win32';
    if (isWindows && this.pluginPath.endsWith('.js')) {
      this.proc = spawn(process.execPath, [this.pluginPath], { stdio: ['pipe', 'pipe', 'pipe'] });
    } else {
      this.proc = spawn(this.pluginPath, [], { stdio: ['pipe', 'pipe', 'pipe'] });
    }
    this.stdin = this.proc.stdin;
    this.stdout = this.proc.stdout;
    this.stderr = this.proc.stderr;
    this.buffer = Buffer.alloc(0);
    this.pending = new Map();
    this.running = true;
    this.receivedNotifications = [];
    this.responseCount = 0;
    this.generation++;

    if (this.stdout) {
      this.stdout.on('data', (data: Buffer) => this.onData(data));
    }
    if (this.stderr) {
      this.stderr.on('data', () => {});
    }

    this.proc.on('exit', (code: number | null) => {
      this.exitCode = code === null ? -1 : code;
      this.exited = true;
      this.running = false;
      this.rejectAll(new Error('plugin exited with code ' + this.exitCode));
      this.emitter.emit('exit', this.exitCode);
    });

    this.proc.on('error', (err: Error) => {
      this.rejectAll(err);
    });
  }

  async callRPC(
    method: string,
    payload: unknown = {},
    timeoutMs: number = 5000
  ): Promise<IncomingEnvelope> {
    await this.waitForHandshake();
    this.rpcSeq++;
    const id = 'rpc-' + this.rpcSeq;
    const env: RequestEnvelope = {
      type: 'request',
      id,
      method,
      payload,
    };

    return new Promise<IncomingEnvelope>((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new Error('rpc timeout: ' + method));
      }, timeoutMs);

      this.pending.set(id, { resolve, reject, timer });
      this.writeFrame(env);
    });
  }

  async sendNotification(method: string, payload: unknown = {}): Promise<void> {
    await this.waitForHandshake();
    this.rpcSeq++;
    const id = 'notif-' + this.rpcSeq;
    const env: NotificationEnvelope = {
      type: 'notification',
      id,
      method,
      payload,
    };
    this.writeFrame(env);
  }

  private waitForHandshake(): Promise<void> {
    if (this.handshakeDone) {
      return Promise.resolve();
    }
    return new Promise<void>((resolve) => {
      this.handshakeWaiters.push(resolve);
    });
  }

  private completeHandshake(): void {
    this.handshakeDone = true;
    for (const waiter of this.handshakeWaiters) {
      waiter();
    }
    this.handshakeWaiters = [];
  }

  kill(): void {
    if (this.proc) {
      this.proc.kill('SIGKILL');
    }
  }

  softKill(): void {
    if (this.proc) {
      this.proc.kill('SIGTERM');
    }
  }

  async waitExit(timeoutMs: number = 5000): Promise<number> {
    if (this.exited) {
      return this.exitCode === null ? -1 : this.exitCode;
    }
    return new Promise<number>((resolve, reject) => {
      const timer = setTimeout(() => {
        reject(new Error('waitExit timed out'));
      }, timeoutMs);
      this.emitter.once('exit', (code: number) => {
        clearTimeout(timer);
        resolve(code);
      });
    });
  }

  getExitCode(): number | null {
    return this.exitCode;
  }

  getGeneration(): number {
    return this.generation;
  }

  isRunning(): boolean {
    return this.running;
  }

  isHandshakeDone(): boolean {
    return this.handshakeDone;
  }

  getResponseCount(): number {
    return this.responseCount;
  }

  getReceivedNotifications(): IncomingEnvelope[] {
    return [...this.receivedNotifications];
  }

  async restart(): Promise<void> {
    this.kill();
    try {
      await this.waitExit(3000);
    } catch {
      // force continue
    }
    this.handshakeDone = false;
    await this.start();
  }

  getResidue(): ResidueSnapshot {
    return {
      processRunning: this.running,
      handshakeDone: this.handshakeDone,
      responseCount: this.responseCount,
      pendingCount: this.pending.size,
    };
  }

  private writeFrame(env: RequestEnvelope | NotificationEnvelope | ResponseEnvelope): void {
    const payload = Buffer.from(JSON.stringify(env), 'utf8');
    const header = Buffer.alloc(4);
    header.writeUInt32BE(payload.length, 0);
    if (this.stdin) {
      this.stdin.write(Buffer.concat([header, payload]));
    }
  }

  private onData(data: Buffer): void {
    this.buffer = Buffer.concat([this.buffer, data]);
    while (true) {
      if (this.buffer.length < 4) break;
      const length = this.buffer.readUInt32BE(0);
      if (length > 16 * 1024 * 1024) {
        this.rejectAll(new Error('frame exceeds max size'));
        return;
      }
      if (this.buffer.length < 4 + length) break;
      const payload = this.buffer.subarray(4, 4 + length);
      this.buffer = this.buffer.subarray(4 + length);
      let env: IncomingEnvelope;
      try {
        env = JSON.parse(payload.toString('utf8')) as IncomingEnvelope;
      } catch {
        this.rejectAll(new Error('malformed frame received'));
        return;
      }
      this.routeIncoming(env);
    }
  }

  private routeIncoming(env: IncomingEnvelope): void {
    if (env.method === 'control.handshake.hello' && env.id) {
      const protocol = (env.payload as Record<string, unknown>)?.protocol || 'amitia-game-host/1';
      this.writeFrame({
        type: 'response',
        id: 'hs-' + env.id,
        requestId: env.id,
        payload: { protocol },
        protocol,
      });
      this.completeHandshake();
      return;
    }

    if (env.type === 'notification') {
      this.receivedNotifications.push(env);
      const lookupId = env.id || '';
      const pending = this.pending.get(lookupId);
      if (pending) {
        clearTimeout(pending.timer);
        this.pending.delete(lookupId);
        pending.resolve(env);
      }
      return;
    }

    if (env.type === 'response') {
      this.responseCount++;
      const lookupId = env.requestId || '';
      const pending = this.pending.get(lookupId);
      if (pending) {
        clearTimeout(pending.timer);
        this.pending.delete(lookupId);
        pending.resolve(env);
      }
      return;
    }

    if (env.type === 'error') {
      this.responseCount++;
      const lookupId = env.requestId || '';
      const pending = this.pending.get(lookupId);
      if (pending) {
        clearTimeout(pending.timer);
        this.pending.delete(lookupId);
        pending.resolve(env);
      }
    }
  }

  private rejectAll(err: Error): void {
    for (const [, p] of this.pending) {
      clearTimeout(p.timer);
      p.reject(err);
    }
    this.pending.clear();
  }
}

export function createHarness(pluginPath: string): ExternalE2EHarness {
  return new ExternalE2EHarness(pluginPath);
}
