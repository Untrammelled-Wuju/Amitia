<script setup lang="ts">
import { ref, watch } from "vue";
import { ElImage } from "element-plus";
import { apiClient } from "@/composables/useApi";

const props = defineProps<{ src: string; alt?: string; preview?: boolean; fit?: "fill" | "contain" | "cover" | "none" | "scale-down" }>();
const resolved = ref("");
const failed = ref(false);

watch(() => props.src, async (source, _, onCleanup) => {
  let disposed = false;
  let objectUrl = "";
  const controller = new AbortController();
  onCleanup(() => {
    disposed = true;
    controller.abort();
    if (objectUrl) URL.revokeObjectURL(objectUrl);
  });
  resolved.value = "";
  failed.value = false;
  if (!source) return;
  if (!source.startsWith("/api/extension/resources/")) {
    resolved.value = source;
    return;
  }
  try {
    const response = await apiClient.get<Blob>(source, { responseType: "blob", signal: controller.signal });
    if (disposed) return;
    if (!response.data.type.startsWith("image/")) throw new Error("图片资源不可用");
    objectUrl = URL.createObjectURL(response.data);
    resolved.value = objectUrl;
  } catch (error) {
  console.error("[original-error]", error);
if (!disposed) failed.value = true;
  }
}, { immediate: true });
</script>

<template>
  <el-image
    v-if="preview"
    :src="resolved"
    :alt="alt || ''"
    :fit="fit || 'contain'"
    :preview-src-list="resolved ? [resolved] : []"
    preview-teleported
  >
    <template #error><span class="schema-image-status">图片加载失败</span></template>
    <template #placeholder><span class="schema-image-status">加载中</span></template>
  </el-image>
  <span v-else-if="failed" class="schema-image-status" role="img" :aria-label="alt || '图片加载失败'">图片加载失败</span>
  <img v-else :src="resolved || undefined" :alt="alt || ''" :style="{ objectFit: fit }" loading="lazy" @error="failed = true" />
</template>

<style scoped>
.schema-image-status {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  color: var(--text-secondary);
  font-size: var(--ac-font-size-sm);
}
</style>
