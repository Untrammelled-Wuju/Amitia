<template>
  <div class="provider-settings">
    <div class="heading">
      <div>
        <h2>界面提供者</h2>
        <p>为每个 UI 能力选择 Built-in 或扩展 Provider。未显式选择时优先使用 Amitia Built-in，并按平台自动解析可用回退。</p>
      </div>
      <el-button :loading="store.loading" @click="store.refreshSnapshot(true)">刷新</el-button>
    </div>
    <el-alert type="info" :closable="false" show-icon title="恢复路径、登录和首次引导属于安全壳，不允许扩展替换。" />
    <div v-for="group in groups" :key="group.name" class="group">
      <h3>{{ group.name }}</h3>
      <div v-for="capability in group.capabilities" :key="capability" class="row">
        <div class="capability">
          <strong>{{ capability }}</strong>
          <span>{{ providerLabel(capability) }}</span>
        </div>
        <el-select
          :model-value="selection(capability)"
          :loading="saving === capability"
          @change="(value) => choose(capability, String(value || ''))"
        >
          <el-option label="自动（Built-in 优先）" value="" />
          <el-option
            v-for="provider in store.getProviders(capability)"
            :key="provider.providerId"
            :label="`${provider.providerId}${provider.builtin ? ' · Built-in' : ''}`"
            :value="provider.providerId"
            :disabled="!provider.enabled"
          />
        </el-select>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { ElMessage } from "element-plus";
import { useExtensionUIStore } from "@/stores/extensionUI";
import type { UIProviderCapability } from "@/ui-runtime/types";

const store = useExtensionUIStore();
const saving = ref<UIProviderCapability | "">("");
const groups: Array<{ name: string; capabilities: UIProviderCapability[] }> = [
  { name: "应用外壳", capabilities: ["app.shell", "app.navigation", "app.workspace", "route.registry", "page.provider"] },
  { name: "对话", capabilities: ["conversation.shell", "conversation.header", "conversation.messages", "conversation.message_renderer", "conversation.sidebar", "conversation.composer", "conversation.overlay"] },
  { name: "业务页面", capabilities: ["character.shell", "character.detail", "memory.shell", "memory.detail", "settings.shell", "settings.section", "extension.center", "extension.page"] },
  { name: "设计系统", capabilities: ["ui.theme", "ui.tokens", "ui.icons", "ui.components"] },
];

function selection(capability: UIProviderCapability) {
  return store.snapshot?.profile?.selections?.[capability] ?? "";
}
function providerLabel(capability: UIProviderCapability) {
  const provider = store.getResolvedProvider(capability);
  return provider ? `当前：${provider.providerId}` : "当前：无可用 Provider";
}
async function choose(capability: UIProviderCapability, providerId: string) {
  saving.value = capability;
  try {
    await store.selectProvider(capability, providerId || undefined);
    ElMessage.success("界面提供者已更新");
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "更新失败");
  } finally {
    saving.value = "";
  }
}
void store.refreshSnapshot();
</script>

<style scoped>
.provider-settings { display: grid; gap: 18px; }
.heading { display:flex; justify-content:space-between; align-items:flex-start; gap:16px; }
h2 { margin:0 0 6px; font-size:22px; }
p { margin:0; color:var(--el-text-color-secondary); }
.group { border:1px solid var(--el-border-color-light); border-radius:12px; overflow:hidden; }
.group h3 { margin:0; padding:12px 16px; background:var(--el-fill-color-light); font-size:14px; }
.row { display:grid; grid-template-columns:minmax(280px,1fr) minmax(260px,420px); gap:16px; align-items:center; padding:12px 16px; border-top:1px solid var(--el-border-color-lighter); }
.capability { display:flex; flex-direction:column; gap:4px; }
.capability span { font-size:12px; color:var(--el-text-color-secondary); }
@media (max-width:760px) { .row { grid-template-columns:1fr; } }
</style>
