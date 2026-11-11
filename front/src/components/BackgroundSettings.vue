<template>
  <el-card shadow="never" class="background-settings" data-amitia-ui="component.card">
    <template #header><div class="setting-row"><span>自定义背景</span><span class="control-wrapper" :tabindex="backgroundReason ? 0 : undefined" :role="backgroundReason ? 'button' : undefined" :aria-label="backgroundReason ? `启用自定义背景：${backgroundReason}` : undefined" @click.capture="explainUnavailable($event, backgroundReason)" @keydown.enter="explainUnavailable($event, backgroundReason)" @keydown.space="explainUnavailable($event, backgroundReason)"><el-switch data-amitia-ui="component.switch" :model-value="settings.enabled" :disabled="!!backgroundReason" aria-label="启用自定义背景" @update:model-value="toggle" /></span></div></template>
    <p>支持图片与静音循环视频，保存在当前设备。视频在后台或减少动画时暂停。</p>
    <div class="setting-row"><span>{{ settings.name || (settings.mode === 'color' ? '纯色背景' : settings.mode === 'gradient' ? '渐变背景' : '尚未选择背景') }}</span><div><span class="control-wrapper" :tabindex="fileReason ? 0 : undefined" :role="fileReason ? 'button' : undefined" :aria-label="fileReason || undefined" @click.capture="explainUnavailable($event, fileReason)" @keydown.enter="explainUnavailable($event, fileReason)" @keydown.space="explainUnavailable($event, fileReason)"><el-button data-amitia-ui="component.button" :disabled="!!fileReason" @click="picker?.click()">{{ source ? '更换背景' : '选择背景' }}</el-button></span><span v-if="source || settings.mode === 'color' || settings.mode === 'gradient'" class="control-wrapper" :tabindex="fileReason ? 0 : undefined" :role="fileReason ? 'button' : undefined" :aria-label="fileReason || undefined" @click.capture="explainUnavailable($event, fileReason)" @keydown.enter="explainUnavailable($event, fileReason)" @keydown.space="explainUnavailable($event, fileReason)"><el-button data-amitia-ui="component.button" :disabled="!!fileReason" @click="run(clear)">移除</el-button></span></div></div>
    <input ref="picker" type="file" accept=".png,.jpg,.jpeg,.webp,.gif,.mp4,.webm" class="file-picker" data-amitia-ui="component.input" aria-label="选择图片或视频背景" @change="picked" />
    <p>图片最大 20 MB，视频最大 150 MB。建议使用 MP4 视频；具体编码支持由设备决定。</p>
    <el-alert v-if="error || message" :title="error ? '无法读取背景设置，请重试' : '保存背景失败，请重试'" type="error" :closable="false" />
    <details v-if="error || message"><summary>查看原始错误</summary><pre>{{ message || error }}</pre></details>
    <el-button v-if="error" data-amitia-ui="component.button" @click="run(init)">重试读取</el-button>
    <label>背景不透明度 {{ Math.round(settings.opacity * 100) }}%</label><div class="slider-wrapper" :tabindex="adjustReason ? 0 : undefined" :role="adjustReason ? 'button' : undefined" :aria-label="adjustReason ? `背景不透明度：${adjustReason}` : undefined" @click.capture="explainUnavailable($event, adjustReason)" @keydown.enter="explainUnavailable($event, adjustReason)" @keydown.space="explainUnavailable($event, adjustReason)"><el-slider data-amitia-ui="component.slider" :model-value="settings.opacity * 100" :disabled="!!adjustReason" :min="0" :max="100" aria-label="背景不透明度" @update:model-value="opacityChanged" /></div>
    <div class="setting-row"><span>背景模糊</span><span class="control-wrapper" :tabindex="adjustReason ? 0 : undefined" :role="adjustReason ? 'button' : undefined" :aria-label="adjustReason ? `背景模糊：${adjustReason}` : undefined" @click.capture="explainUnavailable($event, adjustReason)" @keydown.enter="explainUnavailable($event, adjustReason)" @keydown.space="explainUnavailable($event, adjustReason)"><el-switch data-amitia-ui="component.switch" :model-value="settings.blurEnabled" :disabled="!!adjustReason" aria-label="背景模糊" @update:model-value="blurChanged" /></span></div>
    <label>模糊半径 {{ settings.blurRadius }} px</label><div class="slider-wrapper" :tabindex="radiusReason ? 0 : undefined" :role="radiusReason ? 'button' : undefined" :aria-label="radiusReason ? `模糊半径：${radiusReason}` : undefined" @click.capture="explainUnavailable($event, radiusReason)" @keydown.enter="explainUnavailable($event, radiusReason)" @keydown.space="explainUnavailable($event, radiusReason)"><el-slider data-amitia-ui="component.slider" :model-value="settings.blurRadius" :disabled="!!radiusReason" :min="0" :max="30" aria-label="模糊半径" @update:model-value="radiusChanged" /></div>
    <BackgroundMedia v-if="source || settings.mode === 'color' || settings.mode === 'gradient'" class="background-preview" :settings="settings" :source="source" :animate="false" preview>
      <div class="preview-content"><strong>背景效果预览</strong><span>正文内容保持清晰可读</span><div class="preview-card">内容卡片与输入区保留原有底色</div></div>
    </BackgroundMedia>
  </el-card>
