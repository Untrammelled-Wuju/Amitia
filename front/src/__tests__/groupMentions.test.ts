import { describe, expect, it } from "vitest";
import { resolveGroupMentions } from "@/composables/groupMentions";
import type { OwnedGroupMember } from "@/runtime/device-owned-chat";

const members = [
  {
    memberId: "a",
    characterId: "a",
    roleOwnerId: "owner",
    characterName: "大肥鱼",
    status: "active",
  },
  {
    memberId: "b",
    characterId: "b",
    roleOwnerId: "owner",
    characterName: "大肥鱼 (副本)",
    status: "active",
  },
  {
    memberId: "left",
    characterId: "left",
    roleOwnerId: "owner",
    characterName: "已离开",
    status: "left",
  },
] as OwnedGroupMember[];
const mentions = [
  { memberId: "a", characterId: "a", roleOwnerId: "owner" },
  { memberId: "b", characterId: "b", roleOwnerId: "owner" },
];

describe("群聊点名", () => {
  it("角色名称重叠时只匹配最长的完整名称", () => {
    expect(
      resolveGroupMentions("@大肥鱼 (副本) 你好", members).map(
        (item) => item.memberId,
      ),
    ).toEqual(["b"]);
  });
  it("支持分别点名和所有成员", () => {
    expect(resolveGroupMentions("@大肥鱼 你好 @大肥鱼 (副本)", members)).toEqual(
      mentions,
    );
    expect(resolveGroupMentions("@所有成员 你好", members)).toEqual(mentions);
  });
  it("不发送删除的或失效成员的点名", () => {
    expect(resolveGroupMentions("你好", members)).toEqual([]);
    expect(resolveGroupMentions("@已离开 你好", members)).toEqual([]);
    expect(resolveGroupMentions("@所有成员列表", members)).toEqual([]);
    expect(resolveGroupMentions("@所有成员", [members[0]])).toEqual([
      mentions[0],
    ]);
  });
  it("从手动输入和粘贴的文本直接生成提及元数据", () => {
    expect(resolveGroupMentions("请 @大肥鱼 查看", members)).toEqual([
      mentions[0],
    ]);
  });
});
