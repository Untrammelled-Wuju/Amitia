import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import { promises as fs } from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it } from "vitest";

import {
  downloadSegmented,
  RangeUnsupportedError,
} from "./segmented-download";

const servers: ReturnType<typeof createServer>[] = [];
const directories: string[] = [];

afterEach(async () => {
  await Promise.all(
    servers.splice(0).map(
      (server) =>
        new Promise<void>((resolve) => server.close(() => resolve())),
    ),
  );
  await Promise.all(
    directories.splice(0).map((directory) =>
      fs.rm(directory, { recursive: true, force: true }),
    ),
  );
});

function parseRange(value: string | undefined, total: number) {
  if (!value) return null;
  const match = /^bytes=(\d+)-(\d+)$/.exec(value);
  if (!match) return null;
  const start = Number(match[1]);
  const end = Math.min(Number(match[2]), total - 1);
  return { start, end };
}

async function rangeServer(content: Buffer, ranges: boolean) {
  const state = { active: 0, maximum: 0 };
  const server = createServer(
    (request: IncomingMessage, response: ServerResponse) => {
      const range = parseRange(request.headers.range, content.length);
      if (!ranges) {
        response.writeHead(200, {
          "Content-Length": content.length,
          "Content-Type": "application/octet-stream",
        });
        response.end(content);
        return;
      }
      if (!range) {
        response.writeHead(200, {
          "Accept-Ranges": "bytes",
          "Content-Length": content.length,
        });
        response.end(content);
        return;
      }
      state.active += 1;
      state.maximum = Math.max(state.maximum, state.active);
      setTimeout(() => {
        response.writeHead(206, {
          "Accept-Ranges": "bytes",
          "Content-Range": `bytes ${range.start}-${range.end}/${content.length}`,
          "Content-Length": range.end - range.start + 1,
          ETag: '"fixture-v1"',
          "Last-Modified": "Thu, 23 Oct 2026 00:00:00 GMT",
        });
        response.end(content.subarray(range.start, range.end + 1), () => {
          state.active -= 1;
        });
      }, 15);
    },
  );
  servers.push(server);
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  if (!address || typeof address === "string") throw new Error("server address unavailable");
  return { url: `http://127.0.0.1:${address.port}/update.exe`, state };
}

describe("segmented update download", () => {
  it("downloads with concurrent byte ranges and preserves the payload", async () => {
    const content = Buffer.alloc(2 * 1024 * 1024 + 193);
    for (let index = 0; index < content.length; index += 1) {
      content[index] = (index * 13 + 7) % 251;
    }
    const fixture = await rangeServer(content, true);
    const directory = await fs.mkdtemp(path.join(os.tmpdir(), "amitia-segment-test-"));
    directories.push(directory);
    const destination = path.join(directory, "update.exe");
    const progress: number[] = [];
    const result = await downloadSegmented({
      url: fixture.url,
      destination,
      chunkSize: 256 * 1024,
      concurrency: 4,
      fetch,
      onProgress: (value) => progress.push(value.percent),
    });
    expect(result.bytes).toBe(content.length);
    expect(await fs.readFile(destination)).toEqual(content);
    expect(fixture.state.maximum).toBeGreaterThan(1);
    expect(progress.at(-1)).toBe(100);
  });

  it("reports unsupported ranges for the updater fallback", async () => {
    const fixture = await rangeServer(Buffer.from("single-thread"), false);
    const directory = await fs.mkdtemp(path.join(os.tmpdir(), "amitia-segment-test-"));
    directories.push(directory);
    await expect(
      downloadSegmented({
        url: fixture.url,
        destination: path.join(directory, "update.exe"),
        fetch,
      }),
    ).rejects.toBeInstanceOf(RangeUnsupportedError);
  });
});