</template>
<script setup lang="ts">
import { computed, ref, onMounted } from "vue";
import { ElMessage } from "element-plus";
import { useBackgroundPreference } from "../composables/useBackgroundPreference";
import BackgroundMedia from "./BackgroundMedia.vue";
import { originalErrorText } from "@/runtime/original-error";
const { settings, source, loading, error, init, update, selectFile, clear } = useBackgroundPreference();
const picker = ref<HTMLInputElement>();
const busy = ref(false), message = ref("");
const hasVisual = computed(() => !!source.value || settings.value.mode === "color" || settings.value.mode === "gradient");
const fileReason = computed(() => loading.value || busy.value ? "正在处理背景设置，请稍后重试。" : error.value ? "无法读取背景设置，请先重试并查看原始错误。" : "");
const adjustReason = computed(() => fileReason.value || (!hasVisual.value ? "请先选择背景。" : ""));
const backgroundReason = computed(() => adjustReason.value);
const radiusReason = computed(() => adjustReason.value || (!settings.value.blurEnabled ? "请先开启背景模糊。" : ""));
function explainUnavailable(event: Event, reason: string) {
  if (!reason) return;
  event.preventDefault();
  event.stopPropagation();
  ElMessage.warning(reason);
}
async function run(action: () => Promise<void>) {
  busy.value = true; message.value = "";
  try { await action(); } catch (error) { message.value = originalErrorText(error); }
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
function opacityChanged(value: number | number[]) { if (typeof value === "number") void update({ opacity: value / 100 }).catch((error) => { message.value = originalErrorText(error); }); }
function radiusChanged(value: number | number[]) { if (typeof value === "number") void update({ blurRadius: value }).catch((error) => { message.value = originalErrorText(error); }); }
onMounted(() => { void run(init); });
</script>
<style scoped>
.background-settings { margin-bottom: 14px; }
.setting-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin: 12px 0; }
.setting-row > span { overflow-wrap: anywhere; min-width: 0; }
.setting-row > div { flex-shrink: 0; }
.control-wrapper { display: inline-flex; }
.slider-wrapper { display: flow-root; }
.control-wrapper[role="button"] :deep(.el-switch),
.control-wrapper[role="button"] :deep(.el-button),
.slider-wrapper[role="button"] :deep(.el-slider) { pointer-events: none; }
pre { white-space: pre-wrap; overflow-wrap: anywhere; user-select: text; }
p { font-size: 13px; line-height: 1.6; color: var(--ac-color-text-secondary); }
label { display: block; margin-top: 16px; }
.file-picker { display: none; }
.background-preview { border: 1px solid var(--ac-color-border); border-radius: var(--ac-radius-md); margin-top: 16px; min-height: 220px; }
.preview-content { position: relative; padding: 24px; display: grid; gap: 12px; color: var(--ac-color-text); }
.preview-card { padding: 16px; background: var(--ac-color-surface); border-radius: var(--ac-radius-md); }
</style>
