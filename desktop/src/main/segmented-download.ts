import { createHash } from "node:crypto";
import { createReadStream } from "node:fs";
import { promises as fs } from "node:fs";
import path from "node:path";

export interface SegmentedProgress {
  total: number;
  transferred: number;
  percent: number;
  bytesPerSecond: number;
}

export interface SegmentedDownloadOptions {
  url: string;
  destination: string;
  expectedSha512?: string | null;
  headers?: Record<string, string>;
  concurrency?: number;
  chunkSize?: number;
  signal?: AbortSignal;
  fetch: (input: string, init?: RequestInit) => Promise<Response>;
  onProgress?: (progress: SegmentedProgress) => void;
  retries?: number;
}

interface RangeProbe {
  supported: boolean;
  total: number;
  etag: string;
  lastModified: string;
}

interface DownloadState {
  url: string;
  total: number;
  etag: string;
  lastModified: string;
  chunkSize: number;
  chunkCount: number;
  expectedSha512: string;
}

export class RangeUnsupportedError extends Error {
  constructor(message = "server does not support byte ranges") {
    super(message);
    this.name = "RangeUnsupportedError";
  }
}

function headerValue(headers: Headers, name: string) {
  return headers.get(name)?.trim() ?? "";
}

function parseContentRange(value: string) {
  const match = /^bytes\s+\d+-\d+\/(\d+)$/i.exec(value);
  if (!match) return 0;
  const total = Number(match[1]);
  return Number.isSafeInteger(total) && total > 0 ? total : 0;
}

async function probeRange(options: SegmentedDownloadOptions): Promise<RangeProbe> {
  const response = await options.fetch(options.url, {
    method: "GET",
    headers: {
      ...options.headers,
      "Accept-Encoding": "identity",
      Range: "bytes=0-0",
      "Cache-Control": "no-cache",
    },
    signal: options.signal,
  });
  const total = parseContentRange(headerValue(response.headers, "content-range"));
  const supported =
    response.status === 206 &&
    total > 0 &&
    headerValue(response.headers, "content-length") === "1";
  const etag = headerValue(response.headers, "etag");
  const lastModified = headerValue(response.headers, "last-modified");
  await response.body?.cancel().catch(() => undefined);
  if (!supported) {
    throw new RangeUnsupportedError(`range probe returned ${response.status}`);
  }
  return { supported, total, etag, lastModified };
}

function stateKey(options: SegmentedDownloadOptions, probe: RangeProbe) {
  const hash = createHash("sha256");
  hash.update(options.url);
  hash.update("\n");
  hash.update(String(probe.total));
  hash.update("\n");
  hash.update(probe.etag);
  hash.update("\n");
  hash.update(probe.lastModified);
  hash.update("\n");
  hash.update(options.expectedSha512 ?? "");
  return hash.digest("hex").slice(0, 24);
}

async function ensureStateDirectory(
  options: SegmentedDownloadOptions,
  probe: RangeProbe,
  chunkSize: number,
  chunkCount: number,
) {
  const root = path.join(path.dirname(options.destination), ".segmented-downloads");
  const directory = path.join(root, stateKey(options, probe));
  await fs.mkdir(directory, { recursive: true, mode: 0o700 });
  const metadataPath = path.join(directory, "state.json");
  const state: DownloadState = {
    url: options.url,
    total: probe.total,
    etag: probe.etag,
    lastModified: probe.lastModified,
    chunkSize,
    chunkCount,
    expectedSha512: options.expectedSha512 ?? "",
  };
  let existing: DownloadState | null = null;
  try {
    existing = JSON.parse(await fs.readFile(metadataPath, "utf8")) as DownloadState;
  } catch {
    existing = null;
  }
  if (
    !existing ||
    existing.url !== state.url ||
    existing.total !== state.total ||
    existing.etag !== state.etag ||
    existing.lastModified !== state.lastModified ||
    existing.chunkSize !== state.chunkSize ||
    existing.chunkCount !== state.chunkCount ||
    existing.expectedSha512 !== state.expectedSha512
  ) {
    for (const entry of await fs.readdir(directory).catch(() => [])) {
      if (entry.endsWith(".part") || entry.endsWith(".partial")) {
        await fs.rm(path.join(directory, entry), { force: true });
      }
    }
    await fs.writeFile(metadataPath, JSON.stringify(state, null, 2), { mode: 0o600 });
  }
  return directory;
}

