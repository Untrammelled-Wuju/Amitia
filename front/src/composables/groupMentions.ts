import type {
  OwnedGroupMember,
  OwnedGroupMention,
} from "@/runtime/device-owned-chat";

function toMention(member: OwnedGroupMember): OwnedGroupMention {
  return {
    memberId: member.memberId,
    characterId: member.characterId,
    roleOwnerId: member.roleOwnerId,
  };
}

function hasMentionToken(text: string, token: string): boolean {
  let index = text.indexOf(token);
  while (index >= 0) {
    const next = text[index + token.length];
    if (!next || /[\s，。！？、,.!?;；:：]/u.test(next)) return true;
    index = text.indexOf(token, index + 1);
  }
  return false;
}

export function resolveGroupMentions(
  text: string,
  members: OwnedGroupMember[],
): OwnedGroupMention[] {
  const active = members.filter((member) => member.status === "active");
  if (hasMentionToken(text, "@所有成员")) return active.map(toMention);
  const names = [...active].sort(
    (left, right) => right.characterName.length - left.characterName.length,
  );
  const present = new Set<string>();
  for (let index = 0; index < text.length; index++) {
    if (text[index] !== "@") continue;
    const member = names.find((candidate) => {
      const token = `@${candidate.characterName}`;
      if (!text.startsWith(token, index)) return false;
      const next = text[index + token.length];
      return !next || /[\s，。！？、,.!?;；:：]/u.test(next);
    });
    if (!member) continue;
    present.add(member.memberId);
    index += member.characterName.length;
  }
  return active.filter((member) => present.has(member.memberId)).map(toMention);
}
