import { flushPromises, mount } from "@vue/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";

const route = vi.hoisted(() => ({
  path: "/chat",
  fullPath: "/chat",
  query: {} as Record<string, string>,
}));
const push = vi.hoisted(() => vi.fn());
const replace = vi.hoisted(() => vi.fn());
const get = vi.hoisted(() => vi.fn());

vi.mock("vue-router", () => ({
  useRoute: () => route,
  useRouter: () => ({ push, replace }),
}));
vi.mock("element-plus", () => ({
  ElMessage: { error: vi.fn(), success: vi.fn() },
}));
vi.mock("@/composables/useApi", () => ({ apiClient: { get } }));
vi.mock("@/runtime/owned-chat-management", () => ({ ownedChatRuntimeRequest: vi.fn() }));
vi.mock("@/views/kernel/tasks/api", () => ({ decideSourceTaskApproval: vi.fn() }));

import AgentApprovalGuard from "../components/AgentApprovalGuard.vue";

const stubs = {
  ElDialog: {
    props: ["modelValue"],
    template: '<div data-test="approval-dialog" :data-visible="String(modelValue)"><slot name="header" /><slot /><slot name="footer" /></div>',
  },
  ElAlert: { template: "<div><slot /></div>" },
  ElTag: { template: "<span><slot /></span>" },
  ElButton: { template: "<button><slot /></button>" },
  ElTimeline: { template: "<div><slot /></div>" },
  ElTimelineItem: { template: "<div><slot /></div>" },
};

describe("attention action guard", () => {
  beforeEach(() => {
    route.path = "/chat";
    route.fullPath = "/chat";
    route.query = {};
    get.mockReset();
    push.mockReset();
    replace.mockReset();
  });

  it("does not show an attention dialog outside a conversation detail page", async () => {
    get.mockResolvedValue({
      data: [
        {
          id: "approval:one",
          kind: "agent_approval",
          priority: 100,
          requiresAction: true,
          conversationId: "conversation-1",
          title: "工具执行需要批准",
          summary: "workspace.write",
          reason: "等待确认",
          createdAt: "2026-11-02T10:00:00Z",
        },
      ],
    });
    const wrapper = mount(AgentApprovalGuard, { global: { stubs } });
    await flushPromises();
    expect(wrapper.get('[data-test="approval-dialog"]').attributes("data-visible")).toBe("false");
    wrapper.unmount();
  });

  it("only shows actionable items for the conversation bound by the route", async () => {
    route.query = { conversationId: "conversation-2" };
    route.fullPath = "/chat?conversationId=conversation-2";
    get.mockResolvedValue({
      data: [
        {
          id: "approval:one",
          kind: "agent_approval",
          priority: 100,
          requiresAction: true,
          conversationId: "conversation-1",
          title: "错误会话",
          summary: "read",
          reason: "等待确认",
          createdAt: "2026-11-02T10:00:00Z",
        },
        {
          id: "approval:two",
          kind: "agent_approval",
          priority: 100,
          requiresAction: true,
          conversationId: "conversation-2",
          title: "工具执行需要批准",
          summary: "workspace.write",
          reason: "等待确认",
          createdAt: "2026-11-02T10:01:00Z",
          payload: { arguments: "{}", riskLevel: "high" },
        },
      ],
    });
    const wrapper = mount(AgentApprovalGuard, { global: { stubs } });
    await flushPromises();
    expect(wrapper.get('[data-test="approval-dialog"]').attributes("data-visible")).toBe("true");
    expect(wrapper.text()).toContain("workspace.write");
    expect(wrapper.text()).not.toContain("错误会话");
    wrapper.unmount();
  });

  it("prioritizes the attention item carried by a notification deep link", async () => {
    route.query = { conversationId: "conversation-2", attentionId: "approval:linked" };
    route.fullPath = "/chat?conversationId=conversation-2&attentionId=approval%3Alinked";
    get.mockResolvedValue({
      data: [
        {
          id: "approval:high",
          kind: "task_failed",
          priority: 100,
          requiresAction: true,
          conversationId: "conversation-2",
          title: "更高优先级",
          summary: "other",
          reason: "等待恢复",
          createdAt: "2026-11-02T10:00:00Z",
        },
        {
          id: "approval:linked",
          kind: "agent_approval",
          priority: 90,
          requiresAction: true,
          conversationId: "conversation-2",
          title: "通知指向事项",
          summary: "workspace.patch",
          reason: "等待确认",
          createdAt: "2026-11-02T10:01:00Z",
          payload: { arguments: "{}", riskLevel: "medium" },
        },
      ],
    });
    const wrapper = mount(AgentApprovalGuard, { global: { stubs } });
    await flushPromises();
    expect(wrapper.text()).toContain("通知指向事项");
    wrapper.unmount();
  });

  it("clears the notification deep link when the user defers an item", async () => {
    route.query = { conversationId: "conversation-2", attentionId: "approval:linked", characterId: "character-1" };
    route.fullPath = "/chat?conversationId=conversation-2&attentionId=approval%3Alinked&characterId=character-1";
    get.mockResolvedValue({
      data: [
        {
          id: "approval:linked",
          kind: "agent_approval",
          priority: 100,
          requiresAction: true,
          conversationId: "conversation-2",
          title: "通知指向事项",
          summary: "workspace.patch",
          reason: "等待确认",
          createdAt: "2026-11-02T10:01:00Z",
          payload: { arguments: "{}", riskLevel: "medium" },
        },
      ],
    });
    const wrapper = mount(AgentApprovalGuard, { global: { stubs } });
    await flushPromises();
    await wrapper.findAll("button")[0].trigger("click");
    expect(replace).toHaveBeenCalledWith({
      path: "/chat",
      query: { conversationId: "conversation-2", characterId: "character-1" },
    });
    wrapper.unmount();
  });
});
