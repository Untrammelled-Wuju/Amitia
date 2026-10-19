import type { ChannelPresentationDescriptor } from "@/ui-runtime/channelPresentation";
import { resolveConversationChannelId } from "./channelMessageMedia";

export interface ChannelConversationGroup {
  channel: string;
  label: string;
  order: number;
  items: Array<Record<string, any>>
}

export function buildChannelConversationGroups(
  presentations: ChannelPresentationDescriptor[],
  conversations: Array<Record<string, any>>,
): ChannelConversationGroup[] {
  const groups = new Map<string, ChannelConversationGroup>()
  for (const presentation of presentations) {
    groups.set(presentation.channelId, {
      channel: presentation.channelId,
      label: presentation.displayName || presentation.channelId,
      order: presentation.order,
      items: [],
    })
  }
  const availableChannels = new Set(groups.keys())
  for (const conversation of conversations) {
    const channel = resolveConversationChannelId(conversation, availableChannels)
    if (!channel || channel === "web") continue
    const group = groups.get(channel)
    if (!group) continue
    group.items.push({ ...conversation, channel })
  }
  for (const group of groups.values()) {
    group.items.sort((left, right) => String(right.updatedAt || "").localeCompare(String(left.updatedAt || "")))
  }
  return [...groups.values()].sort(
    (left, right) => left.order - right.order || left.label.localeCompare(right.label) || left.channel.localeCompare(right.channel),
  )
}
