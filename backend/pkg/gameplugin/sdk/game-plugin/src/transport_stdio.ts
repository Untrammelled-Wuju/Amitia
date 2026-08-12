import { Envelope } from './protocol';

export const STDIO_MAX_FRAME_SIZE = 16 * 1024 * 1024;
export const STDIO_FRAME_HEADER_SIZE = 4;

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
