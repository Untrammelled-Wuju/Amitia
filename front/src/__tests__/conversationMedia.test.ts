import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  getApiBaseURLForPath: vi.fn(),
  getBackendAuthHeaders: vi.fn(),
}));

vi.mock("@/composables/useApi", () => ({
  apiClient: { get: mocks.get },
}));

vi.mock("@/runtime/runtime-adapter", () => ({
  getApiBaseURLForPath: mocks.getApiBaseURLForPath,
  getBackendAuthHeaders: mocks.getBackendAuthHeaders,
}));

import {
  resolveConversationDownloadUrl,
  resolveConversationMediaUrl,
} from "@/conversation/rendering/media";

describe("conversation media URL resolution", () => {
  beforeEach(() => {
    mocks.get.mockReset();
    mocks.getApiBaseURLForPath.mockReset();
    mocks.getBackendAuthHeaders.mockReset();
    mocks.getApiBaseURLForPath.mockResolvedValue("http://127.0.0.1:18899");
    mocks.getBackendAuthHeaders.mockResolvedValue({});
  });

  it("mints a short-lived media URL for artifact references", async () => {
    mocks.get.mockResolvedValue({
      data: {
        url: "/media/artifacts/art_ticket/ticket-value",
        expiresAt: new Date(Date.now() + 60_000).toISOString(),
      },
    });
    const resolved = await resolveConversationMediaUrl(
      "amitia://artifacts/art_ticket",
    );
    expect(resolved).toBe(
      "http://127.0.0.1:18899/media/artifacts/art_ticket/ticket-value",
    );
    expect(mocks.get).toHaveBeenCalledWith(
      "/api/artifacts/v1/art_ticket/media-ticket",
    );
  });

  it("adds the forced download query to media URLs", async () => {
    mocks.get.mockResolvedValue({
      data: {
        url: "/media/artifacts/art_download/ticket-value",
        expiresAt: new Date(Date.now() + 60_000).toISOString(),
      },
    });
    const resolved = await resolveConversationDownloadUrl(
      "amitia://artifacts/art_download",
    );
    expect(resolved).toBe(
      "http://127.0.0.1:18899/media/artifacts/art_download/ticket-value?download=1",
    );
  });
});
