import { describe, expect, it } from "vitest";
import {
  compareChatMessages,
  getMessageUIKey,
  mergeMessageCollections,
  parseMessageTime,
  shouldShowAssistantIdentity,
  shouldShowRoleSwitch,
} from "@/utils/message-order";

describe("parseMessageTime", () => {
  it("解析后端时间与 ISO 时间", () => {
    expect(parseMessageTime("2026-09-10 15:34:55")).toBe(
      new Date("2026-09-10T15:34:55").getTime(),
    );
    expect(parseMessageTime("2026-09-10T07:34:55.850Z")).toBe(
      Date.parse("2026-09-10T07:34:55.850Z"),
    );
  });

  it("非法值返回 0", () => {
    expect(parseMessageTime("")).toBe(0);
    expect(parseMessageTime("not-a-date")).toBe(0);
  });
});

describe("助手身份与角色切换", () => {
  it("同一角色的连续持久消息不重复身份", () => {
    expect(
      shouldShowAssistantIdentity(
        { id: "a2", role: "assistant", characterId: "character-1" },
        { id: "a1", role: "assistant", characterId: "character-1" },
      ),
    ).toBe(false);
  });

  it("不同角色重新显示身份与角色切换", () => {
    const previous = { id: "a1", role: "assistant", characterId: "character-1" };
    const current = { id: "a2", role: "assistant", characterId: "character-2" };
    expect(shouldShowAssistantIdentity(current, previous)).toBe(true);
    expect(shouldShowRoleSwitch(previous, current)).toBe(true);
  });

  it("缺少发送者标识时不通过请求标识猜关系", () => {
    expect(
      shouldShowAssistantIdentity(
        { id: "a2", role: "assistant", requestId: "request-1" },
        { id: "a1", role: "assistant", requestId: "request-1" },
      ),
    ).toBe(true);
  });
});

describe("消息顺序", () => {
  it("优先使用持久 messageSequence", () => {
    const messages = [
      { id: "m3", sequence: 3, createdAt: "2026-09-21 10:00:00" },
      { id: "m1", sequence: 1, createdAt: "2026-09-21 10:00:02" },
      { id: "m2", sequence: 2, createdAt: "2026-09-21 10:00:01" },
    ].sort(compareChatMessages);
    expect(messages.map((message) => message.id)).toEqual(["m1", "m2", "m3"]);
  });

  it("没有 sequence 时才使用时间", () => {
    const messages = [
      { id: "m2", createdAt: "2026-09-21T10:00:02+08:00" },
      { id: "m1", createdAt: "2026-09-21T10:00:01+08:00" },
    ].sort(compareChatMessages);
    expect(messages.map((message) => message.id)).toEqual(["m1", "m2"]);
  });

  it("显式锚点仍保持附件/错误等附属项位于目标消息之后", () => {
    const messages = [
      { id: "attached", anchorMessageId: "user-1", createdAt: "2026-09-21 09:59:00" },
      { id: "user-1", createdAt: "2026-09-21 10:00:00" },
    ].sort(compareChatMessages);
    expect(messages.map((message) => message.id)).toEqual(["user-1", "attached"]);
  });
});

describe("历史分页合并", () => {
  it("按服务器 messageId 去重并使用新快照字段覆盖旧字段", () => {
    const merged = mergeMessageCollections(
      [
        { id: "m2", sequence: 2, content: "old", reasoningContent: "r" },
        { id: "m3", sequence: 3, content: "three" },
      ],
      [
        { id: "m1", sequence: 1, content: "one" },
        { id: "m2", sequence: 2, content: "new" },
      ],
    );
    expect(merged.map((message) => message.id)).toEqual(["m1", "m2", "m3"]);
    expect(merged.find((message) => message.id === "m2")).toMatchObject({
      content: "new",
      reasoningContent: "r",
    });
  });

  it("不同客户端用户消息不会因为 requestId 以外的时间字段被误合并", () => {
    const first = { role: "user", clientMessageId: "req-1", createdAt: "2026-09-21 10:00:00" };
    const second = { role: "user", clientMessageId: "req-2", createdAt: "2026-09-21 10:00:00" };
    const merged = mergeMessageCollections([], [first, second]);
    expect(merged).toHaveLength(2);
    expect(getMessageUIKey(merged[0])).not.toBe(getMessageUIKey(merged[1]));
  });
});
