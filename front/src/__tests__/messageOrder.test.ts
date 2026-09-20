import { describe, it, expect } from "vitest";
import {
  compareChatMessages,
  hasAssistantReplyAfterLatestUser,
  getMessageUIKey,
  shouldShowRoleSwitch,
  shouldShowAssistantIdentity,
  parseMessageTime,
  mergeChatMessage,
  upsertStreamingAssistantMessage,
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

describe("AI 消息发送者连续段", () => {
  it("同一 AI 连续消息只在第一条显示身份", () => {
    const previous = {
      id: "assistant-1",
      role: "assistant",
      characterId: "character-1",
      responseGroupId: "request-1",
    };
    const current = {
      id: "assistant-2",
      role: "assistant",
      characterId: "character-1",
      responseGroupId: "request-1",
    };

    expect(shouldShowAssistantIdentity(current, previous)).toBe(false);
  });

  it("不同 AI 连续发言时重新显示身份", () => {
    const previous = {
      id: "assistant-1",
      role: "assistant",
      characterId: "character-1",
      responseGroupId: "request-1",
    };
    const current = {
      id: "assistant-2",
      role: "assistant",
      characterId: "character-2",
      responseGroupId: "request-2",
    };

    expect(shouldShowAssistantIdentity(current, previous)).toBe(true);
  });

  it("同一 AI 连续回复按发送顺序合并为一个连续段", () => {
    const previous = {
      id: "assistant-1",
      role: "assistant",
      characterId: "character-1",
      responseGroupId: "request-1",
    };
    const current = {
      id: "assistant-2",
      role: "assistant",
      characterId: "character-1",
      responseGroupId: "request-2",
    };

    expect(shouldShowAssistantIdentity(current, previous)).toBe(false);
  });

  it("用户消息、系统消息或首条消息重新显示身份", () => {
    const current = {
      id: "assistant-1",
      role: "assistant",
      characterId: "character-1",
      responseGroupId: "request-1",
    };

    expect(shouldShowAssistantIdentity(current, null)).toBe(true);
    expect(
      shouldShowAssistantIdentity(current, {
        id: "user-1",
        role: "user",
      }),
    ).toBe(true);
    expect(
      shouldShowAssistantIdentity(current, {
        id: "system-1",
        role: "system",
      }),
    ).toBe(true);
  });

  it("发送者标识缺失时使用回复分组判断连续性", () => {
    const previous = {
      id: "assistant-1",
      role: "assistant",
      responseGroupId: "request-1",
    };
    const continuation = {
      id: "assistant-2",
      role: "assistant",
      responseGroupId: "request-1",
    };
    const nextReply = {
      id: "assistant-3",
      role: "assistant",
      responseGroupId: "request-2",
    };

    expect(shouldShowAssistantIdentity(continuation, previous)).toBe(false);
    expect(shouldShowAssistantIdentity(nextReply, previous)).toBe(true);
  });
});

describe("对话角色切换", () => {
  it("相邻消息角色变化时显示切换分隔", () => {
    const messageA = {
      id: "a1",
      role: "assistant",
      characterId: "character-a",
    };
    const messageB = {
      id: "b1",
      role: "user",
      characterId: "character-b",
    };
    const messageAAgain = {
      id: "a2",
      role: "assistant",
      characterId: "character-a",
    };

    expect(shouldShowRoleSwitch(messageA, messageB)).toBe(true);
    expect(shouldShowRoleSwitch(messageB, messageAAgain)).toBe(true);
  });

  it("同一角色连续消息不重复显示切换分隔", () => {
    expect(
      shouldShowRoleSwitch(
        { id: "a1", role: "assistant", characterId: "character-a" },
        { id: "a2", role: "assistant", characterId: "character-a" },
      ),
    ).toBe(false);
  });

  it("缺少角色标识时不显示切换分隔", () => {
    expect(
      shouldShowRoleSwitch(
        { id: "a1", role: "assistant", characterId: "" },
        { id: "b1", role: "assistant", characterId: "character-b" },
      ),
    ).toBe(false);
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

describe("hasAssistantReplyAfterLatestUser", () => {
  it("忽略上一轮回复并识别当前轮是否已有回复", () => {
    const messages: any[] = [
      { id: "u1", role: "user", content: "第一轮", sequence: 1 },
      { id: "a1", role: "assistant", content: "第一轮回复", sequence: 2 },
      { id: "u2", role: "user", content: "第二轮", sequence: 3 },
    ];
    expect(hasAssistantReplyAfterLatestUser(messages)).toBe(false);
    messages.push({
      id: "a2",
      role: "assistant",
      content: "第二轮回复",
      status: "streaming",
      sequence: 4,
    });
    expect(hasAssistantReplyAfterLatestUser(messages)).toBe(true);
  });

  it("没有用户消息时不显示生成占位", () => {
    expect(
      hasAssistantReplyAfterLatestUser([
        { id: "a1", role: "assistant", content: "历史回复" },
      ]),
    ).toBe(false);
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

  it("流式助手消息按服务端ID原地更新而不是重复插入", () => {
    const messages: any[] = [
      {
        id: "assistant-1",
        role: "assistant",
        content: "你好",
        status: "sent",
      },
    ];

    const index = upsertStreamingAssistantMessage(
      messages,
      {
        id: "assistant-1",
        role: "assistant",
        content: "你好",
        status: "streaming",
      },
      null,
    );

    expect(index).toBe(0);
    expect(messages).toHaveLength(1);
    expect(messages[0]!.content).toBe("你好");
    expect(messages[0]!.status).toBe("streaming");
  });

  it("生成占位消息合并服务端响应后保持稳定uiKey", () => {
    const messages: any[] = [
      {
        id: "generating:request-1",
        requestId: "request-1",
        uiKey: "generation:request-1",
        role: "assistant",
        content: "",
        status: "streaming",
        generationPending: true,
      },
    ];

    expect(
      mergeChatMessage(messages, {
        id: "assistant-1",
        requestId: "request-1",
        role: "assistant",
        content: "回复",
        createdAt: "2026-09-20 10:00:00",
      }),
    ).toBe(true);

    expect(messages).toHaveLength(1);
    expect(messages[0]!.id).toBe("assistant-1");
    expect(messages[0]!.uiKey).toBe("generation:request-1");
    expect(messages[0]!.generationPending).toBe(false);
    expect(messages[0]!.status).toBe("sent");
    expect(messages[0]!.reasoningContent).toBeUndefined();
    expect(getMessageUIKey(messages[0])).toBe("generation:request-1");
  });
});
