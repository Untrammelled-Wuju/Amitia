import { flushPromises, mount } from "@vue/test-utils";
import ElementPlus, { ElMessageBox } from "element-plus";
import { beforeEach, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({ edit: vi.fn(), fetch: vi.fn(), conversations: vi.fn() }));
vi.mock("@/composables/useDeviceOwnedConversation", () => ({
  useDeviceOwnedConversation: () => ({ enabled: { value: true }, coreId: { value: "core" }, roleOwnerId: { value: "owner" }, roles: { value: [] }, notice: { value: "" }, refresh: async () => true, selectInitialRole: () => "role", projects: async () => ({ projects: [] }), conversations: mocks.conversations, edit: mocks.edit }),
}));
vi.mock("@/stores/chat", () => ({ useChatStore: () => ({ archivedRevision: 0, fetchSidebar: mocks.fetch }) }));
vi.mock("@/components/ChatBubble.vue", () => ({ default: { template: "<div />" } }));
import ChatLogsView from "@/views/chat-logs/ChatLogsView.vue";

beforeEach(() => {
  mocks.edit.mockReset();
  mocks.fetch.mockReset().mockResolvedValue(undefined);
  mocks.conversations.mockReset().mockResolvedValue([
    { id: "one", title: "一", archivedAt: "2026-01-01", characterId: "role" },
    { id: "two", title: "二", archivedAt: "2026-01-02", characterId: "role" },
  ]);
});

it("批量撤销时保留失败的对话和完整原始错误", async () => {
  mocks.edit.mockImplementation(async (kind, id) => { if (id === "one") throw new Error("原始恢复错误"); });
  const alert = vi.spyOn(ElMessageBox, "alert").mockResolvedValue("confirm" as any);
  const wrapper = mount(ChatLogsView, { global: { plugins: [ElementPlus] } });
  await flushPromises();
  await wrapper.findAll("button").find((button) => button.text() === "多选")!.trigger("click");
  await wrapper.findAll("button").find((button) => button.text() === "全选")!.trigger("click");
  expect(wrapper.findAll('input[type="checkbox"]').every((item) => (item.element as HTMLInputElement).checked)).toBe(true);
  await wrapper.findAll("button").find((button) => button.text() === "撤销")!.trigger("click");
  await flushPromises();
  expect(mocks.edit).toHaveBeenCalledTimes(2);
  expect(wrapper.findAll(".archive-item")).toHaveLength(1);
  expect(wrapper.find(".archive-item").text()).toContain("一");
  expect((wrapper.get('input[type="checkbox"]').element as HTMLInputElement).checked).toBe(true);
  expect(String(alert.mock.calls[0][0])).toContain("原始恢复错误");
  alert.mockRestore();
  wrapper.unmount();
});

it("永久删除须确认，并且全选可切换为取消全选", async () => {
  mocks.edit.mockResolvedValue(undefined);
  const confirm = vi.spyOn(ElMessageBox, "confirm").mockResolvedValue("confirm" as any);
  const wrapper = mount(ChatLogsView, { global: { plugins: [ElementPlus] } });
  await flushPromises();
  await wrapper.findAll("button").find((button) => button.text() === "多选")!.trigger("click");
  await wrapper.findAll("button").find((button) => button.text() === "全选")!.trigger("click");
  expect(wrapper.text()).toContain("取消全选");
  await wrapper.findAll("button").find((button) => button.text() === "删除（永久）")!.trigger("click");
  await flushPromises();
  expect(confirm).toHaveBeenCalledOnce();
  expect(mocks.edit).toHaveBeenCalledTimes(2);
  expect(mocks.edit.mock.calls.every((call) => call[3].deleted === true)).toBe(true);
  expect(wrapper.findAll(".archive-item")).toHaveLength(0);
  confirm.mockRestore();
  wrapper.unmount();
});
