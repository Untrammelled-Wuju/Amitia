import { flushPromises, mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { createPinia, setActivePinia } from "pinia";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import ChatInput from "../components/ChatInput.vue";
import type { AgentSkillDefinition } from "../views/extensions/types";
import type { OwnedGroupMember } from "@/runtime/device-owned-chat";

const fetchAgentSkillsMock = vi.hoisted(() => vi.fn());

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
  fetchAgentSkills: fetchAgentSkillsMock,
}));

const skills = [
  {
    id: "skill-a",
    extensionId: "extension-a",
    name: "browser-research",
    displayName: "浏览器研究",
    description: "检索网页并整理资料",
    shortDescription: "检索网页",
    scope: "global",
    enabled: true,
    source: "agent_skill",
    metadata: {},
    contentHash: "hash-a",
    resources: [],
    toolMappings: [],
    mcpDependencies: [],
    compatibilityStatus: "compatible",
    warnings: [],
    createdAt: "2026-10-27T00:00:00Z",
    updatedAt: "2026-10-27T00:00:00Z",
  },
] as AgentSkillDefinition[];

const members = [
  {
    memberId: "member-a",
    characterId: "role-a",
    roleOwnerId: "owner",
    characterName: "角色甲",
    status: "active",
    orderIndex: 0,
    joinedSequence: 0,
    historyStartSequence: 0,
    characterRevision: 1,
  },
] as OwnedGroupMember[];

function page(items = skills) {
  return {
    items,
    total: items.length,
    page: 1,
    pageSize: 50,
  };
}

async function mountInput(props: Record<string, unknown> = {}) {
  const pinia = createPinia();
  setActivePinia(pinia);
  return mount(ChatInput, {
    attachTo: document.body,
    props,
    global: {
      plugins: [pinia, ElementPlus],
    },
  });
}

describe("ChatInput command palette", () => {
  beforeEach(() => {
    localStorage.clear();
    const overlayRoot = document.createElement("div");
    overlayRoot.id = "amitia-overlay-root";
    document.body.appendChild(overlayRoot);
    fetchAgentSkillsMock.mockReset();
    fetchAgentSkillsMock.mockResolvedValue(page());
  });

  afterEach(() => {
    document.body.innerHTML = "";
  });

  it("uses one file input and one upload row in the plus menu", async () => {
    const wrapper = await mountInput();
    await flushPromises();
    expect(wrapper.findAll('input[type="file"]')).toHaveLength(1);
    expect(wrapper.get('input[type="file"]').attributes("accept")).toBeUndefined();

    await wrapper.get(".add-btn").trigger("click");
    const picker = wrapper.get(".composer-picker");
    expect(picker.classes()).toContain("composer-picker");
    expect(wrapper.findAll(".picker-action")).toHaveLength(1);
    expect(wrapper.get(".picker-action").text()).toContain("上传文件");
    expect(wrapper.find(".picker-action .picker-icon .el-icon").exists()).toBe(
      true,
    );
    expect(wrapper.get(".picker-option").text()).toContain("浏览器研究");
    expect(wrapper.get("textarea").attributes("aria-activedescendant")).toBe(
      "composer-picker-upload",
    );
    wrapper.unmount();
  });

  it("opens skills from slash and selects with the keyboard", async () => {
    const wrapper = await mountInput();
    await flushPromises();
    await wrapper.get("textarea").setValue("/");
    await new Promise((resolve) => setTimeout(resolve, 200));
    await flushPromises();

    expect(wrapper.get(".composer-picker").text()).toContain("选择技能");
    await wrapper.get("textarea").trigger("keydown", { key: "Enter" });
    await flushPromises();

    expect(wrapper.find(".composer-picker").exists()).toBe(false);
    expect(wrapper.get(".skill-context-chip").text()).toContain(
      "$browser-research",
    );
    wrapper.unmount();
  });

  it("only opens mentions in group chats and inserts the selected member", async () => {
    const direct = await mountInput();
    await flushPromises();
    await direct.get("textarea").setValue("@");
    expect(direct.find(".composer-picker").exists()).toBe(false);
    direct.unmount();

    const group = await mountInput({
      isGroup: true,
      groupMembers: members,
    });
    await flushPromises();
    await group.get("textarea").setValue("@");
    expect(group.get(".composer-picker").text()).toContain("群成员");

    await group.get("textarea").trigger("keydown", { key: "ArrowDown" });
    await group.get("textarea").trigger("keydown", { key: "Enter" });
    await flushPromises();
    expect(group.get("textarea").element.value).toBe("@角色甲 ");
    group.unmount();
  });

  it("loads the next skill page when the list reaches the bottom", async () => {
    fetchAgentSkillsMock
      .mockResolvedValueOnce({ ...page(), total: 100 })
      .mockResolvedValueOnce({ ...page(), page: 2 });
    const wrapper = await mountInput();
    await flushPromises();
    await wrapper.get(".add-btn").trigger("click");

    const list = wrapper.get(".picker-list");
    Object.defineProperty(list.element, "scrollHeight", {
      configurable: true,
      value: 300,
    });
    Object.defineProperty(list.element, "clientHeight", {
      configurable: true,
      value: 200,
    });
    Object.defineProperty(list.element, "scrollTop", {
      configurable: true,
      value: 80,
    });
    await list.trigger("scroll");
    await flushPromises();

    expect(fetchAgentSkillsMock).toHaveBeenLastCalledWith(
      expect.objectContaining({ page: 2, pageSize: 50 }),
    );
    wrapper.unmount();
  });

  it("routes a generic file through the unified file input", async () => {
    const wrapper = await mountInput();
    await flushPromises();
    const input = wrapper.get('input[type="file"]');
    const file = new File(["report"], "report.pdf", {
      type: "application/pdf",
    });
    Object.defineProperty(input.element, "files", {
      configurable: true,
      value: [file],
    });
    await input.trigger("change");
    expect(wrapper.emitted("file")?.[0]).toEqual([file]);
    wrapper.unmount();
  });
});
