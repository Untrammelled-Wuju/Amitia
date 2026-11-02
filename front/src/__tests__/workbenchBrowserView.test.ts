import { beforeEach, describe, expect, it, vi } from 'vitest';
import { flushPromises, mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import WorkbenchBrowserView from '@/components/workbench/WorkbenchBrowserView.vue';
import { useWorkbenchStore } from '@/stores/workbench';

const { get, post, remove } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  remove: vi.fn(),
}));

vi.mock('@/composables/useApi', () => ({
  apiClient: { get, post, delete: remove },
}));

describe('WorkbenchBrowserView', () => {
  beforeEach(() => {
    localStorage.clear();
    get.mockReset();
    post.mockReset();
    remove.mockReset();
    setActivePinia(createPinia());
  });

  it('does not probe an independent session before it is started', async () => {
    const store = useWorkbenchStore();
    store.setConversation('conversation-a');
    const tab = store.openTab({ kind: 'browser' });
    const wrapper = mount(WorkbenchBrowserView, { props: { tab } });
    await flushPromises();
    expect(get).not.toHaveBeenCalled();
    wrapper.unmount();
  });

  it('clears an invalid preview error after a valid preview', async () => {
    const store = useWorkbenchStore();
    store.setConversation('conversation-a');
    const tab = store.openTab({ kind: 'browser' });
    const wrapper = mount(WorkbenchBrowserView, { props: { tab } });
    const button = (text: string) => wrapper.findAll('button').find(item => item.text() === text)!;
    await button('轻量网页预览').trigger('click');
    const address = wrapper.get('input[aria-label="浏览器地址"]');
    await address.setValue('invalid');
    await button('预览').trigger('click');
    expect(wrapper.text()).toContain('请输入有效的 HTTP(S) 网页地址');
    await address.setValue('https://example.com');
    await button('预览').trigger('click');
    expect(wrapper.text()).not.toContain('请输入有效的 HTTP(S) 网页地址');
    wrapper.unmount();
  });
});
