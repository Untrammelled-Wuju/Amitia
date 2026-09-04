import { ref, type Ref } from 'vue';
import type { AmitiaUI, AmitiaUIContext } from './amitia-ui';
import { getAmitiaUI, isAmitiaUIAvailable, waitUntilReady } from './amitia-ui';

export function useAmitiaUI() {
  const ui = ref<AmitiaUI | null>(null);
  const ready = ref(false);
  const error = ref<string | null>(null);
  const context = ref<AmitiaUIContext | null>(null);

  async function init() {
    try {
      const instance = await waitUntilReady();
      ui.value = instance;
      ready.value = true;
      const ctx = await instance.getContext();
      context.value = ctx as AmitiaUIContext;
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e);
    }
  }

  return { ui, ready, error, context, init };
}

export function useTheme() {
  const mode = ref('light');
  const density = ref('comfortable');

  async function refresh() {
    if (!isAmitiaUIAvailable()) return;
    try {
      const ui = getAmitiaUI();
      const ctx = await ui.getContext();
      mode.value = ctx.theme.mode;
      density.value = ctx.theme.density;
    } catch {}
  }

  return { mode, density, refresh };
}
