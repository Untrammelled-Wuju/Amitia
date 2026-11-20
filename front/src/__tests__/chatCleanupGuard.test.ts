import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  request: vi.fn(),
  warning: vi.fn(),
  error: vi.fn(),
  success: vi.fn(),
}));

vi.mock("@/runtime/owned-chat-management", () => ({ ownedManagementRequest: mocks.request }));
vi.mock("element-plus", () => ({ ElMessage: { warning: mocks.warning, error: mocks.error, success: mocks.success } }));

import { useChatCleanup } from "@/views/chat-cleanup/composables/useChatCleanup";

describe("聊天清理条件守卫", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.request.mockResolvedValue({ conversationCount: 0, messageCount: 0 });
  });

  it("没有条件时拒绝预览并给出明确提示", async () => {
    const cleanup = useChatCleanup();
    expect(cleanup.hasCleanupCondition()).toBe(false);
    await cleanup.previewCleanup();
    expect(mocks.request).not.toHaveBeenCalled();
    expect(mocks.warning).toHaveBeenCalledWith("请至少选择一个清理条件：日期、天数、渠道或来源");
    expect(cleanup.previewResult.value).toBeNull();
  });

  it("选择任一条件后放行并按条件提交", async () => {
    const cleanup = useChatCleanup();
    cleanup.form.olderThanDays = 90;
    expect(cleanup.hasCleanupCondition()).toBe(true);
    await cleanup.previewCleanup();
    expect(mocks.warning).not.toHaveBeenCalled();
    expect(mocks.request).toHaveBeenCalledWith("cleanup/preview", { olderThanDays: 90, includeMemories: false }, "post");
  });

  it("渠道与来源选择同样构成有效条件", () => {
    const cleanup = useChatCleanup();
    cleanup.form.channels = ["web"];
    expect(cleanup.hasCleanupCondition()).toBe(true);
    cleanup.form.channels = [];
    cleanup.form.sources = ["manual"];
    expect(cleanup.hasCleanupCondition()).toBe(true);
    cleanup.form.sources = [];
    cleanup.form.beforeDate = "2026-01-01";
    expect(cleanup.hasCleanupCondition()).toBe(true);
  });
});
