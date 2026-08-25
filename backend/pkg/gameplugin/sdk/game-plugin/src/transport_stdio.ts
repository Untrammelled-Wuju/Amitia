import { Envelope } from './protocol';

export const STDIO_MAX_FRAME_SIZE = 16 * 1024 * 1024;
export const STDIO_FRAME_HEADER_SIZE = 4;
export const STDIO_BINARY_FRAME_FLAG = 0x80000000;
export const STDIO_FRAME_LENGTH_MASK = 0x7fffffff;

export interface StdioTransportOptions {
  stdin?: NodeJS.ReadableStream;
  stdout?: NodeJS.WritableStream;
  maxFrameSize?: number;
}

export class StdioTransport {
  private stdin: NodeJS.ReadableStream;
  private stdout: NodeJS.WritableStream;
  private maxFrameSize: number;
  private buffer: Buffer | null = null;
  private resolveReceive: ((data: Buffer) => void) | null = null;

  constructor(options: StdioTransportOptions = {}) {
    this.stdin = options.stdin ?? process.stdin;
    this.stdout = options.stdout ?? process.stdout;
    this.maxFrameSize = options.maxFrameSize ?? STDIO_MAX_FRAME_SIZE;
    this.setupReadHandler();
  }

  private setupReadHandler(): void {
    this.stdin.on('data', (chunk: Buffer) => {
      if (this.resolveReceive) {
        this.resolveReceive(chunk);
        this.resolveReceive = null;
      } else {
        this.buffer = this.buffer
          ? Buffer.concat([this.buffer, chunk])
          : chunk;
      }
    });
  }

  private async readFromBuffer(size: number): Promise<Buffer | null> {
    if (this.buffer && this.buffer.length >= size) {
      const result = this.buffer.subarray(0, size);
      this.buffer = this.buffer.subarray(size);
      if (this.buffer.length === 0) {
        this.buffer = null;
      }
      return result;
    }

    return new Promise<Buffer | null>((resolve) => {
      this.resolveReceive = (chunk: Buffer) => {
        const combined = this.buffer
          ? Buffer.concat([this.buffer, chunk])
          : chunk;
        if (combined.length >= size) {
          const result = combined.subarray(0, size);
          this.buffer = combined.subarray(size);
          if (this.buffer.length === 0) {
            this.buffer = null;
          }
          resolve(result);
        } else {
          this.buffer = combined;
          resolve(null);
        }
      };
    });
  }

  async send(message: Envelope): Promise<void> {
    const data = JSON.stringify(message);
    const payload = Buffer.from(data, 'utf-8');

    if (payload.length > this.maxFrameSize) {
      throw new Error(`frame size ${payload.length} exceeds limit ${this.maxFrameSize}`);
    }

    const header = Buffer.alloc(STDIO_FRAME_HEADER_SIZE);
    header.writeUInt32BE(payload.length, 0);

    return new Promise<void>((resolve, reject) => {
      this.stdout.write(Buffer.concat([header, payload]), (err) => {
        if (err) reject(err);
        else resolve();
      });
    });
  }


  async sendBinaryFrame(message: Envelope, objectId: string, offset: number, data: Uint8Array): Promise<void> {
    if (!message.id) throw new Error('binary frame requires message id');
    if (!objectId || !objectId.startsWith('bin_')) throw new Error('binary frame requires valid object id');
    if (!Number.isSafeInteger(offset) || offset < 0) throw new Error('binary frame offset must be a non-negative safe integer');
    if (!(data instanceof Uint8Array) || data.byteLength === 0) throw new Error('binary frame payload must not be empty');

    const frameHeader = {
      protocol: message.protocol,
      id: message.id,
      runtimeId: message.runtimeId,
      pluginId: message.pluginId,
      serviceId: message.serviceId,
      generation: message.generation,
      objectId,
      offset,
    };
    const headerJson = Buffer.from(JSON.stringify(frameHeader), 'utf-8');
    const payload = Buffer.from(data);
    const bodyLength = 4 + headerJson.length + payload.length;
    if (bodyLength <= 4 || bodyLength > this.maxFrameSize || bodyLength > STDIO_FRAME_LENGTH_MASK) {
      throw new Error(`binary frame size ${bodyLength} exceeds limit ${this.maxFrameSize}`);
    }

    const outer = Buffer.alloc(STDIO_FRAME_HEADER_SIZE);
    outer.writeUInt32BE((STDIO_BINARY_FRAME_FLAG | bodyLength) >>> 0, 0);
    const meta = Buffer.alloc(4);
    meta.writeUInt32BE(headerJson.length, 0);
    const frame = Buffer.concat([outer, meta, headerJson, payload]);

    return new Promise<void>((resolve, reject) => {
      this.stdout.write(frame, (err) => {
        if (err) reject(err);
        else resolve();
      });
    });
  }

  async receive(): Promise<Envelope> {
    const headerBuf = await this.readExact(STDIO_FRAME_HEADER_SIZE);
    const frameLen = headerBuf.readUInt32BE(0);

    if (frameLen === 0) {
      throw new Error('invalid frame: zero length');
    }
    if (frameLen > this.maxFrameSize) {
      throw new Error(`frame size ${frameLen} exceeds limit ${this.maxFrameSize}`);
    }

    const data = await this.readExact(frameLen);
    const jsonStr = data.toString('utf-8');
    return JSON.parse(jsonStr) as Envelope;
  }

  private async readExact(size: number): Promise<Buffer> {
    while (true) {
      const result = await this.readFromBuffer(size);
      if (result) return result;
      await new Promise((resolve) => setImmediate(resolve));
    }
  }

  async close(): Promise<void> {
    return Promise.resolve();
  }
}
