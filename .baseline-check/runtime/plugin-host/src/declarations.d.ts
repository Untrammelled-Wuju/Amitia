declare var process: {
  stdin: { on(event: string, listener: (...args: any[]) => void): void; isTTY?: boolean; readable: boolean; };
  stdout: { write(data: string | Buffer): boolean; isTTY?: boolean; writable: boolean; };
  stderr: { write(data: string | Buffer): boolean; isTTY?: boolean; writable: boolean; };
  on(event: "SIGTERM", listener: () => void): void;
  on(event: "SIGINT", listener: () => void): void;
  on(event: "uncaughtException", listener: (err: Error) => void): void;
  on(event: "unhandledRejection", listener: (reason: any) => void): void;
  on(event: string, listener: (...args: any[]) => void): void;
  exit(code?: number): void;
  pid: number;
  platform: string;
  argv: string[];
  env: Record<string, string | undefined>;
};

declare var module: any;

declare var Buffer: {
  from(data: any, encoding?: string): Buffer;
  isBuffer(obj: any): obj is Buffer;
  alloc(size: number, fill?: any, encoding?: string): Buffer;
};

declare interface Buffer {
  length: number;
  toString(encoding?: string, start?: number, end?: number): string;
}

declare module "crypto" {
  export function randomBytes(size: number): Buffer;
  export function randomBytes(size: number, callback: (err: Error | null, buf: Buffer) => void): void;
}

declare module "path" {
  export function resolve(...paths: string[]): string;
  export function dirname(p: string): string;
  export function basename(p: string, ext?: string): string;
  export function extname(p: string): string;
  export function isAbsolute(p: string): boolean;
  export function relative(from: string, to: string): string;
  export function join(...paths: string[]): string;
  export function normalize(p: string): string;
  export const sep: string;
  export const delimiter: string;
}

declare module "fs" {
  export function existsSync(path: string): boolean;
  export function readFileSync(path: string, options: { encoding: string; flag?: string }): string;
  export function readFileSync(path: string, encoding: string): string;
  export function readFileSync(path: string): Buffer;
}

declare module "readline" {
  interface Interface {
    on(event: "line", listener: (line: string) => void): Interface;
    on(event: "close", listener: () => void): Interface;
    on(event: "pause", listener: () => void): Interface;
    on(event: "resume", listener: () => void): Interface;
    on(event: string, listener: (...args: any[]) => void): Interface;
    close(): void;
    pause(): void;
    resume(): void;
  }
  interface ReadLineOptions {
    input: any;
    output?: any;
    terminal?: boolean;
    historySize?: number;
    crlfDelay?: number;
  }
  export function createInterface(options: ReadLineOptions): Interface;
}

declare module "module" {
  const Module: any;
  export = Module;
}
