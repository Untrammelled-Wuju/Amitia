<template>
  <div v-if="visible" class="fa-panel mem-inject-panel">
    <div class="ps-header"><h4>用户画像</h4><button class="ps-close-btn" type="button" aria-label="关闭用户画像" @click="$emit('close')">✕</button></div>
    <DeviceOwnedMemoryPanel v-if="owned.enabled.value" embedded :visible="true" :character-id="characterId || ''" conversation-id="" initial-kind="profile" />
    <el-alert v-else title="当前记忆服务尚未就绪，请恢复连接后重试" type="warning" :closable="false"><el-button @click="refresh">重试</el-button></el-alert>
  </div>
</template>
<script setup lang="ts">
import { watch } from "vue";
import DeviceOwnedMemoryPanel from "@/components/DeviceOwnedMemoryPanel.vue";
import { useDeviceOwnedConversation } from "@/composables/useDeviceOwnedConversation";
const props = defineProps<{ visible: boolean; characterId?: string;  }>();
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
.profile-summary-panel { width: 300px; max-height: min(520px, 68vh); }
.profile-panel-header { display: flex; align-items: center; justify-content: space-between; padding: 10px 12px; border-bottom: 1px solid var(--surface-border); }
.profile-panel-header h4 { margin: 0; color: var(--text-primary); font-size: 13px; font-weight: 600; }
.profile-close-btn { display: grid; place-items: center; width: 26px; height: 26px; border: 0; border-radius: 6px; background: transparent; color: var(--text-muted); cursor: pointer; }
.profile-close-btn:hover { background: var(--control-hover-bg); color: var(--text-primary); }
.profile-loading, .profile-empty { padding: 22px 14px; text-align: center; color: var(--text-muted); font-size: 12px; }
.profile-items { padding: 5px 0; }
.profile-item { display: flex; align-items: center; gap: 8px; padding: 7px 12px; }
.profile-item:hover { background: var(--control-hover-bg); }
.profile-cat { flex: 0 0 auto; min-width: 42px; color: var(--text-muted); font-size: 9px; }
.profile-copy { min-width: 0; flex: 1; }
.profile-copy strong, .profile-copy span { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.profile-copy strong { color: var(--text-secondary); font-size: 10px; font-weight: 550; }
.profile-copy span { margin-top: 1px; color: var(--text-primary); font-size: 11px; }
.profile-conf { flex: 0 0 auto; min-width: 34px; text-align: right; font-size: 9px; font-weight: 600; }
.conf-high { color: var(--ac-color-success); }
.conf-mid { color: var(--ac-color-warning); }
.conf-low { color: var(--ac-color-danger); }
@media (max-width: 768px) { .profile-summary-panel { right: 8px; left: 8px; width: auto; max-height: 66vh; } }
</style>
