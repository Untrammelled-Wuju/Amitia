<template>
  <div class="capability-list">
    <article v-for="capability in capabilities" :key="capability" class="capability-item" :class="{ high: highRisk.includes(capability) }">
      <div><strong>{{ capability }}</strong><el-tag v-if="highRisk.includes(capability)" type="danger" size="small">高风险</el-tag><el-tag v-else type="info" size="small">常规</el-tag></div>
      <p>{{ stepText(capability) }}</p>
      <el-checkbox v-if="confirmable" :model-value="confirmed.includes(capability)" @change="$emit('toggle', capability, Boolean($event))">我已理解并确认此权限</el-checkbox>
    </article>
    <el-empty v-if="capabilities.length === 0" description="此工作流不需要外部 Capability" :image-size="70" />
  </div>
</template>
<script setup lang="ts">
const props = defineProps<{ capabilities: string[]; highRisk: string[]; byStep: Record<string, string[]>; confirmed: string[]; confirmable?: boolean }>()
defineEmits<{ toggle: [capability: string, value: boolean] }>()
function stepText(capability: string) { const steps = Object.entries(props.byStep).filter(([, values]) => values.includes(capability)).map(([step]) => step); return steps.length ? `由步骤 ${steps.join("、")} 触发` : "由依赖 Skill 或工作流行为触发" }
</script>
<style scoped>
.capability-list { display: grid; gap: 10px; }
.capability-item { padding: 14px 16px; border: 1px solid var(--console-border); border-radius: 10px; background: var(--ac-color-surface); }
.capability-item.high { border-color: var(--el-color-danger-light-5); }
.capability-item div { display: flex; align-items: center; gap: 8px; }
.capability-item p { margin: 7px 0; color: var(--console-text-muted); font-size: 13px; }
</style>
