<template>
  <div v-if="surface?.sections?.length" class="surface-grid">
    <el-card
      v-for="section in surface.sections"
      :key="section.id"
      shadow="never"
    >
      <template #header>{{
        section.title || section.label || surface.title
      }}</template>
      <SurfaceForm
        v-if="section.type === 'form'"
        v-model="formModel"
        :fields="section.fields || []"
        :saving="saving"
        @save="$emit('save-config', formModel)"
      />
      <SurfaceAction
        v-else-if="section.type === 'action'"
        :section="section"
        :running="runningAction === section.id"
        @run="$emit('run-action', $event)"
      />
      <SurfaceStatus v-else-if="section.type === 'status'" :health="health" />
      <SurfaceTable
        v-else-if="section.type === 'table'"
        :columns="section.columns || []"
        :rows="stateRows"
      />
    </el-card>
  </div>
  <el-empty v-else description="此插件未声明管理界面" />
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import SurfaceAction from "./SurfaceAction.vue";
import SurfaceForm from "./SurfaceForm.vue";
import SurfaceStatus from "./SurfaceStatus.vue";
import SurfaceTable from "./SurfaceTable.vue";
import type { PluginHealth, PluginState, SurfaceDocument } from "../types";

const props = defineProps<{
  surface?: SurfaceDocument;
  config: Record<string, unknown>;
  health?: PluginHealth;
  states: PluginState[];
  saving?: boolean;
  runningAction?: string;
}>();
defineEmits<{
  "save-config": [config: Record<string, unknown>];
  "run-action": [actionId: string];
}>();
const formModel = ref<Record<string, any>>({});
watch(
  () => props.config,
  (value) => {
    formModel.value = structuredClone(value || {});
  },
  { immediate: true, deep: true },
);
const stateRows = computed(() =>
  props.states.map((item) =>
    typeof item.data === "object" && item.data
      ? (item.data as Record<string, unknown>)
      : { value: item.data },
  ),
);
</script>

<style scoped>
.surface-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}
@media (max-width: 840px) {
  .surface-grid {
    grid-template-columns: 1fr;
  }
}
</style>
