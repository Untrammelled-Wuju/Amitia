import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";
import WorkbenchAgentRunsView from "../components/workbench/WorkbenchAgentRunsView.vue";
import { useWorkbenchStore } from "../stores/workbench";

const mocks = vi.hoisted(() => ({
  listRuns: vi.fn(),
  listMessages: vi.fn(),
  send: vi.fn(),
}));

vi.mock("../api/agent-runs", () => ({
  listAgentRuns: mocks.listRuns,
  listAgentRunMessages: mocks.listMessages,
  sendAgentRunMessage: mocks.send,
}));

const root = {
  id: "root", spaceId: "space", conversationId: "conversation", parentRunId: "",
  coordinationId: "coord", assignmentId: "", parentInteractionId: "parent", childInteractionId: "",
  characterId: "role", workerRole: "main", objective: "主对话智能体", status: "running", depth: 0,
  createdAt: "2026-11-03T00:00:00Z", updatedAt: "2026-11-03T00:00:00Z",
};
const child = {
  ...root, id: "child", parentRunId: "root", assignmentId: "assignment", childInteractionId: "interaction",
  workerRole: "research_worker", objective: "检查接口", status: "waiting", depth: 1,
};

describe("子智能体工作区", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.listRuns.mockResolvedValue([child, root]);
    mocks.listMessages.mockResolvedValue([{
      id: "message", spaceId: "space", conversationId: "conversation", senderRunId: "child",
      recipientRunId: "root", kind: "result", content: "接口检查完成", status: "pending",
      correlationId: "coord", createdAt: "2026-11-03T00:01:00Z",
    }]);
    mocks.send.mockResolvedValue({
      id: "sent", spaceId: "space", conversationId: "conversation", senderRunId: "user:child",
      recipientRunId: "child", kind: "user", content: "继续", status: "pending",
      correlationId: "", createdAt: "2026-11-03T00:02:00Z",
    });
  });

  it("展示运行树、双向消息并支持发送人工消息", async () => {
    const pinia = createPinia();
    setActivePinia(pinia);
    const store = useWorkbenchStore(pinia);
    store.setConversation("conversation", "role");
    const wrapper = mount(WorkbenchAgentRunsView, { global: { plugins: [pinia] } });
    try {
      await flushPromises();
      expect(wrapper.text()).toContain("主对话智能体");
      expect(wrapper.text()).toContain("检查接口");
      await wrapper.findAll("nav button")[1].trigger("click");
      await flushPromises();
      expect(wrapper.text()).toContain("接口检查完成");
      await wrapper.get("textarea").setValue("继续");
      await wrapper.get("form").trigger("submit");
      await flushPromises();
      expect(mocks.send).toHaveBeenCalledWith("child", "继续");
    } finally {
      wrapper.unmount();
    }
  });
});
