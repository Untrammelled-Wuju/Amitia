import { beforeEach, describe, expect, it, vi } from "vitest";
import { ref } from "vue";
import { useWebChatConversation } from "../composables/useWebChatConversation";
const mocks = vi.hoisted(() => ({ get: vi.fn(), summary: vi.fn(), enabled: { value: true } }));
vi.mock("../composables/useApi", () => ({ useApi: () => ({ get: mocks.get, del: vi.fn() }) }));
vi.mock("../composables/useCachedApi", () => ({ useCachedApi: () => ({ saveCache: vi.fn() }) }));
vi.mock("../composables/useDeviceOwnedConversation", () => ({ useDeviceOwnedConversation: () => ({ enabled: mocks.enabled, conversationSummary: mocks.summary }) }));
vi.mock("../stores/chat", () => ({ useChatStore: () => ({}) }));
vi.mock("../composables/useConversationWorkspace", () => ({ useConversationWorkspace: () => ({ applySnapshotWorkspace: vi.fn() }) }));
describe("网页摘要读取原设备历史", () => {
  beforeEach(() => { mocks.get.mockReset(); mocks.summary.mockReset(); mocks.enabled.value = true; });
  const conversation = () => useWebChatConversation(ref([]), ref("meshconv1:source:chat"), ref("role"), ref(""), ref(""), ref(""), ref(""), ref(false), vi.fn(), vi.fn());
  it("Core 无当前摘要时可显示 Source 历史摘要，保留 Owner 引用且不读 legacy API", async () => {
    mocks.summary.mockResolvedValue({ summaryText: "旧设备聊天摘要", sourceOwnerId: "source", editable: false });
    expect(await conversation().fetchConvSummary()).toBe("旧设备聊天摘要");
    expect(mocks.summary).toHaveBeenCalledWith("meshconv1:source:chat", "role");
    expect(mocks.get).not.toHaveBeenCalled();
  });
  it("历史旧格式摘要保持可读，查询失败不会回落到另一 Owner API", async () => {
    mocks.summary.mockResolvedValue({ summary_text: "历史旧格式" });
    expect(await conversation().fetchConvSummary()).toBe("历史旧格式");
    mocks.summary.mockRejectedValue(new Error("Core 已切换"));
    expect(await conversation().fetchConvSummary()).toBe("");
    expect(mocks.get).not.toHaveBeenCalled();
  });
});
