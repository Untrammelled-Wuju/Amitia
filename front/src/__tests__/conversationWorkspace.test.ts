import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  del: vi.fn(),
  getDeploymentConfig: vi.fn(),
}));

vi.mock("@/composables/useApi", () => ({
  useApi: () => ({
    get: mocks.get,
    post: mocks.post,
    put: mocks.put,
    del: mocks.del,
  }),
}));

vi.mock("@/runtime/runtime-adapter", () => ({
  getDeploymentConfig: mocks.getDeploymentConfig,
}));

import { useConversationWorkspace } from "@/composables/useConversationWorkspace";

describe("conversation workspace routes", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.get.mockResolvedValue([]);
    mocks.post.mockResolvedValue({});
    mocks.getDeploymentConfig.mockResolvedValue({ mode: "local" });
    window.amitiaDesktop = {
      selectWorkspaceDirectory: vi.fn().mockResolvedValue({
        path: "D:\\workspace",
        name: "workspace",
      }),
      getMeshIdentity: vi.fn().mockResolvedValue({ deviceId: "device-1" }),
    } as any;
  });

  it("uses the registered workspace API namespace", async () => {
    const workspace = useConversationWorkspace();

    await workspace.refreshRecentWorkspaces();
    expect(mocks.get).toHaveBeenCalledWith("/api/workspaces");

    await workspace.selectWorkspaceMount({
      id: "workspace-1",
      name: "workspace",
      kind: "local",
      rootUri: "amitia://workspace/@workspace-1/",
      readOnly: false,
      available: true,
      status: "ready",
    });
    expect(mocks.post).toHaveBeenCalledWith("/api/workspaces/workspace-1/touch");

    mocks.post.mockResolvedValueOnce({
      id: "workspace-2",
      name: "workspace",
      kind: "local",
      rootUri: "amitia://workspace/@workspace-2/",
      readOnly: false,
      available: true,
      status: "ready",
    });
    await workspace.chooseWorkspaceDirectory();
    expect(mocks.post).toHaveBeenCalledWith("/api/workspaces/local", {
      name: "workspace",
      localRoot: "D:\\workspace",
      readOnly: false,
    });
  });
});
