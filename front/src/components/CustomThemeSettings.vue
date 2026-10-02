<template>
  <el-card shadow="never" class="custom-theme">
    <template #header><div class="setting-row"><span>自定义配色</span><el-switch :model-value="palette.enabled" aria-label="启用自定义配色" @update:model-value="togglePalette" /></div></template>
    <p>主色用于主要操作，副色用于辅助操作。关闭后恢复预设配色。</p>
    <div class="color-grid">
      <label v-for="field in fields" :key="field.key">{{ field.label }}<el-color-picker :model-value="palette[field.key]" :disabled="!palette.enabled" :aria-label="field.label" @change="changeColor(field.key, $event)" /><span>{{ palette[field.key] }}</span></label>
    </div>
    <div class="setting-row"><span>文字颜色模式</span><el-select :model-value="palette.textMode" :disabled="!palette.enabled" aria-label="文字颜色模式" @update:model-value="changeTextMode"><el-option v-for="option in modes" :key="option.value" :label="option.label" :value="option.value" /></el-select></div>
    <p>选择文字颜色会切换为自选；自动模式按背景选择高对比文字。页面亮色、暗色和跟随系统仍由上方主题模式控制。</p>
    <div class="preview-grid">
      <div v-for="sample in samples" :key="sample.label" class="sample" :style="{ background: sample.background, color: sample.text }">
        <strong>{{ sample.label }}</strong><span>这是文字与背景的预览</span><small>对比度 {{ contrastRatio(sample.text, sample.background).toFixed(2) }}:1 · {{ contrastRatio(sample.text, sample.background) >= 4.5 ? '正文清晰' : '建议提高对比度' }}</small>
      </div>
    </div>
  </el-card>
</template>
<script setup lang="ts">
import { computed } from "vue";
import { useTheme } from "../composables/useTheme";
import { ElMessage } from "element-plus";
import { contrastRatio, paletteText, readableText, type CustomPalette } from "../composables/customPalette";
const { state, setCustomPalette } = useTheme();
const palette = computed(() => state.value.customPalette);
const fields = [{ key: "primary", label: "主色" }, { key: "secondary", label: "副色" }, { key: "text", label: "文字颜色" }] as const;
const modes = [{ value: "auto", label: "自动" }, { value: "dark", label: "深色文字" }, { value: "light", label: "浅色文字" }, { value: "custom", label: "自选颜色" }];
function save(value: Partial<CustomPalette>) {
  try { setCustomPalette(value); } catch { ElMessage.error("保存配色失败，请重试"); }
}
function togglePalette(value: boolean | string | number) { save({ enabled: Boolean(value) }); }
function changeTextMode(value: CustomPalette["textMode"]) { save({ textMode: value }); }
function changeColor(key: "primary" | "secondary" | "text", value: string | null) {
  if (value) save({ [key]: value, ...(key === "text" ? { textMode: "custom" as CustomPalette["textMode"] } : {}) });
}
const samples = computed(() => [
  { label: "浅色背景", background: "#FFFFFF", text: paletteText(palette.value, "#FFFFFF") },
  { label: "深色背景", background: "#121214", text: paletteText(palette.value, "#121214") },
  { label: "主色操作", background: palette.value.primary, text: readableText(palette.value.primary) },
  { label: "副色操作", background: palette.value.secondary, text: readableText(palette.value.secondary) },
]);
</script>
<style scoped>
.custom-theme { margin-bottom: 14px; }
.setting-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.setting-row .el-select { width: 150px; }
p { color: var(--ac-color-text-secondary); font-size: 13px; line-height: 1.6; }
.color-grid, .preview-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 12px; margin: 16px 0; }
label { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
label span { font-size: 12px; }
.sample { display: flex; flex-direction: column; gap: 10px; padding: 16px; border-radius: var(--ac-radius-md); border: 1px solid var(--ac-color-border); }
.sample small { font-size: 12px; }
</style>
