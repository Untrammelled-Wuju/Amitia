import { describe, it, expect } from "vitest";
import {
  compareChatMessages,
  parseMessageTime,
  mergeChatMessage,
} from "@/utils/message-order";
import { compareTimeline } from "@/ui-runtime/conversationProjection";

function renderOrder(messages: any[]): string[] {
  return messages
    .map((message, index) => ({
      key: `message:${message.id}`,
      message,
      sequence: message?.seq ?? message?.sequence ?? undefined,
      timestamp: String(message?.createdAt ?? message?.timestamp ?? ""),
    }))
    .sort((a, b) => {
      const aId = String(a.message?.id || "");
      const bId = String(b.message?.id || "");
      const aAnchor = String(a.message?.anchorMessageId || "");
      const bAnchor = String(b.message?.anchorMessageId || "");
      if (aAnchor && aAnchor === bId) return 1;
      if (bAnchor && bAnchor === aId) return -1;
      const order = compareTimeline(a.sequence, a.timestamp, b.sequence, b.timestamp);
      if (order !== 0) return order;
      return a.key.localeCompare(b.key);
    })
    .map((item) => item.message.id);
}

describe("parseMessageTime", () => {
  it("解析后端秒级本地时间字符串", () => {
    expect(parseMessageTime("2026-09-10 15:34:55")).toBe(
      new Date("2026-09-10T15:34:55").getTime(),
    );
  });

  it("解析毫秒级 ISO 字符串", () => {
    expect(parseMessageTime("2026-09-10T07:34:55.850Z")).toBe(
      Date.parse("2026-09-10T07:34:55.850Z"),
    );
  });

  it("解析数字与数字字符串", () => {
    const now = 1780000000000;
    expect(parseMessageTime(now)).toBe(now);
    expect(parseMessageTime(String(now))).toBe(now);
  });

  it("空值与非法值返回 0", () => {
    expect(parseMessageTime("")).toBe(0);
    expect(parseMessageTime(null)).toBe(0);
    expect(parseMessageTime(undefined)).toBe(0);
    expect(parseMessageTime("not-a-date")).toBe(0);
  });
});

describe("compareChatMessages 同秒平局", () => {
  it("历史消息(带sequence)排在同秒的实时消息之前", () => {
    const mergedUser = { id: "u1", sequence: 5, createdAt: "2026-09-10 15:34:55" };
    const liveReply = { id: "a1", createdAt: "2026-09-10 15:34:55" };
    expect([liveReply, mergedUser].sort(compareChatMessages).map((m) => m.id)).toEqual([
      "u1",
      "a1",
    ]);
  });
});

describe("聊天时间线渲染顺序", () => {
  it("历史合并后继续实时收发,顺序保持正确", () => {
    const history = [
      { id: "h1", sequence: 1, createdAt: "2026-09-10 15:00:00" },
      { id: "h2", sequence: 2, createdAt: "2026-09-10 15:01:00" },
    ];
    const live = [
      { id: "u1", createdAt: "2026-09-10T07:30:00.500Z" },
      { id: "a1", createdAt: "2026-09-10 15:31:00" },
      { id: "u2", createdAt: "2026-09-10T07:32:10.200Z" },
      { id: "a2", createdAt: "2026-09-10 15:32:11" },
    ];
    expect(renderOrder([...history, ...live])).toEqual([
      "h1",
      "h2",
      "u1",
      "a1",
      "u2",
      "a2",
    ]);
  });

  it("同一秒内回复消息按锚点排在用户消息之后", () => {
    const user = { id: "u1", createdAt: "2026-09-10T07:34:55.000Z" };
    const reply = {
      id: "a1",
      sequence: 6,
      createdAt: "2026-09-10 15:34:55",
      anchorMessageId: "u1",
    };
    expect([user, reply].sort(compareChatMessages).map((m) => m.id)).toEqual([
      "u1",
      "a1",
    ]);
    expect([reply, user].sort(compareChatMessages).map((m) => m.id)).toEqual([
      "u1",
      "a1",
    ]);
    expect(renderOrder([user, reply])).toEqual(["u1", "a1"]);
    expect(renderOrder([reply, user])).toEqual(["u1", "a1"]);
  });

  it("毫秒精度乐观消息与秒精度回复不再因亚秒差倒挂", () => {
    const user = { id: "u1", createdAt: "2026-09-10T07:34:55.850Z" };
    const reply = {
      id: "a1",
      sequence: 6,
      createdAt: "2026-09-10 15:34:55",
      anchorMessageId: "u1",
    };
    expect(renderOrder([user, reply])).toEqual(["u1", "a1"]);
    expect(renderOrder([reply, user])).toEqual(["u1", "a1"]);
  });

  it("SSE事件带sequence时按sequence排序", () => {
    const messages = [
      { id: "m1", sequence: 1, createdAt: "2026-09-10 15:00:00" },
      { id: "m2", sequence: 2, createdAt: "2026-09-10 15:01:00" },
      { id: "m3", sequence: 3, createdAt: "2026-09-10 15:02:00" },
    ];
    expect(renderOrder([...messages].sort(compareChatMessages))).toEqual([
      "m1",
      "m2",
      "m3",
    ]);
  });

  it("合并事件消息不会丢失已有sequence", () => {
    const messages: any[] = [
      { id: "m1", sequence: 7, createdAt: "2026-09-10 15:00:00", content: "old" },
    ];
    const incoming = {
      id: "m1",
      content: "new",
      createdAt: "2026-09-10 15:00:00",
      status: "sent",
    };
    expect(mergeChatMessage(messages, incoming)).toBe(true);
    expect(messages[0]!.sequence).toBe(7);
    expect(messages[0]!.content).toBe("new");
  });
});
