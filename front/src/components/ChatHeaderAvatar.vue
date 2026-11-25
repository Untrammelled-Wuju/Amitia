<template>
  <el-avatar class="header-character-avatar" :size="20" :src="imageUrl || undefined" fit="cover" :title="imageError || name" :aria-label="name">
    {{ initial }}
  </el-avatar>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { apiClient } from "@/composables/useApi";
import { originalErrorText } from "@/runtime/original-error";

const props = defineProps<{ characterId?: string; name: string; avatar?: string }>();
const initial = computed(() => Array.from(props.name.trim() || "角")[0].toUpperCase());
const imageUrl = ref("");
const imageError = ref("");

watch([() => props.characterId, () => props.avatar], async ([characterId, avatar], _previous, onCleanup) => {
  const source = avatar?.trim() || "";
  imageError.value = "";
  imageUrl.value = source.startsWith("/avatars/") ? "" : source;
  if (!source.startsWith("/avatars/") || !characterId) return;

  const controller = new AbortController();
  let objectUrl = "";
  onCleanup(() => {
    controller.abort();
    if (objectUrl) URL.revokeObjectURL(objectUrl);
  });
  try {
    const result = await apiClient.get<Blob>(`/api/characters/${encodeURIComponent(characterId)}/avatar`, {
      params: { v: source },
      responseType: "blob",
      signal: controller.signal,
    });
    if (controller.signal.aborted) return;
    objectUrl = URL.createObjectURL(result.data);
    imageUrl.value = objectUrl;
  } catch (error) {
    if (controller.signal.aborted) return;
    console.error("[original-error]", error);
    imageError.value = originalErrorText(error);
  }
}, { immediate: true });
</script>

<style scoped>
.header-character-avatar {
  flex: 0 0 20px;
  background: var(--control-active-bg);
  color: var(--text-primary);
  font-size: 9px;
  font-weight: 650;
}
</style>
