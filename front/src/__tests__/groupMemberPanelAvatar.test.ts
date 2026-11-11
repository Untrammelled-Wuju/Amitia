import { flushPromises, mount } from "@vue/test-utils";
import ElementPlus, { ElMessage } from "element-plus";
import { afterEach, describe, expect, it, vi } from "vitest";
import GroupMemberPanel from "../components/GroupMemberPanel.vue";

vi.mock("../composables/useDeviceOwnedConversation", () => ({
  useDeviceOwnedConversation: () => ({
    inviteGroupMembers: vi.fn(),
    removeGroupMember: vi.fn(),
    updateGroupSettings: vi.fn(),
    requestGroupReplies: vi.fn(),
    failedGroupDeliveries: vi.fn().mockResolvedValue([]),
    retryGroupDelivery: vi.fn(),
  }),
}));

afterEach(() => {
  document.body.innerHTML = "";
});

describe("GroupMemberPanel avatar", () => {
  it("未选角色或超过人数上限时提示且不提交", async () => {
    const warning = vi.spyOn(ElMessage, "warning").mockImplementation(() => undefined as any);
    const wrapper = mount(GroupMemberPanel, {
      attachTo: document.body,
      props: {
        visible: true,
        group: null,
        characters: Array.from({ length: 8 }, (_, index) => ({ id: `role-${index}`, name: `角色 ${index}` })),
        roleId: "current",
        conversationId: "conversation",
      },
      global: { plugins: [ElementPlus] },
    });
    try {
      await flushPromises();
      const control = document.body.querySelector<HTMLElement>(".invite-control");
      expect(control?.getAttribute("role")).toBe("button");
      control?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
      expect(warning).toHaveBeenCalledWith("请先选择要邀请的角色。");
      const state = (wrapper.vm as any).$.setupState;
      for (let index = 0; index < 8; index++) state.toggleSelection(`role-${index}`, true);
      await wrapper.vm.$nextTick();
      control?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
      expect(warning).toHaveBeenCalledWith("群聊最多支持 8 位角色，请减少选择数量。");
      expect(wrapper.emitted("changed")).toBeUndefined();
    } finally { wrapper.unmount(); warning.mockRestore(); }
  });
  it("单聊邀请列表排除当前角色并保持头像比例", async () => {
    const wrapper = mount(GroupMemberPanel, {
      attachTo: document.body,
      props: {
        visible: true,
        group: null,
        characters: [
          { id: "role-a", name: "当前角色", avatar: "/current.png" },
          { id: "role-b", name: "小林", avatar: "/wide-avatar.png" },
        ],
        roleId: "role-a",
        conversationId: "conversation",
      },
      global: { plugins: [ElementPlus] },
    });
    await flushPromises();
    const rows = document.body.querySelectorAll(".invite-row");
    const avatar = rows.item(0)?.querySelector<HTMLElement>(".el-avatar");
    const image = avatar?.querySelector("img");
    expect(rows).toHaveLength(1);
    expect(rows.item(0)?.textContent).toContain("小林");
    expect(rows.item(0)?.textContent).not.toContain("当前角色");
    expect(avatar?.style.getPropertyValue("--el-avatar-size")).toBe("30px");
    expect(image?.style.objectFit).toBe("cover");
    wrapper.unmount();
  });
});
