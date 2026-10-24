import { nextTick, reactive } from "vue";
import { mount } from "@vue/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";

const route = reactive({
  path: "/chat",
  query: {} as Record<string, string>,
});

vi.mock("vue-router", () => ({ useRoute: () => route }));
vi.mock("element-plus", () => ({ ElMessage: { error: vi.fn() } }));
vi.mock("@/runtime/owned-chat-management", () => ({ ownedChatRuntimeRequest: vi.fn() }));

import AgentApprovalGuard from "../components/AgentApprovalGuard.vue";

const stubs = {
  ElDialog: {
    props: ["modelValue"],
    template: '<div data-test="approval-dialog" :data-visible="String(modelValue)"><slot name="header" /><slot /><slot name="footer" /></div>',
  },
  ElAlert: true,
  ElTag: true,
  ElButton: true,
};

describe("agent approval guard", () => {
  beforeEach(() => {
    route.path = "/chat";
    route.query = {};
  });

  it("shows an approval for a newly started chat before the route receives its conversation id", async () => {
    const wrapper = mount(AgentApprovalGuard, { global: { stubs } });
    window.dispatchEvent(new CustomEvent("amitia:agent-approval", {
      detail: {
        action: "requested",
        id: "approval-1",
        conversationId: "conversation-1",
        turnId: "turn-1",
        toolName: "read_environment_variable",
        riskLevel: "high",
        arguments: "{}",
      },
    }));
    await nextTick();
    expect(wrapper.get('[data-test="approval-dialog"]').attributes("data-visible")).toBe("true");
    wrapper.unmount();
  });

  it("only shows approvals for the conversation bound by the route", async () => {
    route.query = { conversationId: "conversation-2" };
    const wrapper = mount(AgentApprovalGuard, { global: { stubs } });
    for (const conversationId of ["conversation-1", "conversation-2"]) {
      window.dispatchEvent(new CustomEvent("amitia:agent-approval", {
        detail: {
          action: "requested",
          id: `approval-${conversationId}`,
          conversationId,
          turnId: "turn-1",
          toolName: "workspace.write",
          arguments: "{}",
        },
      }));
    }
    await nextTick();
    expect(wrapper.get('[data-test="approval-dialog"]').attributes("data-visible")).toBe("true");
    expect(wrapper.text()).toContain("workspace.write");
    expect(wrapper.text()).not.toContain("read_environment_variable");
    wrapper.unmount();
  });
});
