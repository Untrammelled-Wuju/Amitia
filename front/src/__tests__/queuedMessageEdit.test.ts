import { mount } from "@vue/test-utils";
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { defineComponent, ref } from "vue";

const request = vi.hoisted(() => vi.fn());
const scopedRequest = vi.hoisted(() => vi.fn());

vi.mock("@/runtime/owned-chat-management", () => ({
  ownedChatRuntimeRequest: request,
  ownedChatRuntimeScopedRequest: scopedRequest,
}));

import { useConversationQueue } from "@/composables/useConversationQueue";
import { useWorkbenchStore } from "@/stores/workbench";
import { createPinia, setActivePinia } from "pinia";

const scope = { spaceId: "space", coreId: "core", targetDeviceId: "device", resourceOwnerId: "device", roleId: "role", roleOwnerId: "device", initiatorDeviceId: "device" };

function mountQueue() {
  const holder: { queue?: ReturnType<typeof useConversationQueue> } = {};
  const Host = defineComponent({
    setup() {
      holder.queue = useConversationQueue(ref("conv"), ref("role"), ref("turn"));
      return () => null;
    },
  });
  const wrapper = mount(Host);
  return { queue: holder.queue!, wrapper };
}

describe("排队消息原位更新", () => {
  beforeAll(() => {
    setActivePinia(createPinia());
  });

  beforeEach(() => {
    request.mockReset();
    scopedRequest.mockReset();
    request.mockResolvedValue({ items: [], executionScope: scope });
  });

  it("编辑后走 queue/update 并携带请求编号、正文与附件", async () => {
    const { queue, wrapper } = mountQueue();
    await queue.update("req-1", "修订后的正文", [{ artifactId: "art", ownerDeviceId: "device", storageDeviceId: "device", kind: "file", name: "说明.pdf", mimeType: "application/pdf", sha256: "abc", sizeBytes: 3 }]);
    expect(request).toHaveBeenCalledWith("queue/update", {
      conversationId: "conv",
      queuedRequestId: "req-1",
      message: "修订后的正文",
      attachments: [{ artifactId: "art", ownerDeviceId: "device", storageDeviceId: "device", kind: "file", name: "说明.pdf", mimeType: "application/pdf", sha256: "abc", sizeBytes: 3 }],
    }, "role");
    expect(request.mock.calls.some((call) => call[0] === "queue/list")).toBe(true);
    wrapper.unmount();
  });
});

describe("侧边工作区旁路对话", () => {
  it("按排队消息新开 chat 标签并记住种子文本", () => {
    const pinia = createPinia();
    setActivePinia(pinia);
    const store = useWorkbenchStore();
    store.setConversation("main-conv", "role-1");
    const tab = store.openSideChat({ queueRequestId: "req-1", message: "第一条待处理的消息内容", attachments: [], sourceConversationId: "main-conv", roleId: "role-1" });
    expect(tab.kind).toBe("chat");
    expect(tab.title).toBe("第一条待处理的消息内容");
    expect(store.tabs.map((item) => item.id)).toContain(tab.id);
    expect(store.sideChatSeed(tab.id)?.message).toBe("第一条待处理的消息内容");
    expect(store.sideChatSeed(tab.id)?.roleId).toBe("role-1");

    const again = store.openSideChat({ queueRequestId: "req-1", message: "第一条待处理的消息内容", attachments: [], sourceConversationId: "main-conv", roleId: "role-1" });
    expect(again.id).toBe(tab.id);

    store.rememberSideChatConversation(tab.id, "side-conv");
    expect(store.sideChatSeed(tab.id)?.conversationId).toBe("side-conv");
  });
});
