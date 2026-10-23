import { beforeEach, describe, expect, it, vi } from "vitest";

const managed = vi.hoisted(() => vi.fn());

vi.mock("@/runtime/owned-chat-management", () => ({
  ownedManagementRequest: managed,
}));

import {
  createMessageFeedback,
  deleteMessageFeedback,
  loadMessageFeedback,
} from "@/runtime/message-feedback";

describe("消息反馈 Owner 接入", () => {
  beforeEach(() => managed.mockReset());

  it("按角色读取最新反馈", async () => {
    managed.mockResolvedValue({ items: [{ id: 2, messageId: "reply", feedbackType: "good" }] });
    const item = await loadMessageFeedback("reply", "role");
    expect(item?.id).toBe(2);
    expect(managed).toHaveBeenCalledWith("feedback/list", { messageId: "reply" }, "post", "role");
  });

  it("提交反馈并校验确认结果", async () => {
    managed.mockResolvedValue({ feedback: { id: 3, messageId: "reply", feedbackType: "other", reason: "detail" } });
    const item = await createMessageFeedback("reply", "role", "other", "detail");
    expect(item.id).toBe(3);
    expect(managed).toHaveBeenCalledWith("feedback/create", { messageId: "reply", feedbackType: "other", reason: "detail" }, "post", "role");
  });

  it("删除反馈要求 revision 确认", async () => {
    managed.mockResolvedValue({ deleted: true, revision: 2 });
    await deleteMessageFeedback(3, "role");
    expect(managed).toHaveBeenCalledWith("feedback/delete", { feedbackId: 3 }, "post", "role");
  });
});
