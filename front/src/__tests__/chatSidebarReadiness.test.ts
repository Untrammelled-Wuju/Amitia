import { beforeEach, describe, expect, it, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
const owned = vi.hoisted(() => ({ enabled: { value: false }, roles: { value: [{ id: "one" }] }, notice: { value: "" }, policy: { value: null }, coreId: { value: "core", __v_isRef: true }, roleOwnerId: { value: "owner" }, refresh: vi.fn(), selectInitialRole: vi.fn(), conversations: vi.fn(), projects: vi.fn() }));
vi.mock("@/composables/useDeviceOwnedConversation", () => ({ useDeviceOwnedConversation: () => owned }));
vi.mock("@/composables/useApi", () => ({ apiClient: {} }));
import { useChatStore } from "@/stores/chat";
describe("sidebar initialization", () => {
  beforeEach(() => { setActivePinia(createPinia()); vi.resetAllMocks(); owned.roles.value = [{ id: "one" }]; owned.enabled.value = false; owned.selectInitialRole.mockReturnValue("one"); owned.conversations.mockResolvedValue([]); owned.projects.mockResolvedValue({ projects: [], historicalRoles: [], selectedHistoricalRole: "", executionScope: {} }); });
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
  it("shows unread reminders from another role in the same sidebar", async () => {
    owned.enabled.value = true;
    owned.roles.value = [{ id: "one" }, { id: "two" }];
    owned.conversations.mockImplementation(async (role: string) => role === "one"
      ? []
      : [{ id: "reminder", source: "reminder", channel: "web", characterId: "two", hasUnread: true, unreadCompletedCount: 1 }, { id: "other", source: "manual", channel: "web" }]);
    const store = useChatStore();
    await store.fetchSidebar();
    expect(store.sidebar.recent.map((item) => item.id)).toEqual(["reminder"]);
    expect(store.sidebar.recent[0].unreadCompletedCount).toBe(1);
  });
  it("refreshes again when a reminder arrives during an existing sidebar load", async () => {
    owned.enabled.value = true;
    let complete: (value: unknown[]) => void = () => {};
    owned.conversations.mockImplementationOnce(() => new Promise((resolve) => { complete = resolve; }));
    const store = useChatStore();
    const first = store.fetchSidebar();
    const afterReminder = store.fetchSidebar(true);
    complete([]);
    await Promise.all([first, afterReminder]);
    expect(owned.conversations).toHaveBeenCalledTimes(2);
  });
});
