import { describe, expect, it } from 'vitest';
import { currentOwnerConversations, requireSavedTask, validateChannelTargets } from '../views/reminders/contract';

describe('统一提醒接入契约', () => {
  it('仅使用当前核心的真实资源编号，不提交 UI 引用或旧所有者会话', () => {
    expect(currentOwnerConversations([{ id: 'meshconv1:core-a:real', resourceId: 'real', ownerId: 'core-a', title: '会话' }, { id: 'old', resourceId: 'old', ownerId: 'core-b' }], 'core-a')).toEqual([{ id: 'real', title: '会话' }]);
  });
  it('保存和查询拒绝空、无版本和虚假成功记录', () => {
    for (const value of [null, {}, { saved: true, id: 'x', ownerId: 'core', generation: 0 }, { saved: true, id: 'x', ownerId: 'core', generation: 1 }]) expect(() => requireSavedTask(value)).toThrow();
    expect(requireSavedTask({ saved: true, id: 'x', ownerId: 'core', generation: 2, enabled: 1 }).enabled).toBe(true);
  });
  it('仅附加已接入的明确接收方，不使用 all 广播或重复目标', () => {
    expect(() => validateChannelTargets([{ channelId: 'wechat', peerId: 'person' }], ['wechat'])).not.toThrow();
    for (const targets of [[{ channelId: 'all', peerId: 'person' }], [{ channelId: 'wechat', peerId: '' }], [{ channelId: 'wechat', peerId: 'person' }, { channelId: 'wechat', peerId: 'person' }]]) expect(() => validateChannelTargets(targets, ['wechat'])).toThrow();
  });
});
