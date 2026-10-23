import { beforeEach, describe, expect, it, vi } from "vitest";
import { shallowMount, flushPromises } from "@vue/test-utils";
import { ref } from "vue";
const mocks = vi.hoisted(() => ({ load: vi.fn(), create: vi.fn(), remove: vi.fn() }));
vi.mock("@/runtime/message-feedback", () => ({ loadMessageFeedback: mocks.load, createMessageFeedback: mocks.create, deleteMessageFeedback: mocks.remove }));
vi.mock("@/composables/useTheme", () => ({ useTheme: () => ({ resolvedMode: ref("light") }) }));
vi.mock("@/composables/useChatAppearancePreference", () => ({ useChatAppearancePreference: () => ({ messageStyle: ref("flow") }) }));
vi.mock("@/conversation/rendering/markdown/MarkdownContent.vue", () => ({ default: { template: "<div />" } }));
import Renderer from "@/conversation/rendering/AIMessageRenderer.vue";
const message = { id: "reply", role: "assistant", content: "hello", status: "streaming", ownerId: "owner", executionScope: { coreId: "core", roleId: "one" } };
describe("message feedback lifecycle", () => {
  it("shows immediate waiting, ends thinking before text, and hides the caret before save confirmation", async () => {
    const wrapper = shallowMount(Renderer, { global: { stubs: { "el-icon": true } }, props: { characterId: "one", message: { ...message, content: "", status: "queued" } } });
    expect(wrapper.find("amitia-thinking-block-stub").exists()).toBe(true);
    await wrapper.setProps({ message: { ...message, content: "", reasoningContent: "思考内容" } });
    expect(wrapper.find("amitia-thinking-block-stub").attributes("state")).toBe("streaming");
    await wrapper.setProps({ message: { ...message, content: "正文", reasoningContent: "思考内容", reasoningComplete: true } });
    expect(wrapper.find("amitia-thinking-block-stub").attributes("state")).toBe("completed");
    await wrapper.setProps({ message: { ...message, content: "正文", reasoningContent: "思考内容", reasoningComplete: true, outputComplete: true } });
    expect(wrapper.text()).toContain("正在确认保存");
    expect(wrapper.find("amitia-thinking-block-stub").attributes("state")).toBe("completed");
    await wrapper.setProps({ message: { ...message, content: "正文", status: "completed", saved: true, sourceRevision: 1, outputComplete: true } });
    expect(wrapper.text()).not.toContain("正在确认保存");
    wrapper.unmount();
  });

  beforeEach(() => { vi.clearAllMocks(); mocks.load.mockResolvedValue(null); });
  it("waits for persistence then loads feedback for the original authority", async () => {
    const wrapper = shallowMount(Renderer, { global: { stubs: { "el-icon": true } }, props: { message, characterId: "one" } });
    await flushPromises();
    expect(mocks.load).not.toHaveBeenCalled();
    await wrapper.setProps({ message: { ...message, status: "completed", saved: false } });
    expect(mocks.load).not.toHaveBeenCalled();
    await wrapper.setProps({ message: { ...message, status: "completed", saved: true, sourceRevision: 1 } });
    await flushPromises();
    expect(mocks.load).toHaveBeenCalledTimes(1);
    expect(mocks.load).toHaveBeenCalledWith("reply", "one", message.executionScope);
    wrapper.unmount();
  });
  it("discards feedback that arrives after changing role", async () => {
    let resolve: (value: any) => void = () => {};
    mocks.load.mockImplementationOnce(() => new Promise(done => { resolve = done; }));
    const wrapper = shallowMount(Renderer, { global: { stubs: { "el-icon": true } }, props: { message: { ...message, status: "completed", sourceRevision: 1 }, characterId: "one" } });
    await wrapper.setProps({ characterId: "two", message: { ...message, status: "completed", sourceRevision: 1, executionScope: { coreId: "core", roleId: "two" } } });
    await flushPromises();
    resolve({ id: 1, feedbackType: "good" });
    await flushPromises();
    expect(wrapper.find('[title="有帮助"]').classes()).not.toContain("is-active");
    wrapper.unmount();
  });
});
