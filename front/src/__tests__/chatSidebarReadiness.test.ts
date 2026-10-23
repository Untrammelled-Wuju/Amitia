import { beforeEach, describe, expect, it, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
const owned = vi.hoisted(() => ({ enabled: { value: false }, notice: { value: "" }, policy: { value: null }, coreId: { value: "core", __v_isRef: true }, roleOwnerId: { value: "owner" }, refresh: vi.fn(), selectInitialRole: vi.fn(), conversations: vi.fn(), projects: vi.fn() }));
vi.mock("@/composables/useDeviceOwnedConversation", () => ({ useDeviceOwnedConversation: () => owned }));
vi.mock("@/composables/useApi", () => ({ apiClient: {} }));
import { useChatStore } from "@/stores/chat";
describe("sidebar initialization", () => {
  beforeEach(() => { setActivePinia(createPinia()); vi.clearAllMocks(); owned.enabled.value = false; owned.selectInitialRole.mockReturnValue("one"); owned.conversations.mockResolvedValue([]); owned.projects.mockResolvedValue({ projects: [], historicalRoles: [], selectedHistoricalRole: "", executionScope: {} }); });
  it("waits for the device service before loading the sidebar", async () => {
    let resolve: (value: boolean) => void = () => {};
    owned.refresh.mockImplementation(() => new Promise(done => { resolve = done; }));
    const loading = useChatStore().fetchSidebar();
    expect(owned.conversations).not.toHaveBeenCalled();
    owned.enabled.value = true;
    resolve(true);
    await loading;
    expect(owned.conversations).toHaveBeenCalledWith("one");
  });
  it("does not query conversations when initialization fails", async () => {
    owned.refresh.mockResolvedValue(false);
    await expect(useChatStore().fetchSidebar()).rejects.toThrow("设备对话服务尚未就绪");
    expect(owned.conversations).not.toHaveBeenCalled();
  });
});
