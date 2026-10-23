import { createHash } from "node:crypto";
import { createServer } from "node:http";
import { promises as fs } from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("electron", () => ({
  session: {
    fromPartition: () => ({ fetch }),
  },
}));

import { installSegmentedFullDownload } from "./segmented-updater";

const servers: ReturnType<typeof createServer>[] = [];

afterEach(() => {
  servers.splice(0).forEach((server) => server.close());
});

describe("segmented updater integration", () => {
  it("replaces only the full download executor and verifies the installer", async () => {
    const content = Buffer.alloc(1024 * 1024 + 73);
    for (let index = 0; index < content.length; index += 1) {
      content[index] = (index * 7 + 19) % 251;
    }
    const server = createServer((request, response) => {
      const match = /^bytes=(\d+)-(\d+)$/.exec(request.headers.range ?? "");
      if (!match) {
        response.writeHead(200, { "Content-Length": content.length });
        response.end(content);
        return;
      }
      const start = Number(match[1]);
      const end = Math.min(Number(match[2]), content.length - 1);
      response.writeHead(206, {
        "Content-Range": `bytes ${start}-${end}/${content.length}`,
        "Content-Length": end - start + 1,
        ETag: '"fixture"',
      });
      response.end(content.subarray(start, end + 1));
    });
    servers.push(server);
    await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
    const address = server.address();
    if (!address || typeof address === "string") throw new Error("server unavailable");
    const directory = await fs.mkdtemp(path.join(os.tmpdir(), "amitia-updater-test-"));
    const destination = path.join(directory, "update.exe");
    const fallback = vi.fn(async () => destination);
    const updater = {
      httpExecutor: { download: fallback },
    } as never;
    installSegmentedFullDownload(updater);
    const executor = (updater as { httpExecutor: { download: Function } }).httpExecutor;
    await executor.download(new URL(`http://127.0.0.1:${address.port}/update.exe`), destination, {
      sha512: createHash("sha512").update(content).digest("base64"),
      headers: { "Accept-Encoding": "identity" },
      cancellationToken: {
        cancelled: false,
        onCancel: () => undefined,
        removeListener: () => undefined,
      },
      onProgress: () => undefined,
    });
    expect(fallback).not.toHaveBeenCalled();
    expect(await fs.readFile(destination)).toEqual(content);
    await fs.rm(directory, { recursive: true, force: true });
  });
});
