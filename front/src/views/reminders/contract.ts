export function requireSavedTask(value: any): any {
  if (value?.saved !== true || typeof value.id !== 'string' || !value.id || typeof value.ownerId !== 'string' || !value.ownerId || !Number.isInteger(value.generation) || value.generation < 1 || ![0, 1, true, false].includes(value.enabled)) throw new Error('服务未确认有效任务记录');
  return { ...value, enabled: Boolean(value.enabled) };
}

export function currentOwnerConversations(rows: any[], ownerId: string): Array<{ id: string; title: string }> {
  return rows.filter((item) => item.ownerId === ownerId && typeof item.resourceId === 'string' && item.resourceId).map((item) => ({ id: item.resourceId, title: item.title || item.resourceId }));
}

export function validateChannelTargets(targets: Array<{ channelId: string; peerId: string }>, channels: string[]): void {
  const seen = new Set<string>();
  for (const target of targets) {
    const key = `${target.channelId}\0${target.peerId.trim()}`;
    if (!channels.includes(target.channelId) || !target.peerId.trim() || seen.has(key)) throw new Error('请选择已接入渠道、填写接收方标识，并移除重复目标');
    seen.add(key);
  }
}
