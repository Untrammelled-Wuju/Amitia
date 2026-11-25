import { flushPromises, mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { createPinia } from "pinia";
import { describe, expect, it, vi } from "vitest";
import { apiClient } from "../composables/useApi";
import ChatHeaderBar from "../components/ChatHeaderBar.vue";
import webChatSource from "../views/web-chat/BuiltinWebChatView.vue?raw";
import { useExtensionUIStore } from "../stores/extensionUI";
import titleBarSource from "../components/DesktopTitleBar.vue?raw";

function mountHeader(charAvatar = "", pinia = createPinia()) {
  return mount(ChatHeaderBar, {
    props: {
      characterId: "role/1",
      charName: "小夏",
      charAvatar,
      charIdentity: "",
      convTitle: "周末旅行计划",
      messagesCount: 2,
      convId: "conversation-1",
      showProfiles: false,
      showMemInject: false,
      callActive: false,
    },
    global: { plugins: [pinia, ElementPlus] },
  });
}

describe("聊天窗口标题栏", () => {
  it("只渲染对话标题和当前角色头像，不再显示品牌副标题", () => {
    const wrapper = mountHeader("/avatar.png");
    try {
      expect(wrapper.get(".header-conv-name").text()).toBe("周末旅行计划");
      expect(wrapper.get(".header-character-avatar img").attributes("src")).toBe("/avatar.png");
      expect(wrapper.text()).not.toContain("Amitia · AI 陪伴角色");
    } finally {
      wrapper.unmount();
    }
  });

  it("标题插槽等待会话 ID 时仍显示新对话", async () => {
    const pinia = createPinia();
    const generatedAt = new Date().toISOString();
    useExtensionUIStore(pinia).snapshot = {
      generatedAt,
      version: 1,
      slots: [{
        slotId: "chat.header.lineage",
        contractVersion: 1,
        kind: "single",
        layout: "inline",
        scope: "session",
        fallbackPolicy: "default",
        contributions: [],
        generatedAt,
      }],
    };
    const wrapper = mountHeader("", pinia);
    try {
      await wrapper.setProps({ convId: "", convTitle: "" });
      expect(wrapper.get(".header-conv-name").text()).toBe("新对话");
      await wrapper.setProps({ convId: "saved", convTitle: "已保存的对话标题" });
      expect(wrapper.get(".header-conv-name").text()).toBe("已保存的对话标题");
    } finally {
      wrapper.unmount();
    }
  });

  it("角色没有图片时使用角色名首字作为头像回退", () => {
    const wrapper = mountHeader();
    try {
      expect(wrapper.get(".header-character-avatar").text()).toBe("小");
    } finally {
      wrapper.unmount();
    }
  });

  it("将已上传的角色头像通过鉴权接口加载", async () => {
    const createObjectURL = vi.fn(() => "blob:role-avatar");
    const revokeObjectURL = vi.fn();
    vi.stubGlobal("URL", Object.assign(class extends URL {}, { createObjectURL, revokeObjectURL }));
    const get = vi.spyOn(apiClient, "get").mockResolvedValue({ data: new Blob([], { type: "image/png" }) });
    const wrapper = mountHeader("/avatars/role.png");
    try {
      await flushPromises();
      expect(get).toHaveBeenCalledWith("/api/characters/role%2F1/avatar", expect.objectContaining({
        params: { v: "/avatars/role.png" },
        responseType: "blob",
      }));
      expect(wrapper.get(".header-character-avatar img").attributes("src")).toBe("blob:role-avatar");
      await wrapper.setProps({ charAvatar: "/next.png" });
      expect(revokeObjectURL).toHaveBeenCalledWith("blob:role-avatar");
      expect(wrapper.get(".header-character-avatar img").attributes("src")).toBe("/next.png");
    } finally {
      wrapper.unmount();
      get.mockRestore();
      vi.unstubAllGlobals();
    }
  });

  it("添加成员后在标题右侧显示各成员头像，添加按钮仍可打开成员管理", async () => {
    const wrapper = mountHeader("/current.png");
    try {
      await wrapper.setProps({
        isGroup: true,
        groupMembers: [1, 2, 3, 4].map((index) => ({
          memberId: `member-${index}`,
          characterId: `role-${index}`,
          roleOwnerId: "owner",
          characterName: `角色 ${index}`,
          characterAvatar: `/role-${index}.png`,
          status: "active",
          orderIndex: index,
          joinedSequence: 0,
          historyStartSequence: 0,
          characterRevision: 1,
        })),
      });
      const avatars = wrapper.findAll(".header-info + .header-group-avatars img");
      expect(avatars.map((avatar) => avatar.attributes("src"))).toEqual([
        "/role-1.png", "/role-2.png", "/role-3.png", "/role-4.png",
      ]);
      const button = wrapper.get(".header-member-entry");
      expect(button.find(".header-character-avatar").exists()).toBe(false);
      await button.trigger("click");
      expect(wrapper.emitted("manageGroup")).toHaveLength(1);
    } finally {
      wrapper.unmount();
    }
  });

  it("聊天页通过 Teleport 把标题和操作区放入窗口标题栏", () => {
    expect(webChatSource).toContain(':to="titlebarTeleportTarget"');
    expect(webChatSource).toContain('document.querySelector<HTMLElement>("#window-chat-slot")');
    expect(webChatSource).toContain('v-if="titlebarTeleportReady"');
    expect(webChatSource).toContain("'is-titlebar': useTitlebarChrome");
    expect(webChatSource).toContain("const useTitlebarChrome = ref(!props.embedded && window.innerWidth >= 900)");
    expect(webChatSource).toContain("const headerHeight = useTitlebarChrome.value ? 0");
  });

  it("标题左侧竖线只占标题栏一半高度并垂直居中", () => {
    expect(titleBarSource).toContain(".window-chat-slot::before");
    expect(titleBarSource).toContain("top: 50%;");
    expect(titleBarSource).toContain("height: 50%;");
    expect(titleBarSource).toContain("transform: translateY(-50%);");
  });
});
