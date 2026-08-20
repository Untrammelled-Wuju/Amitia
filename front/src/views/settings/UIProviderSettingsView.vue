<template>
  <div class="provider-settings">
    <div class="heading">
      <div>
        <h2>界面提供者</h2>
        <p>Provider 按云端默认、用户、平台、设备与运行时分层解析。当前层只保存 override，不会复制继承配置。</p>
      </div>
      <el-button :loading="store.loading" @click="refreshAll">刷新</el-button>
    </div>
    <el-alert type="info" :closable="false" show-icon title="恢复路径、登录和首次引导属于安全壳，不允许扩展替换。云端不可用时自动使用 Last-Known-Good，最终回退 Built-in。" />
    <el-alert v-if="store.usingLastKnownGood" type="warning" :closable="false" show-icon title="当前使用上次验证通过的离线 UI 配置；云端恢复后会自动刷新。" />

    <div class="scope-bar">
      <div>
        <strong>配置层</strong>
        <span>{{ scopeHint }}</span>
      </div>
      <el-select :model-value="store.profileScope" style="width: 220px" @change="changeScope">
        <el-option v-for="item in scopeOptions" :key="item.value" :label="item.label" :value="item.value" />
      </el-select>
      <el-button v-if="store.profileScope !== 'global' && store.scopeExists" type="danger" plain @click="resetScope">清除此层覆盖</el-button>
      <span class="revision">revision {{ store.scopeProfile?.revision ?? 0 }}</span>
    </div>

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
          @change="(value: unknown) => choose(capability, String(value || ''))"
        >
          <el-option label="此层继承（自动解析）" value="" />
          <el-option
            v-for="provider in store.getProviders(capability)"
            :key="provider.providerId"
            :label="providerOptionLabel(provider)"
            :value="provider.providerId"
            :disabled="!provider.enabled"
          />
        </el-select>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useExtensionUIStore } from "@/stores/extensionUI";
import type { UIProviderCapability, UIProviderDefinition, UIProfileScopeKind } from "@/ui-runtime/types";

const store = useExtensionUIStore();
const saving = ref<UIProviderCapability | "">("");
const groups: Array<{ name: string; capabilities: UIProviderCapability[] }> = [
  { name: "应用外壳", capabilities: ["app.shell", "app.navigation", "app.workspace", "route.registry", "page.provider"] },
  { name: "对话", capabilities: ["conversation.shell", "conversation.header", "conversation.messages", "conversation.message_renderer", "conversation.sidebar", "conversation.composer", "conversation.overlay"] },
  { name: "业务页面", capabilities: ["character.shell", "character.detail", "memory.shell", "memory.detail", "settings.shell", "settings.section", "extension.center", "extension.page"] },
  { name: "设计系统", capabilities: ["ui.theme", "ui.tokens", "ui.icons", "ui.components"] },
];
const scopeOptions: Array<{ value: UIProfileScopeKind; label: string }> = [
  { value: "global", label: "云端默认（管理员）" },
  { value: "user", label: "当前用户" },
  { value: "platform", label: "当前平台" },
  { value: "device", label: "当前设备" },
  { value: "device_platform", label: "当前设备 + 平台" },
  { value: "runtime", label: "当前运行时" },
];
const scopeHint = computed(() => {
  const ctx = store.snapshot?.providerContext;
  return [ctx?.runtimeProfile, ctx?.platform, ctx?.deviceId ? `设备 ${ctx.deviceId}` : "未绑定设备"].filter(Boolean).join(" · ");
});

function selection(capability: UIProviderCapability) {
  return store.scopeProfile?.selections?.[capability] ?? "";
}
function providerLabel(capability: UIProviderCapability) {
  const provider = store.getResolvedProvider(capability);
  const inherited = !selection(capability) ? " · 当前层继承" : "";
  return provider ? `有效：${provider.providerId}${inherited}` : "有效：无可用 Provider";
}
function providerOptionLabel(provider: UIProviderDefinition) {
  const placement = provider.placement && provider.placement !== "any" ? ` · ${provider.placement}` : "";
  return `${provider.providerId}${provider.builtin ? " · Built-in" : ""}${placement}`;
}
async function refreshAll() {
  await store.refreshSnapshot(true);
  await store.loadProfileScope(store.profileScope).catch(() => {});
}
async function changeScope(value: UIProfileScopeKind) {
  try {
    await store.loadProfileScope(value);
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "无法加载该配置层");
  }
}
async function resetScope() {
  try {
    await ElMessageBox.confirm("清除后将重新继承上层 UI Profile。", "清除此层覆盖", { type: "warning" });
    await store.resetProfileScope();
    ElMessage.success("此层覆盖已清除");
  } catch (error) {
    if (error === "cancel" || error === "close") return;
    ElMessage.error(error instanceof Error ? error.message : "清除失败");
  }
}
async function choose(capability: UIProviderCapability, providerId: string) {
  saving.value = capability;
  try {
    await store.selectProvider(capability, providerId || undefined, store.profileScope);
    ElMessage.success("界面提供者已更新");
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "更新失败；若为 revision 冲突，请刷新后重试");
  } finally {
    saving.value = "";
  }
}
void refreshAll();
</script>

<style scoped>
.provider-settings { display: grid; gap: 18px; }
.heading { display:flex; justify-content:space-between; align-items:flex-start; gap:16px; }
h2 { margin:0 0 6px; font-size:22px; }
p { margin:0; color:var(--el-text-color-secondary); }
.scope-bar { display:flex; align-items:center; gap:12px; flex-wrap:wrap; padding:12px 14px; border:1px solid var(--el-border-color-light); border-radius:12px; }
.scope-bar > div { display:flex; flex-direction:column; gap:2px; margin-right:auto; }
.scope-bar span { color:var(--el-text-color-secondary); font-size:12px; }
.scope-bar .revision { margin-left:auto; font-family:monospace; }
.group { border:1px solid var(--el-border-color-light); border-radius:12px; overflow:hidden; }
.group h3 { margin:0; padding:12px 16px; background:var(--el-fill-color-light); font-size:14px; }
.row { display:grid; grid-template-columns:minmax(280px,1fr) minmax(260px,420px); gap:16px; align-items:center; padding:12px 16px; border-top:1px solid var(--el-border-color-lighter); }
.capability { display:flex; flex-direction:column; gap:4px; }
.capability span { font-size:12px; color:var(--el-text-color-secondary); }
@media (max-width:760px) { .row { grid-template-columns:1fr; } .scope-bar .revision { margin-left:0; } }
</style>
