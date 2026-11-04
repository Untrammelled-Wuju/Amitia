import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushPromises, mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import ElementPlus from 'element-plus';
import WorkbenchDock from '@/components/workbench/WorkbenchDock.vue';
import { useWorkbenchStore } from '@/stores/workbench';

function menuButton(label: string) {
  return [...document.body.querySelectorAll<HTMLButtonElement>('.tab-context-menu button')]
    .find(button => button.textContent?.trim() === label);
}

function mountDock() {
  return mount(WorkbenchDock, {
    props: { mobile: false },
    global: {
      plugins: [ElementPlus],
      stubs: {
        WorkbenchResourceView: { template: '<div class="resource-stub"></div>' },
      },
    },
  });
}

describe('WorkbenchDock', () => {
  beforeEach(() => {
    localStorage.clear();
    document.body.innerHTML = '';
    setActivePinia(createPinia());
    vi.stubGlobal('crypto', { randomUUID: (() => {
      let index = 0;
      return () => `tab-${++index}`;
    })() });
  });

  afterEach(() => {
    document.body.innerHTML = '';
  });

  it('renders the codex-style toolbar and tab icons', () => {
    const store = useWorkbenchStore();
    store.setConversation('conversation-a');
    store.openTab({ kind: 'files', forceNew: true });
    store.openTab({ kind: 'git', forceNew: true });
    const wrapper = mountDock();
    const actions = wrapper.findAll('.dock-actions button');
    expect(actions.map(action => action.attributes('aria-label'))).toEqual([
      '打开文件',
      'Git 变更',
      '打开浏览器',
      '任务状态',
      '子智能体',
      '交互终端',
      '关闭未固定标签',
      '收起工作区',
    ]);
    expect(wrapper.findAll('.dock-actions svg')).toHaveLength(8);
    expect(wrapper.findAll('.tab-icon svg')).toHaveLength(2);
    expect(wrapper.text()).toContain('工作区');
    wrapper.unmount();
  });

  it('pins, protects and closes tabs from the context menu', async () => {
    const store = useWorkbenchStore();
    store.setConversation('conversation-a');
    const files = store.openTab({ kind: 'files', forceNew: true });
    const git = store.openTab({ kind: 'git', forceNew: true });
    const tasks = store.openTab({ kind: 'tasks', forceNew: true });
    store.activateTab(git.id);
    const wrapper = mountDock();

    await wrapper.findAll('.tab-item')[1].trigger('contextmenu', { clientX: 20, clientY: 20 });
    menuButton('固定标签')?.click();
    await flushPromises();
    expect(store.tabs.find(tab => tab.id === git.id)?.pinned).toBe(true);
    expect(wrapper.findAll('.tab-item')[1].get('.tab-close').attributes('disabled')).toBeDefined();

    await wrapper.findAll('.tab-item')[2].trigger('contextmenu', { clientX: 20, clientY: 20 });
    menuButton('关闭其他')?.click();
    await flushPromises();
    expect(store.tabs.map(tab => tab.id)).toEqual([git.id, tasks.id]);
    expect(store.tabs.some(tab => tab.id === files.id)).toBe(false);

    await wrapper.findAll('.tab-item')[1].trigger('contextmenu', { clientX: 20, clientY: 20 });
    menuButton('关闭未固定标签')?.click();
    await flushPromises();
    expect(store.tabs.map(tab => tab.id)).toEqual([git.id]);
    wrapper.unmount();
  });

  it('keeps pinned tabs when closing from the toolbar and preserves tabs to the left', async () => {
    const store = useWorkbenchStore();
    store.setConversation('conversation-a');
    const pinned = store.openTab({ kind: 'files', forceNew: true });
    store.openTab({ kind: 'git', forceNew: true });
    const last = store.openTab({ kind: 'browser', forceNew: true });
    store.setPinned(pinned.id, true);
    store.activateTab(last.id);
    const wrapper = mountDock();

    await wrapper.get('button[aria-label="关闭未固定标签"]').trigger('click');
    expect(store.tabs.map(tab => tab.id)).toEqual([pinned.id]);

    store.openTab({ kind: 'git', forceNew: true });
    await flushPromises();
    await wrapper.findAll('.tab-item')[0].trigger('contextmenu', { clientX: 20, clientY: 20 });
    menuButton('关闭右侧标签')?.click();
    await flushPromises();
    expect(store.tabs.map(tab => tab.id)).toEqual([pinned.id]);
    wrapper.unmount();
  });
});