async function ensureDiskSpace(destination: string, total: number) {
  try {
    const stats = await fs.statfs(path.dirname(destination));
    const available = Number(stats.bavail) * Number(stats.bsize);
    if (Number.isFinite(available) && available < total * 2 + 32 * 1024 * 1024) {
      throw new Error("更新下载可用磁盘空间不足");
    }
  } catch (error) {
    if (error instanceof Error && error.message.includes("磁盘空间不足")) throw error;
  }
}

function partSize(total: number, chunkSize: number, index: number) {
  return Math.min(chunkSize, total - index * chunkSize);
}

async function streamPart(
  response: Response,
  target: string,
  expectedSize: number,
  onBytes: (bytes: number) => void,
  signal?: AbortSignal,
) {
  if (!response.body) throw new Error("更新分片响应为空");
  const partial = `${target}.partial`;
  const handle = await fs.open(partial, "w", 0o600);
  let written = 0;
  try {
    const reader = response.body.getReader();
    while (true) {
      if (signal?.aborted) throw new Error("cancelled");
      const { done, value } = await reader.read();
      if (done) break;
      if (!value?.byteLength) continue;
      written += value.byteLength;
      if (written > expectedSize) throw new Error("更新分片超过预期大小");
      await handle.write(value);
      onBytes(written);
    }
    if (written !== expectedSize) {
      throw new Error(`更新分片大小不匹配: ${written}/${expectedSize}`);
    }
    await handle.sync();
  } catch (error) {
    await handle.close().catch(() => undefined);
    await fs.rm(partial, { force: true });
    throw error;
  }
  await handle.close();
  await fs.rename(partial, target);
}

async function wait(ms: number) {
  await new Promise((resolve) => setTimeout(resolve, ms));
}

async function downloadPart(
  options: SegmentedDownloadOptions,
  stateDirectory: string,
  probe: RangeProbe,
  chunkSize: number,
  index: number,
  updateBytes: (bytes: number) => void,
) {
  const target = path.join(stateDirectory, `${String(index).padStart(8, "0")}.part`);
  const expected = partSize(probe.total, chunkSize, index);
  try {
    const existing = await fs.stat(target);
    if (existing.size === expected) {
      updateBytes(expected);
      return;
    }
    await fs.rm(target, { force: true });
  } catch {
    updateBytes(0);
  }
  const retries = Math.max(0, options.retries ?? 3);
  let lastError: unknown;
  for (let attempt = 0; attempt <= retries; attempt += 1) {
    if (options.signal?.aborted) throw new Error("cancelled");
    const start = index * chunkSize;
    const end = start + expected - 1;
    try {
      const response = await options.fetch(options.url, {
        method: "GET",
        headers: {
          ...options.headers,
          "Accept-Encoding": "identity",
          Range: `bytes=${start}-${end}`,
          "Cache-Control": "no-cache",
        },
        signal: options.signal,
      });
      if (response.status !== 206) {
        await response.body?.cancel().catch(() => undefined);
        throw new RangeUnsupportedError(`range request returned ${response.status}`);
      }
      const contentRange = headerValue(response.headers, "content-range");
      if (!contentRange.endsWith(`/${probe.total}`)) {
        await response.body?.cancel().catch(() => undefined);
        throw new Error("更新服务器返回了不一致的文件范围");
      }
      await streamPart(response, target, expected, updateBytes, options.signal);
      return;
    } catch (error) {
      lastError = error;
      updateBytes(0);
      if (error instanceof RangeUnsupportedError) throw error;
      if (options.signal?.aborted) throw new Error("cancelled");
      if (attempt < retries) await wait(400 * 2 ** attempt);
    }
  }
  throw lastError;
}

