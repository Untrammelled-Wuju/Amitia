import { readonly, ref } from "vue";
import { backgroundFileKind, defaultBackground, readBackground, writeBackground, normalizeBackground, type BackgroundSettings } from "./backgroundStorage";
import { resolveConversationMediaUrl } from "../conversation/rendering/media";

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
      const needsMedia = next.mode === "image" || next.mode === "video";
      if (next.enabled && needsMedia && !source.value) throw new Error("请先选择背景文件");
      await writeBackground(next);
      settings.value = next;
      error.value = "";
    });
  }
  async function selectFile(file: File) {
    const kind = backgroundFileKind(file);
    await init();
    return serialize(async () => {
      const next = {
        ...settings.value,
        enabled: true,
        kind,
        mode: kind,
        name: file.name,
        sourceUri: "",
      };
      await writeBackground(next, file);
      replaceSource(file);
      settings.value = next;
      error.value = "";
    });
  }
  async function clear() {
    await init();
    return serialize(async () => {
      const next = { ...defaultBackground };
      await writeBackground(next, null);
      replaceSource(null);
      settings.value = next;
      error.value = "";
    });
  }

  async function applySource(sourceUri: string, kind: "image" | "video", name = "") {
    const resolved = await resolveConversationMediaUrl(sourceUri);
    if (!resolved) throw new Error("背景资源地址无效");
    const response = await fetch(resolved);
    if (!response.ok) throw new Error(`背景资源加载失败 (${response.status})`);
    const blob = await response.blob();
    const file = new File([blob], name || `background-${Date.now()}`, { type: blob.type });
    await init();
    return serialize(async () => {
      const next = normalizeBackground({
        ...settings.value,
        enabled: true,
        kind,
        mode: kind,
        name: file.name,
        sourceUri,
      });
      await writeBackground(next, file);
      replaceSource(file);
      settings.value = next;
      error.value = "";
    });
  }

  async function replace(value: BackgroundSettings, media: Blob | null = null) {
    await init();
    return serialize(async () => {
      const next = normalizeBackground(value);
      const needsMedia = next.mode === "image" || next.mode === "video";
      if (next.enabled && needsMedia && !media && !source.value) {
        throw new Error("恢复背景失败，资源不可用");
      }
      await writeBackground(next, media);
      replaceSource(media);
      settings.value = next;
      error.value = "";
    });
  }

  async function reset() {
    await replace(defaultBackground, null);
  }

  return {
    settings: readonly(settings),
    source: readonly(source),
    error: readonly(error),
    loading: readonly(loading),
    init,
    update,
    selectFile,
    clear,
    applySource,
    replace,
    reset,
  };
}
