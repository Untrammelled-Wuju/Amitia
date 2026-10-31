import { mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { describe, expect, it, vi } from "vitest";

const owned = vi.hoisted(() => ({
  roles: { value: [{ id: "role-a", name: "小夏", avatar: "" }] },
}));

vi.mock("@/composables/useDeviceOwnedConversation", () => ({
  useDeviceOwnedConversation: () => owned,
}));

import SidebarConversationRow from "@/components/SidebarConversationRow.vue";

describe("桌面端消息行活动投影", () => {
  it("左侧显示头像和未读数，中间显示工具状态，右侧显示时间与加载动画", () => {
    const wrapper = mount(SidebarConversationRow, {
      props: {
        active: false,
        conversation: {
          id: "conversation-a",
          projectId: "",
          title: "小夏",
          channel: "web",
          source: "manual",
          messageCount: 10,
          characterId: "role-a",
          isGenerating: true,
          unreadCompletedCount: 3,
          hasUnread: true,
          activityLabel: "正在使用 workspace.read 工具",
          lastMessage: "最近一条消息",
          lastMessageType: "text",
          createdAt: "",
          updatedAt: new Date().toISOString(),
        },
      },
      global: { plugins: [ElementPlus] },
    });

    expect(wrapper.find(".thread-avatar-wrap").exists()).toBe(true);
    expect(wrapper.find(".thread-unread").text()).toBe("3");
    expect(wrapper.find(".thread-title").text()).toBe("小夏");
    expect(wrapper.find(".thread-preview").text()).toBe("正在使用 workspace.read 工具");
    expect(wrapper.find(".thread-meta time").exists()).toBe(true);
    expect(wrapper.find(".thread-spinner").exists()).toBe(true);
    expect(wrapper.text()).not.toContain("10 条消息");
    wrapper.unmount();
  });
});
