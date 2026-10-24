import { mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { describe, expect, it } from "vitest";
import ChatHeaderBar from "../components/ChatHeaderBar.vue";
import webChatSource from "../views/web-chat/BuiltinWebChatView.vue?raw";
import titleBarSource from "../components/DesktopTitleBar.vue?raw";

function mountHeader(charAvatar = "") {
  return mount(ChatHeaderBar, {
    props: {
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
    global: { plugins: [ElementPlus] },
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

  it("角色没有图片时使用角色名首字作为头像回退", () => {
    const wrapper = mountHeader();
    try {
      expect(wrapper.get(".header-character-avatar").text()).toBe("小");
    } finally {
      wrapper.unmount();
    }
  });

  it("聊天页通过 Teleport 把标题和操作区放入窗口标题栏", () => {
    expect(webChatSource).toContain('to="#window-chat-slot"');
    expect(webChatSource).toContain("'is-titlebar': useTitlebarChrome");
    expect(webChatSource).toContain("const useTitlebarChrome = ref(window.innerWidth >= 900)");
    expect(webChatSource).toContain("const headerHeight = useTitlebarChrome.value ? 0");
  });

  it("标题左侧竖线只占标题栏一半高度并垂直居中", () => {
    expect(titleBarSource).toContain(".window-chat-slot::before");
    expect(titleBarSource).toContain("top: 50%;");
    expect(titleBarSource).toContain("height: 50%;");
    expect(titleBarSource).toContain("transform: translateY(-50%);");
  });
});
