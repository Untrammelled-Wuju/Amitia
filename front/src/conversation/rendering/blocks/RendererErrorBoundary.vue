<template>
  <slot v-if="!errorMessage" />
  <section v-else class="amrp-renderer-error">
    <b>{{ label }} 发生异常</b>
    <p>{{ errorMessage }}</p>
    <button type="button" @click="retry">重试</button>
  </section>
</template>

<script setup lang="ts">
import { onErrorCaptured, ref } from "vue";

const props = withDefaults(
  defineProps<{
    label?: string;
  }>(),
  {
    label: "Renderer",
  },
);

const errorMessage = ref("");
const revision = ref(0);

onErrorCaptured((error) => {
  errorMessage.value = error instanceof Error ? error.message : String(error);
  return false;
});

function retry() {
  errorMessage.value = "";
  revision.value += 1;
}
</script>

<style scoped>
.amrp-renderer-error {
  max-width: 700px;
  margin: 12px 0 16px;
  border: 1px solid color-mix(in srgb, var(--amrp-danger) 38%, var(--amrp-line));
  border-radius: 9px;
  padding: 10px;
  background: color-mix(in srgb, var(--amrp-danger) 7%, var(--amrp-surface));
}

.amrp-renderer-error b {
  color: var(--amrp-danger);
  font-size: 11.5px;
}

.amrp-renderer-error p {
  margin: 4px 0 0;
  color: var(--amrp-muted);
  font-size: 10.5px;
}

.amrp-renderer-error button {
  margin-top: 7px;
  border: 0;
  border-radius: 6px;
  padding: 4px 7px;
  background: var(--amrp-soft);
  color: var(--amrp-accent);
  font: inherit;
  font-size: 10px;
  cursor: pointer;
}
</style>