async function mergeParts(
  options: SegmentedDownloadOptions,
  stateDirectory: string,
  total: number,
  chunkSize: number,
  chunkCount: number,
) {
  const output = await fs.open(options.destination, "w", 0o600);
  const hash = createHash("sha512");
  let written = 0;
  try {
    for (let index = 0; index < chunkCount; index += 1) {
      const partPath = path.join(stateDirectory, `${String(index).padStart(8, "0")}.part`);
      for await (const chunk of createReadStream(partPath)) {
        const buffer = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
        hash.update(buffer);
        await output.write(buffer);
        written += buffer.length;
      }
    }
    if (written !== total) throw new Error("更新文件合并大小不匹配");
    await output.sync();
  } catch (error) {
    await output.close().catch(() => undefined);
    await fs.rm(options.destination, { force: true });
    throw error;
  }
  await output.close();
  const actual = hash.digest("base64");
  if (options.expectedSha512 && actual !== options.expectedSha512) {
    await fs.rm(options.destination, { force: true });
    throw new Error("更新文件 SHA-512 校验失败");
  }
  return actual;
}

export async function downloadSegmented(options: SegmentedDownloadOptions) {
  const probe = await probeRange(options);
  const chunkSize = Math.max(1024 * 1024, options.chunkSize ?? 8 * 1024 * 1024);
  const chunkCount = Math.ceil(probe.total / chunkSize);
  const concurrency = Math.min(
    8,
    Math.max(1, options.concurrency ?? 8),
    chunkCount,
  );
  await ensureDiskSpace(options.destination, probe.total);
  const stateDirectory = await ensureStateDirectory(options, probe, chunkSize, chunkCount);
  const partBytes = new Array<number>(chunkCount).fill(0);
  let networkBytes = 0;
  let lastProgressAt = 0;
  const startedAt = Date.now();
  const emitProgress = (force = false) => {
    if (!options.onProgress) return;
    const now = Date.now();
    if (!force && now - lastProgressAt < 200) return;
    lastProgressAt = now;
    const transferred = partBytes.reduce((total, value) => total + value, 0);
    const elapsedSeconds = Math.max((now - startedAt) / 1000, 0.001);
    options.onProgress({
      total: probe.total,
      transferred,
      percent: Math.min(100, (transferred / probe.total) * 100),
      bytesPerSecond: Math.round(networkBytes / elapsedSeconds),
    });
  };
  emitProgress(true);
  let nextIndex = 0;
  const worker = async () => {
    while (true) {
      const index = nextIndex;
      nextIndex += 1;
      if (index >= chunkCount) return;
      await downloadPart(
        options,
        stateDirectory,
        probe,
        chunkSize,
        index,
        (bytes) => {
          if (bytes > partBytes[index]) networkBytes += bytes - partBytes[index];
          partBytes[index] = bytes;
          emitProgress();
        },
      );
    }
  };
  await Promise.all(Array.from({ length: concurrency }, () => worker()));
  emitProgress(true);
  let actualSha512: string;
  try {
    actualSha512 = await mergeParts(options, stateDirectory, probe.total, chunkSize, chunkCount);
  } catch (error) {
    await fs.rm(stateDirectory, { recursive: true, force: true });
    throw error;
  }
  await fs.rm(stateDirectory, { recursive: true, force: true });
  options.onProgress?.({
    total: probe.total,
    transferred: probe.total,
    percent: 100,
    bytesPerSecond: Math.round(networkBytes / Math.max((Date.now() - startedAt) / 1000, 0.001)),
  });
  return { bytes: probe.total, sha512: actualSha512 };
}
