<script setup lang="ts">
import { ref, watch, onMounted } from "vue";
import type { UIContributionSummary } from "@/stores/extensionUI";

const props = defineProps<{
  contribution: UIContributionSummary;
  context?: Record<string, unknown>;
  slotId: string;
}>();

const schema = ref<Record<string, unknown> | null>(null);
const loading = ref(true);
const error = ref<string | null>(null);

async function loadSchema() {
  if (!props.contribution.schemaPath) {
    loading.value = false;
    return;
  }
  loading.value = true;
  error.value = null;
  try {
    const response = await fetch(`/api/extension/schema/${props.contribution.extensionId}/${props.contribution.contributionId}`, {
      method: "GET",
      headers: { "Content-Type": "application/json" },
    });
    if (!response.ok) throw new Error(`schema load failed: ${response.status}`);
    schema.value = await response.json();
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    loading.value = false;
  }
}

onMounted(loadSchema);
watch(() => props.contribution.contributionId, loadSchema);
</script>

<template>
  <div class="schema-ui-renderer" :data-contribution-id="contribution.contributionId">
    <template v-if="loading">
      <div class="schema-ui-renderer__loading">加载中...</div>
    </template>
    <template v-else-if="error">
      <div class="schema-ui-renderer__error">{{ error }}</div>
    </template>
    <template v-else-if="schema">
      <div class="schema-ui-renderer__content">
        <pre>{{ JSON.stringify(schema, null, 2) }}</pre>
      </div>
    </template>
  </div>
</template>

<style scoped>
.schema-ui-renderer {
  width: 100%;
  min-width: 0;
}
.schema-ui-renderer__loading {
  padding: 8px;
  color: var(--amitia-color-text-secondary, rgba(127, 127, 127, 0.8));
  font-size: 12px;
}
.schema-ui-renderer__error {
  padding: 8px;
  color: rgb(180, 40, 40);
  font-size: 12px;
}
.schema-ui-renderer__content {
  width: 100%;
}
</style>
