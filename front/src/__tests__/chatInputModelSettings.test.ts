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
  afterEach(() => {
    document.body.innerHTML = "";
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
});
