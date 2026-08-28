import { afterEach, describe, expect, it, vi } from "vitest";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createHash } from "node:crypto";

const protocolMock = vi.hoisted(() => ({
  registeredSchemes: [] as unknown[],
  handler: null as ((request: Request) => Promise<Response>) | null,
  registerSchemesAsPrivileged: vi.fn((schemes: unknown[]) => {
    protocolMock.registeredSchemes = schemes;
  }),
  handle: vi.fn((_scheme: string, handler: (request: Request) => Promise<Response>) => {
    protocolMock.handler = handler;
  }),
}));

vi.mock("electron", () => ({
  protocol: {
    registerSchemesAsPrivileged: protocolMock.registerSchemesAsPrivileged,
    handle: protocolMock.handle,
  },
}));

import {
  buildResourceIndex,
  registerPetProtocol,
} from "../resource-protocol";

const tempDirs: string[] = [];

afterEach(async () => {
  for (const dir of tempDirs.splice(0)) {
    await rm(dir, { recursive: true, force: true });
  }
});

describe("amitia-pet resource protocol", () => {
  it("registers the production scheme with CORS enabled", () => {
    expect(protocolMock.registeredSchemes).toEqual([
      expect.objectContaining({
        scheme: "amitia-pet",
        privileges: expect.objectContaining({
          standard: true,
          secure: true,
          supportFetchAPI: true,
          corsEnabled: true,
        }),
      }),
    ]);
  });

  it("returns CORS headers for declared resources", async () => {
    const installPath = await mkdtemp(join(tmpdir(), "amitia-pet-protocol-"));
    tempDirs.push(installPath);
    const relativePath = "actions/idle/action.json";
    const content = Buffer.from('{"ok":true}', "utf8");
    await import("node:fs/promises").then(({ mkdir }) =>
      mkdir(join(installPath, "actions/idle"), { recursive: true }),
    );
    await writeFile(join(installPath, relativePath), content);
    const sha256 = createHash("sha256").update(content).digest("hex");

    const manifest = {
      integrity: {
        files: [
          {
            path: relativePath,
            sha256,
            bytes: content.byteLength,
            mediaType: "application/json",
          },
        ],
      },
    } as never;

    registerPetProtocol(() => ({
      installationId: "install-1",
      installPath,
      manifest,
      resourceIndex: buildResourceIndex(manifest),
    }));

    expect(protocolMock.handler).not.toBeNull();
    const response = await protocolMock.handler!(
      new Request("amitia-pet://installation/install-1/actions/idle/action.json"),
    );

    expect(response.status).toBe(200);
    expect(response.headers.get("access-control-allow-origin")).toBe("*");
    expect(response.headers.get("cross-origin-resource-policy")).toBe("cross-origin");
  });
});
