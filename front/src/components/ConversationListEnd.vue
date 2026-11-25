<template>
  <div ref="end" class="conversation-list-end" aria-hidden="true"></div>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";

const props = defineProps<{ enabled: boolean }>();
const emit = defineEmits<{ reach: [] }>();
const end = ref<HTMLElement>();

watch([end, () => props.enabled], ([element, enabled], _previous, onCleanup) => {
  if (!element || !enabled || typeof IntersectionObserver === "undefined") return;
  const observer = new IntersectionObserver(([entry]) => {
    if (entry?.isIntersecting && props.enabled) emit("reach");
  });
  observer.observe(element);
  onCleanup(() => observer.disconnect());
}, { flush: "post" });
</script>

<style scoped>
.conversation-list-end { flex: 0 0 1px; min-height: 1px; width: 100%; }
</style>
