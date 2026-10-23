import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  managed: vi.fn(),
  get: vi.fn(),
  post: vi.fn(),
  delete: vi.fn(),
}));

vi.mock("@/composables/useApi", () => ({
  apiClient: {
    get: mocks.get,
    post: mocks.post,
    delete: mocks.delete,
  },
}));

vi.mock("@/runtime/owned-chat-management", () => ({
  ownedManagementRequest: mocks.managed,
}));

import { createConversationFromImport } from "@/views/chat-import/api";
import { isDeviceLocalApiPath } from "@/runtime/runtime-adapter";

describe("chat import promotion", () => {
  beforeEach(() => {
    mocks.managed.mockReset();
  });

  it("promotes the persisted import batch through the owned chat authority", async () => {
    mocks.managed.mockResolvedValue({
      conversationId: "import-owned-conversation",
      imported: true,
      executionScope: {
        resourceOwnerId: "device",
        roleId: "role",
      },
    });

    const result = await createConversationFromImport("imp-batch");

    expect(mocks.managed).toHaveBeenCalledWith(
      "from-import",
      { importBatchId: "imp-batch" },
      "post",
      "",
    );
    expect(result.ownerConversationId).toBe("import-owned-conversation");
    expect(result.conversationId).toBe(
      "meshconv1:device:import-owned-conversation",
    );
  });

  it("routes import staging to the retained device runtime", () => {
    expect(isDeviceLocalApiPath("/api/imports/confirm")).toBe(true);
    expect(
      isDeviceLocalApiPath("/api/device-mesh/v1/business/chat-management/from-import"),
    ).toBe(false);
  });
});
