import { mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { afterEach, describe, expect, it, vi } from "vitest";
import { nextTick } from "vue";

const owned = vi.hoisted(() => ({
  roles: { value: [{ id: "role-a", name: "小夏", avatar: "" }] },
}));

vi.mock("@/composables/useDeviceOwnedConversation", () => ({
  useDeviceOwnedConversation: () => owned,
}));

vi.mock("@/runtime/runtime-capabilities", () => ({
  isDesktopShell: () => true,
}));

import SidebarConversationRow from "@/components/SidebarConversationRow.vue";
import {
  closeContextMenu,
  contextMenuState,
} from "@/components/context-menu/contextMenuStore";

const conversation = {
  id: "conversation-a",
  projectId: "project-a",
  title: "项目对话",
  channel: "web",
  source: "manual",
  messageCount: 10,
  characterId: "role-a",
  isGenerating: false,
  unreadCompletedCount: 3,
  hasUnread: true,
  lastTerminalTurnSequence: 8,
  activityLabel: "",
  lastMessage: "最近一条消息",
  lastMessageType: "text",
  createdAt: "",
  updatedAt: new Date().toISOString(),
};

describe("桌面端消息右键菜单", () => {
  afterEach(() => {
    closeContextMenu();
  });

  it("移除行内操作按钮，并在右键时显示完整消息操作", async () => {
    const wrapper = mount(SidebarConversationRow, {
      props: {
        active: false,
        conversation,
        projects: [
          {
            id: "project-a",
            name: "当前项目",
            readOnly: false,
          },
          {
            id: "project-b",
            name: "目标项目",
            readOnly: false,
          },
        ] as any,
      },
      global: { plugins: [ElementPlus] },
    });

    expect(wrapper.find(".thread-actions").exists()).toBe(false);
    expect(wrapper.find(".thread-action").exists()).toBe(false);

    const event = new MouseEvent("contextmenu", {
      bubbles: true,
      cancelable: true,
      clientX: 160,
      clientY: 96,
    });
    wrapper.get(".thread-main").element.dispatchEvent(event);
    await nextTick();

    expect(event.defaultPrevented).toBe(true);
    expect(contextMenuState.visible).toBe(true);
    expect(contextMenuState.title).toBe("项目对话");
    expect(contextMenuState.items.map((item) => item.label)).toEqual([
      "打开对话",
      "重命名",
      "置顶",
      "标为已读",
      "归档",
      "移动到项目",
      "删除对话",
    ]);
    const moveTargets = contextMenuState.items.find((item) => item.id === "move-conversation")?.children || [];
    expect(moveTargets.map((item) => item.label)).toEqual([
      "移回聊天",
      "移入项目：当前项目",
      "移入项目：目标项目",
    ]);
    expect(moveTargets[1].disabled).toBe(true);

    contextMenuState.items.find((item) => item.id === "open-conversation")?.action?.();
    contextMenuState.items.find((item) => item.id === "pin-conversation")?.action?.();
    contextMenuState.items.find((item) => item.id === "mark-conversation-read")?.action?.();
    contextMenuState.items.find((item) => item.id === "archive-conversation")?.action?.();
    contextMenuState.items.find((item) => item.id === "delete-conversation")?.action?.();
    moveTargets[2].action?.();

    expect(wrapper.emitted("open")?.[0]).toEqual([conversation]);
    expect(wrapper.emitted("togglePin")?.[0]).toEqual([conversation]);
    expect(wrapper.emitted("markRead")?.[0]).toEqual([conversation]);
    expect(wrapper.emitted("archive")?.[0]).toEqual([conversation]);
    expect(wrapper.emitted("delete")?.[0]).toEqual([conversation]);
    expect(wrapper.emitted("move")?.[0]).toEqual([conversation, "project-b"]);

    wrapper.unmount();
  });
});
