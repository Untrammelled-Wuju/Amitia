import { createHash } from "node:crypto";
import { createReadStream, promises as fs } from "node:fs";
import { createServer } from "node:http";
import os from "node:os";
import path from "node:path";
import { describe, expect, it } from "vitest";

import { downloadSegmented } from "./segmented-download";

const artifactPath = process.env.AMITIA_DESKTOP_UPDATE_ARTIFACT?.trim() ?? "";
const productionUrl = process.env.AMITIA_DESKTOP_UPDATE_URL?.trim() ?? "";
const productionSha512 = process.env.AMITIA_DESKTOP_UPDATE_SHA512?.trim() ?? "";
const productionSize = Number(process.env.AMITIA_DESKTOP_UPDATE_SIZE ?? 0);
const productionRounds = Number(process.env.AMITIA_PRODUCTION_TEST_ROUNDS ?? 10);
const remoteFullRounds = Number(process.env.AMITIA_DESKTOP_REMOTE_FULL_ROUNDS ?? 0);

async function sha512File(file: string) {
  const hash = createHash("sha512");
  for await (const chunk of createReadStream(file)) hash.update(chunk);
  return hash.digest("base64");
}

async function sha256File(file: string) {
  const hash = createHash("sha256");
  for await (const chunk of createReadStream(file)) hash.update(chunk);
  return hash.digest("hex");
}

function parseRange(value: string | undefined, total: number) {
  const match = /^bytes=(\d+)-(\d+)$/.exec(value ?? "");
  if (!match) return null;
  return {
    start: Number(match[1]),
    end: Math.min(Number(match[2]), total - 1),
  };
}

async function startRangeServer(file: string, size: number) {
  const server = createServer((request, response) => {
    const range = parseRange(request.headers.range, size);
    if (!range) {
      response.writeHead(200, {
        "Accept-Ranges": "bytes",
        "Content-Length": size,
        "Content-Type": "application/octet-stream",
      });
      createReadStream(file).pipe(response);
      return;
    }
    const length = range.end - range.start + 1;
    response.writeHead(206, {
      "Accept-Ranges": "bytes",
      "Content-Range": `bytes ${range.start}-${range.end}/${size}`,
      "Content-Length": length,
      "Content-Type": "application/octet-stream",
      ETag: `"sha512-${path.basename(file)}"`,
      "Last-Modified": new Date(2026, 9, 23).toUTCString(),
    });
    createReadStream(file, { start: range.start, end: range.end }).pipe(response);
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  if (!address || typeof address === "string") throw new Error("range server unavailable");
  return {
    url: `http://127.0.0.1:${address.port}/artifact.exe`,
    close: () => new Promise<void>((resolve) => server.close(() => resolve())),
  };
}

async function verifyProductionRanges(file: string, size: number) {
  if (!productionUrl) return;
  const sampleSize = 1024 * 1024;
  const starts = [0, Math.floor(size / 2), size - sampleSize];
  const handle = await fs.open(file, "r");
  try {
    for (const start of starts) {
      const end = start + sampleSize - 1;
      const response = await fetch(productionUrl, {
        headers: { Range: `bytes=${start}-${end}`, "Accept-Encoding": "identity" },
      });
      expect(response.status).toBe(206);
      const remote = Buffer.from(await response.arrayBuffer());
      const local = Buffer.alloc(sampleSize);
      await handle.read(local, 0, sampleSize, start);
      expect(remote).toEqual(local);
    }
  } finally {
    await handle.close();
  }
}

describe.skipIf(!artifactPath || !productionSha512 || productionSize <= 0)(
  "production artifact segmented download",
  () => {
    it(
      `downloads and verifies ${productionRounds} consecutive production-shaped rounds`,
      async () => {
        const artifact = await fs.realpath(artifactPath);
        const stats = await fs.stat(artifact);
        expect(stats.size).toBe(productionSize);
        expect(await sha512File(artifact)).toBe(productionSha512);
        await verifyProductionRanges(artifact, productionSize);
        const rangeServer = await startRangeServer(artifact, productionSize);
        const directory = await fs.mkdtemp(path.join(os.tmpdir(), "amitia-production-download-"));
        try {
          for (let round = 1; round <= remoteFullRounds && productionUrl; round += 1) {
            const destination = path.join(directory, `remote-installer-${round}.exe`);
            const startedAt = Date.now();
            const result = await downloadSegmented({
              url: productionUrl,
              destination,
              expectedSha512: productionSha512,
              concurrency: 8,
              fetch,
            });
            expect((await fs.stat(destination)).size).toBe(productionSize);
            expect(result.sha512).toBe(productionSha512);
            process.stdout.write(
              `[desktop-production-remote] round ${round}/${remoteFullRounds} pass ${Date.now() - startedAt}ms\n`,
            );
            await fs.rm(destination, { force: true });
          }
          for (let round = 1; round <= productionRounds; round += 1) {
            const destination = path.join(directory, `installer-${round}.exe`);
            const startedAt = Date.now();
            const result = await downloadSegmented({
              url: rangeServer.url,
              destination,
              expectedSha512: productionSha512,
              concurrency: 8,
              fetch,
            });
            const downloaded = await fs.stat(destination);
            expect(downloaded.size).toBe(productionSize);
            expect(result.bytes).toBe(productionSize);
            expect(result.sha512).toBe(productionSha512);
            expect(await sha256File(destination)).toMatch(/^[a-f0-9]{64}$/);
            process.stdout.write(
              `[desktop-production] round ${round}/${productionRounds} pass ${Date.now() - startedAt}ms\n`,
            );
            await fs.rm(destination, { force: true });
          }
        } finally {
          await rangeServer.close();
          await fs.rm(directory, { recursive: true, force: true });
        }
      },
      30 * 60 * 1000,
    );
  },
);
