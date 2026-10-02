import { readonly, ref } from "vue";
import { backgroundFileKind, defaultBackground, readBackground, writeBackground, normalizeBackground, type BackgroundSettings } from "./backgroundStorage";

const settings = ref<BackgroundSettings>({ ...defaultBackground });
const source = ref("");
const error = ref("");
const loading = ref(true);
let initialization: Promise<void> | undefined;
let queue = Promise.resolve();
function replaceSource(blob: Blob | null) {
  const previous = source.value;
  source.value = blob ? URL.createObjectURL(blob) : "";
  if (previous) URL.revokeObjectURL(previous);
}
function init() {
  return initialization ??= readBackground().then(result => {
    settings.value = result.media ? result.settings : { ...result.settings, enabled: false, name: "" };
    replaceSource(result.media);
    error.value = "";
  }).catch(() => {
    error.value = "无法读取背景设置，请重试";
    initialization = undefined;
  }).finally(() => { loading.value = false; });
}
function serialize(action: () => Promise<void>) {
  const next = queue.then(action);
  queue = next.catch(() => {});
  return next;
}
export function useBackgroundPreference() {
  async function update(value: Partial<BackgroundSettings>) {
    await init();
    return serialize(async () => {
      const next = normalizeBackground({ ...settings.value, ...value });
      if (next.enabled && !source.value) throw new Error("请先选择背景文件");
      await writeBackground(next);
      settings.value = next;
      error.value = "";
    });
  }
  async function selectFile(file: File) {
    const kind = backgroundFileKind(file);
    await init();
    return serialize(async () => {
      const next = { ...settings.value, enabled: true, kind, name: file.name };
      await writeBackground(next, file);
      replaceSource(file);
      settings.value = next;
      error.value = "";
    });
  }
  async function clear() {
    await init();
    return serialize(async () => {
      const next = { ...settings.value, enabled: false, name: "" };
      await writeBackground(next, null);
      replaceSource(null);
      settings.value = next;
      error.value = "";
    });
  }
  return { settings: readonly(settings), source: readonly(source), error: readonly(error), loading: readonly(loading), init, update, selectFile, clear };
}
