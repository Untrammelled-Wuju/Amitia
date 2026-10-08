import { mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { createPinia, setActivePinia } from "pinia";
import { afterEach, describe, expect, it, vi } from "vitest";
import ChatInput from "../components/ChatInput.vue";

vi.mock("../composables/useConversationWorkspace", async () => {
  const { ref } = await import("vue");
  return {
    useConversationWorkspace: () => ({
      currentWorkspace: ref(null),
      recentWorkspaces: ref([]),
      workspaceLoading: ref(false),
      refreshRecentWorkspaces: vi.fn(),
      loadConversationWorkspace: vi.fn(),
      chooseWorkspaceDirectory: vi.fn(),
      selectWorkspaceMount: vi.fn(),
      clearWorkspace: vi.fn(),
    }),
  };
});

vi.mock("../views/extensions/api", () => ({
  fetchAgentSkills: vi.fn().mockResolvedValue([]),
}));

describe("ChatInput model settings", () => {
  it("绑定Core后即时移除已打开模型和权限配置入口", async () => {
    const overlayRoot = document.createElement("div");
    overlayRoot.id = "amitia-overlay-root";
    document.body.appendChild(overlayRoot);
    const pinia = createPinia();
    setActivePinia(pinia);
    const wrapper = mount(ChatInput, { props: { models: [{ id: 1, name: "模型", modelName: "test" }], selectedModelId: 1 }, global: { plugins: [pinia, ElementPlus] } });
    try {
      await wrapper.get(".model-effort-trigger").trigger("click");
      await wrapper.setProps({ deviceOwned: true });
      expect(wrapper.find(".model-effort-trigger").exists()).toBe(false);
      expect(wrapper.find(".permission-trigger").exists()).toBe(false);
      expect(wrapper.emitted("model-change")).toBeUndefined();
    } finally { wrapper.unmount(); }
  });
  it("switches the trailing action between voice, send and stop", async () => {
    localStorage.clear();
    const overlayRoot = document.createElement("div");
    overlayRoot.id = "amitia-overlay-root";
    document.body.appendChild(overlayRoot);
    const pinia = createPinia();
    setActivePinia(pinia);
    const wrapper = mount(ChatInput, { global: { plugins: [pinia, ElementPlus] } });
    try {
      expect(wrapper.find('[aria-label="切换到语音输入"]').exists()).toBe(true);
      expect(wrapper.find('[aria-label="发送消息"]').exists()).toBe(false);
      await wrapper.get('textarea').setValue('你好');
      expect(wrapper.find('[aria-label="切换到语音输入"]').exists()).toBe(false);
      await wrapper.get('[aria-label="发送消息"]').trigger('click');
      expect(wrapper.emitted('send')?.[0]).toEqual(['你好']);
      expect(wrapper.find('[aria-label="切换到语音输入"]').exists()).toBe(true);
      await wrapper.get('textarea').setValue('   ');
      expect(wrapper.find('[aria-label="发送消息"]').exists()).toBe(false);
      await wrapper.get('[aria-label="切换到语音输入"]').trigger('click');
      expect(wrapper.get('.hold-voice-btn').isVisible()).toBe(true);
      await wrapper.get('[aria-label="切换到文字输入"]').trigger('click');
      expect(wrapper.get('textarea').isVisible()).toBe(true);
      await wrapper.setProps({ generating: true });
      await wrapper.get('[aria-label="停止生成"]').trigger('click');
      expect(wrapper.emitted('stop')).toHaveLength(1);
      expect(wrapper.find('.voice-mode-toggle').exists()).toBe(false);
    } finally {
      wrapper.unmount();
      localStorage.clear();
    }
  });

  afterEach(() => {
    document.body.innerHTML = "";
  });

  it("shows the request model instead of the config alias in the composer trigger", () => {
    const overlayRoot = document.createElement("div");
    overlayRoot.id = "amitia-overlay-root";
    document.body.appendChild(overlayRoot);
    const pinia = createPinia();
    setActivePinia(pinia);
    const wrapper = mount(ChatInput, {
      props: {
        models: [
          {
            id: 1,
            name: "123",
            modelName: "gpt-5.1",
            apiType: "openai",
          },
        ],
        selectedModelId: 1,
        reasoningEffort: "medium",
      },
      global: {
        plugins: [pinia, ElementPlus],
      },
    });

    const trigger = wrapper.get(".model-effort-trigger");
    expect(trigger.text()).toContain("gpt-5.1");
    expect(trigger.text()).toContain("中");
    expect(trigger.text()).not.toContain("123");
  });

  it("notifies the selected model and closes the model page", async () => {
    const overlayRoot = document.createElement("div");
    overlayRoot.id = "amitia-overlay-root";
    document.body.appendChild(overlayRoot);
    const preview = vi.fn();
    const commit = vi.fn();
    const pinia = createPinia();
    setActivePinia(pinia);
    const wrapper = mount(ChatInput, {
      props: {
        models: [
          {
            id: 1,
            name: "gpt-5",
            modelName: "gpt-5",
            apiType: "openai",
            supportsReasoning: true,
            defaultReasoningEffort: "medium",
          },
          {
            id: 2,
            name: "gpt-5.1",
            modelName: "gpt-5.1",
            apiType: "openai",
            supportsReasoning: true,
            defaultReasoningEffort: "high",
          },
        ],
        selectedModelId: 1,
        reasoningEffort: "medium",
        reasoningEnabled: true,
        modelPreviewChange: preview,
        modelCommitChange: commit,
      },
      global: {
        plugins: [pinia, ElementPlus],
      },
    });

    const state = (wrapper.vm as any).$.setupState;
    state.selectModel({
      id: 2,
      name: "gpt-5.1",
      modelName: "gpt-5.1",
      apiType: "openai",
      supportsReasoning: true,
      defaultReasoningEffort: "high",
    });
    await wrapper.vm.$nextTick();

    expect(commit).toHaveBeenCalledWith(2, "high", true);
    expect(state.modelMenuView).toBe("main");

    await wrapper.setProps({
      selectedModelId: 2,
      reasoningEffort: "high",
      reasoningEnabled: true,
    });
    state.selectReasoningEnabled(false);
    expect(commit).toHaveBeenLastCalledWith(2, "high", false);

    state.selectReasoningEnabled(true);
    state.applyReasoningIndex(3);
    expect(commit).toHaveBeenLastCalledWith(2, "xhigh", true);
    await wrapper.vm.$nextTick();
    expect(state.reasoningActiveWidth).toBe("calc(100% - 16px)");

    state.applyReasoningIndex(0);
    await wrapper.vm.$nextTick();
    expect(state.reasoningActiveWidth).toBe("16px");
  });

  it("keeps popup content visible after switching to the model page", async () => {
    const overlayRoot = document.createElement("div");
    overlayRoot.id = "amitia-overlay-root";
    document.body.appendChild(overlayRoot);
    const pinia = createPinia();
    setActivePinia(pinia);
    const wrapper = mount(ChatInput, {
      attachTo: document.body,
      props: {
        models: [
          {
            id: 1,
            name: "gpt-5",
            modelName: "gpt-5",
            apiType: "openai",
            supportsReasoning: true,
            defaultReasoningEffort: "medium",
          },
          {
            id: 2,
            name: "gpt-5.1",
            modelName: "gpt-5.1",
            apiType: "openai",
            supportsReasoning: true,
            defaultReasoningEffort: "high",
          },
        ],
        selectedModelId: 1,
        reasoningEffort: "medium",
        reasoningEnabled: true,
      },
      global: {
        plugins: [pinia, ElementPlus],
      },
    });

    await wrapper.find(".model-effort-trigger").trigger("click");
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(overlayRoot.textContent).toContain("强度");
    const state = (wrapper.vm as any).$.setupState;
    state.applyReasoningIndex(0);
    await wrapper.vm.$nextTick();
    expect(
      overlayRoot
        .querySelector(".model-effort-track-active")
        ?.classList.contains("is-empty"),
    ).toBe(true);

    const modelButton = Array.from(
      overlayRoot.querySelectorAll("button"),
    ).find((button) => button.textContent?.includes("模型"));
    expect(modelButton).toBeDefined();
    modelButton?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await wrapper.vm.$nextTick();
    await new Promise((resolve) => setTimeout(resolve, 20));

    expect(overlayRoot.textContent).toContain("选择模型");
    expect(overlayRoot.textContent).toContain("gpt-5.1");
    const viewport = overlayRoot.querySelector(
      ".model-menu-viewport",
    ) as HTMLElement | null;
    expect(viewport?.style.height || "auto").not.toBe("0px");
  });

  it("emits the selected permission mode", async () => {
    const overlayRoot = document.createElement("div");
    overlayRoot.id = "amitia-overlay-root";
    document.body.appendChild(overlayRoot);
    const pinia = createPinia();
    setActivePinia(pinia);
    const wrapper = mount(ChatInput, {
      props: {
        permissionMode: "request_approval",
      },
      global: {
        plugins: [pinia, ElementPlus],
      },
    });
    const state = (wrapper.vm as any).$.setupState;
    state.selectPermissionMode("full_access");
    expect(wrapper.emitted("update:permission")?.at(-1)).toEqual([
      "full_access",
    ]);
  });

  it("places permission before workspace in the left action group", () => {
    const overlayRoot = document.createElement("div");
    overlayRoot.id = "amitia-overlay-root";
    document.body.appendChild(overlayRoot);
    (window as any).amitiaDesktop = {
      selectWorkspaceDirectory: vi.fn(),
    };
    const pinia = createPinia();
    setActivePinia(pinia);
    const wrapper = mount(ChatInput, {
      global: {
        plugins: [pinia, ElementPlus],
      },
    });

    const add = wrapper.get(".add-btn").element;
    const permission = wrapper.get(".permission-trigger").element;
    const workspace = wrapper.get(".workspace-trigger").element;

    expect(
      add.compareDocumentPosition(permission) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      permission.compareDocumentPosition(workspace) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();

    delete (window as any).amitiaDesktop;
  });
});
