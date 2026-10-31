<template>
  <div v-if="visible" class="fa-panel mem-inject-panel">
    <div class="mi-header"><h4>记忆上下文</h4><button class="mi-close-btn" type="button" aria-label="关闭记忆上下文" @click="$emit('close')">✕</button></div>
    <DeviceOwnedMemoryPanel v-if="owned.enabled.value" embedded :visible="true" :character-id="characterId || ''" :conversation-id="convId" />
    <el-alert v-else title="当前记忆服务尚未就绪，请恢复连接后重试" type="warning" :closable="false"><el-button @click="refresh">重试</el-button></el-alert>
  </div>
</template>
<script setup lang="ts">
import { watch } from "vue";
import DeviceOwnedMemoryPanel from "@/components/DeviceOwnedMemoryPanel.vue";
import { useDeviceOwnedConversation } from "@/composables/useDeviceOwnedConversation";
const props = defineProps<{ visible: boolean; characterId?: string; convId: string; userMessage?: string; }>();
defineEmits<{ close: [] }>();
const owned = useDeviceOwnedConversation();
async function refresh() { try { await owned.refresh(); } catch (error) {
  console.error("[original-error]", error);
} }
watch(() => props.visible, visible => { if (visible) void refresh(); }, { immediate: true });
</script>
<style scoped>
.fa-panel {
  position: absolute;
  right: 14px;
  top: 8px;
  z-index: 19;
  overflow-y: auto;
  border: 1px solid var(--surface-border);
  border-radius: 10px;
  background: var(--surface-bg-elevated);
  box-shadow: var(--tp-shadow-float);
}
.mem-inject-panel { width: 330px; max-height: min(560px, 72vh); }
.mi-header { display: flex; align-items: center; justify-content: space-between; padding: 10px 12px; border-bottom: 1px solid var(--surface-border); }
.mi-header h4 { margin: 0; color: var(--text-primary); font-size: 13px; font-weight: 600; }
.mi-close-btn { display: grid; place-items: center; width: 26px; height: 26px; border: 0; border-radius: 6px; background: transparent; color: var(--text-muted); cursor: pointer; }
.mi-close-btn:hover { background: var(--control-hover-bg); color: var(--text-primary); }
.mi-loading, .mi-empty { padding: 22px 14px; text-align: center; color: var(--text-muted); font-size: 12px; }
.mi-section { padding: 9px 12px; border-bottom: 1px solid var(--surface-border); }
.mi-section:last-child { border-bottom: 0; }
.mi-section h5 { margin: 0 0 7px; color: var(--text-secondary); font-size: 11px; font-weight: 600; }
.mi-item { padding: 7px 0; border-bottom: 1px solid color-mix(in srgb, var(--surface-border) 70%, transparent); }
.mi-item:last-child { border-bottom: 0; }
.mi-item-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.mi-layer { color: var(--ac-color-primary); font-size: 10px; }
.mi-score { color: var(--text-muted); font-size: 9px; }
.mi-key { margin-top: 4px; color: var(--text-secondary); font-size: 11px; font-weight: 550; }
.mi-content { margin-top: 3px; color: var(--text-primary); font-size: 11px; line-height: 1.5; word-break: break-word; }
.mi-profile-card { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 5px 0; color: var(--text-secondary); font-size: 11px; }
.mi-compress { display: flex; flex-direction: column; gap: 3px; color: var(--text-muted); font-size: 10px; }
.mi-pipeline { display: flex; align-items: center; gap: 7px; min-height: 18px; }
.mi-pl-dot { display: inline-block; width: 7px; height: 7px; border-radius: 50%; }
.mi-muted { color: var(--text-muted); font-size: 10px; }
@media (max-width: 768px) { .mem-inject-panel { right: 8px; left: 8px; width: auto; max-height: 66vh; } }
</style>
