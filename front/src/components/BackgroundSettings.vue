<template>
  <el-card shadow="never" class="background-settings">
    <template #header><div class="setting-row"><span>自定义背景</span><el-switch :model-value="settings.enabled" :disabled="loading || busy || !source" aria-label="启用自定义背景" @update:model-value="toggle" /></div></template>
    <p>支持图片与静音循环视频，保存在当前设备。视频在后台或减少动画时暂停。</p>
    <div class="setting-row"><span>{{ settings.name || '尚未选择背景' }}</span><div><el-button :disabled="busy || loading" @click="picker?.click()">{{ source ? '更换背景' : '选择背景' }}</el-button><el-button v-if="source" :disabled="busy" @click="run(clear)">移除</el-button></div></div>
    <input ref="picker" type="file" accept=".png,.jpg,.jpeg,.webp,.gif,.mp4,.webm" class="file-picker" aria-label="选择图片或视频背景" @change="picked" />
    <p>图片最大 20 MB，视频最大 150 MB。建议使用 MP4 视频；具体编码支持由设备决定。</p>
    <el-alert v-if="error || message" :title="message || error" type="error" :closable="false" />
    <el-button v-if="error" @click="run(init)">重试读取</el-button>
    <label>背景不透明度 {{ Math.round(settings.opacity * 100) }}%</label><el-slider :model-value="settings.opacity * 100" :disabled="busy || !source" :min="0" :max="100" aria-label="背景不透明度" @update:model-value="opacityChanged" />
    <div class="setting-row"><span>背景模糊</span><el-switch :model-value="settings.blurEnabled" :disabled="busy || !source" aria-label="背景模糊" @update:model-value="blurChanged" /></div>
    <label>模糊半径 {{ settings.blurRadius }} px</label><el-slider :model-value="settings.blurRadius" :disabled="busy || !source || !settings.blurEnabled" :min="0" :max="30" aria-label="模糊半径" @update:model-value="radiusChanged" />
    <BackgroundMedia v-if="source" class="background-preview" :settings="settings" :source="source" :animate="false" preview>
      <div class="preview-content"><strong>背景效果预览</strong><span>正文内容保持清晰可读</span><div class="preview-card">内容卡片与输入区保留原有底色</div></div>
    </BackgroundMedia>
  </el-card>
</template>
<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useBackgroundPreference } from "../composables/useBackgroundPreference";
import BackgroundMedia from "./BackgroundMedia.vue";
const { settings, source, loading, error, init, update, selectFile, clear } = useBackgroundPreference();
const picker = ref<HTMLInputElement>();
const busy = ref(false), message = ref("");
async function run(action: () => Promise<void>) {
  busy.value = true; message.value = "";
  try { await action(); } catch (error) { message.value = error instanceof Error ? error.message : "保存背景失败，请重试"; }
  finally { busy.value = false; }
}
async function picked(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (file) await run(() => selectFile(file));
  input.value = "";
}
function toggle(value: boolean | string | number) { void run(() => update({ enabled: Boolean(value) })); }
function blurChanged(value: boolean | string | number) { void run(() => update({ blurEnabled: Boolean(value) })); }
function opacityChanged(value: number | number[]) { if (typeof value === "number") void update({ opacity: value / 100 }).catch(() => { message.value = "保存背景失败，请重试"; }); }
function radiusChanged(value: number | number[]) { if (typeof value === "number") void update({ blurRadius: value }).catch(() => { message.value = "保存背景失败，请重试"; }); }
onMounted(() => { void init(); });
</script>
<style scoped>
.background-settings { margin-bottom: 14px; }
.setting-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin: 12px 0; }
.setting-row > span { overflow-wrap: anywhere; min-width: 0; }
.setting-row > div { flex-shrink: 0; }
p { font-size: 13px; line-height: 1.6; color: var(--ac-color-text-secondary); }
label { display: block; margin-top: 16px; }
.file-picker { display: none; }
.background-preview { border: 1px solid var(--ac-color-border); border-radius: var(--ac-radius-md); margin-top: 16px; min-height: 220px; }
.preview-content { position: relative; padding: 24px; display: grid; gap: 12px; color: var(--ac-color-text); }
.preview-card { padding: 16px; background: var(--ac-color-surface); border-radius: var(--ac-radius-md); }
</style>
