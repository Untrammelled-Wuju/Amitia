import { beforeEach, describe, expect, it, vi } from 'vitest';
import { flushPromises, mount } from '@vue/test-utils';
import CharacterGenerationChat from '../views/character-config/components/CharacterGenerationChat.vue';
const { post } = vi.hoisted(() => ({ post: vi.fn() }));
vi.mock('../composables/useApi', () => ({ useApi: () => ({ post }) }));
const stubs = {
  ElInput: { props: ['modelValue', 'disabled'], emits: ['update:modelValue'], template: '<textarea :value="modelValue" :disabled="disabled" @input="$emit(\'update:modelValue\', $event.target.value)" />' },
  ElButton: { props: ['disabled'], template: '<button :disabled="disabled"><slot/></button>' },
  ElAlert: { props: ['title'], template: '<div role="alert">{{title}}</div>' },
};
describe('character generation draft conversation', () => {
  beforeEach(() => { post.mockReset(); });
  it('merges multi-turn patches and uses manually edited values after applying', async () => {
    post.mockResolvedValueOnce({ reply:'已生成', draft:{name:'星河',personalityConfig:{warmth:72}} })
      .mockResolvedValueOnce({reply:'身份已完善',draft:{identity:'图书管理员'}})
      .mockResolvedValueOnce({reply:'继续调整',draft:{personality:'温和'}});
    const wrapper = mount(CharacterGenerationChat, {props:{draft:{name:'旧名',personalityConfig:{warmth:50,humor:30}}},global:{stubs}});
    async function send(value: string) { await wrapper.get('textarea').setValue(value); await wrapper.findAll('button')[0].trigger('click'); await flushPromises(); }
    await send('喜欢天文的角色');
    await send('设为图书管理员');
    expect(post.mock.calls[1][1].messages).toHaveLength(3);
    expect(post.mock.calls[1][1].draft.name).toBe('星河');
    await wrapper.findAll('button')[1].trigger('click');
    expect(wrapper.emitted('apply')?.[0][0]).toEqual({name:'星河',identity:'图书管理员',personalityConfig:{warmth:72}});
    await wrapper.setProps({draft:{name:'手工改名',personalityConfig:{warmth:60,humor:30}}});
    await send('再温和一点');
    expect(post.mock.calls[2][1].draft.name).toBe('手工改名');
    expect(post.mock.calls[2][1].draft.personalityConfig.warmth).toBe(60);
    wrapper.unmount();
  });
  it('keeps the input and existing draft on failure', async () => {
    post.mockImplementation(async () => { throw new Error('offline'); });
    const draft = {name:'原角色'};
    const wrapper = mount(CharacterGenerationChat,{props:{draft},global:{stubs}});
    await wrapper.get('textarea').setValue('完善设定');
    await wrapper.findAll('button')[0].trigger('click');
    await flushPromises();
    expect(wrapper.get('textarea').element.value).toBe('完善设定');
    expect(wrapper.get('[role="alert"]').text()).toContain('生成失败');
    expect(wrapper.emitted('apply')).toBeUndefined();
    expect(draft.name).toBe('原角色');
    wrapper.unmount();
  });
});
