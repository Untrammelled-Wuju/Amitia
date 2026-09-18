import { spawn, ChildProcess } from 'child_process';
import { EventEmitter } from 'events';

interface RequestEnvelope {
  type: 'request';
  id: string;
  service: string;
  method: string;
  payload: object;
}

interface NotificationEnvelope {
  type: 'notification';
  service: string;
  name: string;
  payload: object;
}

interface ResponseEnvelope {
  type: 'response';
  id: string;
  ok: boolean;
  payload: object;
  err?: string;
}

interface IncomingEnvelope {
  type: string;
  id?: string;
  requestId?: string;
  method?: string;
  service?: string;
  payload?: object;
  protocol?: string;
}

interface PendingRequest {
  resolve: (value: ResponseEnvelope) => void;
  reject: (reason: Error) => void;
  timer: NodeJS.Timeout;
}

export class PseudoHost {
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
  private outboundQueue: Array<RequestEnvelope | NotificationEnvelope> = [];

  constructor(pluginPath: string) {
    this.pluginPath = pluginPath;
    this.emitter.setMaxListeners(0);
  }

  async start(): Promise<void> {
    const isWindows = process.platform === 'win32';
    const scriptPath = this.pluginPath;
    if (isWindows && scriptPath.endsWith('.js')) {
      this.proc = spawn(process.execPath, [scriptPath], { stdio: ['pipe', 'pipe', 'pipe'] });
    } else {
      this.proc = spawn(scriptPath, [], { stdio: ['pipe', 'pipe', 'pipe'] });
    }
    this.stdin = this.proc.stdin;
    this.stdout = this.proc.stdout;
    this.stderr = this.proc.stderr;
    this.buffer = Buffer.alloc(0);
    this.pending = new Map();
    this.running = true;
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
    service: string,
    method: string,
    payload: object,
    timeoutMs: number = 5000
  ): Promise<ResponseEnvelope> {
    await this.waitForHandshake();
    this.rpcSeq++;
    const id = 'rpc-' + this.rpcSeq;
    const env: RequestEnvelope = { type: 'request', id, service, method, payload };

    return new Promise<ResponseEnvelope>((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new Error('rpc timeout: ' + service + ' ' + method));
      }, timeoutMs);

      this.pending.set(id, { resolve, reject, timer });
      this.writeFrame(env);
    });
  }

  async sendNotification(service: string, name: string, payload: object): Promise<void> {
    await this.waitForHandshake();
    const env: NotificationEnvelope = { type: 'notification', service, name, payload };
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
    const queue = this.outboundQueue;
    this.outboundQueue = [];
    for (const env of queue) {
      this.writeFrame(env);
    }
    for (const waiter of this.handshakeWaiters) {
      waiter();
    }
    this.handshakeWaiters = [];
  }

  kill(): void {
    if (this.proc) {
      this.proc.kill();
    }
  }

  async waitExit(): Promise<number> {
    if (this.exited) {
      return this.exitCode === null ? -1 : this.exitCode;
    }
    return new Promise<number>((resolve) => {
      this.emitter.once('exit', (code: number) => resolve(code));
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

  async restart(): Promise<void> {
    this.kill();
    await this.start();
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
      if (this.buffer.length < 4) {
        break;
      }
      const length = this.buffer.readUInt32BE(0);
      if (this.buffer.length < 4 + length) {
        break;
      }
      const payload = this.buffer.slice(4, 4 + length);
      this.buffer = this.buffer.slice(4 + length);
      const env = JSON.parse(payload.toString('utf8')) as IncomingEnvelope;
      this.routeResponse(env);
    }
  }

  private routeResponse(env: IncomingEnvelope): void {
    if (env.method === 'control.handshake.hello' && env.id) {
      const helloPayload = (env.payload || {}) as Record<string, unknown>;
      const respPayload = { ...helloPayload, protocol: 'amitia-game-host/1' };
      this.writeFrame({ type: 'response', id: env.id, ok: true, payload: respPayload });
      this.completeHandshake();
      return;
    }
    if (env.type !== 'response') {
      return;
    }
    const lookupId = env.requestId || env.id || '';
    const pending = this.pending.get(lookupId);
    if (!pending) {
      return;
    }
    clearTimeout(pending.timer);
    this.pending.delete(lookupId);
    const normalized = this.normalizeResponse(env);
    pending.resolve(normalized);
  }

  private normalizeResponse(env: IncomingEnvelope): ResponseEnvelope {
    const anyEnv = env as any;
    return {
      type: 'response',
      id: env.id || '',
      ok: typeof anyEnv.ok === 'boolean' ? anyEnv.ok : !anyEnv.error,
      payload: env.payload || {},
      err: anyEnv.error ? (anyEnv.error.message || anyEnv.error) : undefined,
    };
  }

  private rejectAll(err: Error): void {
    for (const [, p] of this.pending) {
      clearTimeout(p.timer);
      p.reject(err);
    }
    this.pending.clear();
  }
}

export function MustMarshal(value: object): object {
  return value;
}
