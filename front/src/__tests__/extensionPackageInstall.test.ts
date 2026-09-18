import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  post: vi.fn(),
}));

vi.mock("@/composables/useApi", () => ({
  apiClient: {
    post: mocks.post,
  },
}));

import {
  installExtensionPackage,
  setGameCenterExtensionEnabled,
} from "@/views/extensions/api";

describe("extension package install", () => {
  beforeEach(() => {
    mocks.post.mockReset();
    vi.spyOn(globalThis.crypto, "randomUUID").mockReturnValue(
      "11111111-2222-4333-8444-555555555555",
    );
  });

  it("sends one idempotency key in the install body and allowed header", async () => {
    mocks.post
      .mockResolvedValueOnce({ data: { confirmationToken: "confirmation-token" } })
      .mockResolvedValueOnce({ data: { operationId: "operation-1" } });

    await installExtensionPackage(
      {
        sessionId: "preview-1",
        scopeType: "global",
        scopeId: "",
        capabilityConfirmations: [],
      } as any,
      {
        unsigned: true,
        scripts: true,
        capabilities: [],
        versionChange: false,
        signerChange: false,
        configMigration: false,
      },
      "",
      "game-center",
    );

    const operationRequest = mocks.post.mock.calls[1];
    const expectedKey =
      "package:install:preview-1:11111111-2222-4333-8444-555555555555";
    expect(operationRequest[0]).toBe("/api/extensions/packages/operations/install");
    expect(operationRequest[1].idempotencyKey).toBe(expectedKey);
    expect(operationRequest[2].headers).toEqual({
      "X-Amitia-Management-Target": "game-center",
      "Idempotency-Key": expectedKey,
    });
    expect(operationRequest[2].timeout).toBe(300000);
  });

  it("uses update when the preview reports an installed version", async () => {
    mocks.post
      .mockResolvedValueOnce({ data: { confirmationToken: "confirmation-token" } })
      .mockResolvedValueOnce({ data: { operationId: "operation-2" } });

    await installExtensionPackage(
      {
        sessionId: "preview-2",
        id: "com.amitiax/minecraft",
        currentVersion: "0.7.0-dev.1",
        scopeType: "global",
        scopeId: "",
        capabilityConfirmations: ["confirm.update"],
      } as any,
      {
        unsigned: true,
        scripts: true,
        capabilities: [],
        versionChange: true,
        signerChange: true,
        configMigration: true,
      },
      "",
      "game-center",
    );

    const operationRequest = mocks.post.mock.calls[1];
    expect(operationRequest[0]).toBe("/api/extensions/packages/operations/update");
    expect(operationRequest[1].expectedExtensionId).toBe("com.amitiax/minecraft");
    expect(operationRequest[1].idempotencyKey).toContain("package:update:preview-2:");
  });

  it("enables an installed game center extension", async () => {
    mocks.post.mockResolvedValueOnce({ data: { state: "enabled" } });

    await setGameCenterExtensionEnabled("com.amitiax/minecraft", true);

    expect(mocks.post).toHaveBeenCalledWith(
      "/api/game-center/extensions/enable",
      { extensionId: "com.amitiax/minecraft" },
    );
  });
});
