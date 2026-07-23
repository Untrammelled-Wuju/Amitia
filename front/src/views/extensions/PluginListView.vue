<template>
  <div class="plugin-page">
    <ExtensionPageHeader
      title="插件"
      description="管理 Plugin Runtime 生命周期、健康状态、权限与扩展协议。"
    >
      <template #actions
        ><el-button :icon="Refresh" :loading="loading" @click="load"
          >刷新</el-button
        ></template
      >
    </ExtensionPageHeader>
    <el-alert
      v-if="loadError"
      :title="loadError"
      type="error"
      show-icon
      :closable="false"
      ><el-button link type="primary" @click="load"
        >重新加载</el-button
      ></el-alert
    >
    <el-card shadow="never" class="table-card">
      <el-table
        v-loading="loading"
        :data="page.items"
        row-key="manifest.metadata.id"
        empty-text="暂无插件"
        stripe
      >
        <el-table-column label="插件" min-width="260">
          <template #default="{ row }"
            ><button
              class="plugin-link"
              type="button"
              @click="open(row.manifest.metadata.id)"
            >
              <strong>{{ row.manifest.metadata.name }}</strong
              ><code>{{ row.manifest.metadata.id }}</code>
            </button></template
          >
        </el-table-column>
        <el-table-column label="版本" width="90"
          ><template #default="{ row }">{{
            row.manifest.metadata.version
          }}</template></el-table-column
        >
        <el-table-column label="生命周期" width="120"
          ><template #default="{ row }"
            ><el-tag :type="lifecycleType(row.lifecycle)">{{
              lifecycleLabel(row.lifecycle)
            }}</el-tag></template
          ></el-table-column
        >
        <el-table-column label="健康" width="110"
          ><template #default="{ row }"
            ><el-tag
              :type="
                row.health === 'healthy'
                  ? 'success'
                  : row.health === 'unknown'
                    ? 'info'
                    : 'warning'
              "
              >{{ healthLabel(row.health) }}</el-tag
            ></template
          ></el-table-column
        >
        <el-table-column label="Hooks" width="80" align="center"
          ><template #default="{ row }">{{
            row.manifest.hooks.length
          }}</template></el-table-column
        >
        <el-table-column label="Skills" width="80" align="center"
          ><template #default="{ row }">{{
            row.manifest.registeredSkills.length
          }}</template></el-table-column
        >
        <el-table-column label="断路器" width="90" align="center"
          ><template #default="{ row }"
            ><span :class="{ danger: row.currentCircuits > 0 }">{{
              row.currentCircuits
            }}</span></template
          ></el-table-column
        >
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }"
            ><div class="actions">
              <el-button @click="open(row.manifest.metadata.id)">详情</el-button
              ><el-button
                :type="row.enabled ? 'danger' : 'primary'"
                plain
                :loading="changing === row.manifest.metadata.id"
                :disabled="!row.compatible && !row.enabled"
                @click="toggle(row)"
                >{{ row.enabled ? "禁用" : "启用" }}</el-button
              >
            </div></template
          >
        </el-table-column>
      </el-table>
      <div class="pagination">
        <el-pagination
          v-model:current-page="currentPage"
          :page-size="page.pageSize"
          :total="page.total"
          layout="total, prev, pager, next"
          @current-change="load"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import { Refresh } from "@element-plus/icons-vue";
import ExtensionPageHeader from "./components/ExtensionPageHeader.vue";
import { fetchPlugins, resolveCharacterId, setPluginEnabled } from "./api";
import type { PluginPage, PluginView } from "./types";

const router = useRouter();
const loading = ref(false);
const loadError = ref("");
const changing = ref("");
const characterId = ref("");
const currentPage = ref(1);
const page = ref<PluginPage>({ items: [], total: 0, page: 1, pageSize: 20 });
async function load() {
  loading.value = true;
  loadError.value = "";
  try {
    page.value = await fetchPlugins(currentPage.value);
  } catch (error: any) {
    loadError.value = problem(error, "插件列表加载失败");
  } finally {
    loading.value = false;
  }
}
function open(id: string) {
  router.push(`/extensions/plugins/${encodeURIComponent(id)}`);
}
async function toggle(plugin: PluginView) {
  if (plugin.enabled)
    await ElMessageBox.confirm(
      `禁用后，“${plugin.manifest.metadata.name}”的 Hook、事件、调度和已注册技能将停止。`,
      "确认禁用插件",
      { type: "warning", confirmButtonText: "禁用", cancelButtonText: "取消" },
    );
  changing.value = plugin.manifest.metadata.id;
  try {
    if (!characterId.value) characterId.value = await resolveCharacterId();
    await setPluginEnabled(
      plugin.manifest.metadata.id,
      !plugin.enabled,
      characterId.value,
    );
    ElMessage.success(plugin.enabled ? "插件已禁用" : "插件已启用");
    await load();
  } catch (error: any) {
    ElMessage.error(problem(error, "操作失败"));
  } finally {
    changing.value = "";
  }
}
function lifecycleType(value: string) {
  return value === "enabled"
    ? "success"
    : value === "error" || value === "circuit_open"
      ? "danger"
      : "info";
}
function lifecycleLabel(value: string) {
  return (
    (
      {
        registered: "已注册",
        loaded: "已加载",
        enabled: "已启用",
        disabled: "已禁用",
        error: "错误",
        circuit_open: "断路",
        unloading: "卸载中",
        unloaded: "已卸载",
      } as Record<string, string>
    )[value] || value
  );
}
function healthLabel(value: string) {
  return (
    (
      {
        healthy: "健康",
        degraded: "降级",
        unhealthy: "异常",
        unknown: "未知",
      } as Record<string, string>
    )[value] || value
  );
}
function problem(error: any, fallback: string) {
  return error?.response?.data?.detail || error?.message || fallback;
}
onMounted(load);
</script>

<style scoped>
.plugin-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}
.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}
.page-header h1 {
  color: var(--ac-color-text);
  font-size: 24px;
  line-height: 32px;
}
.page-header p {
  margin-top: 6px;
  color: var(--ac-color-text-secondary);
  line-height: 1.6;
}
.table-card :deep(.el-card__body) {
  padding: 0;
  overflow-x: auto;
}
.plugin-link {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
  min-height: 44px;
  padding: 4px 0;
  border: 0;
  background: transparent;
  cursor: pointer;
  text-align: left;
}
.plugin-link code {
  color: var(--ac-color-text-muted);
  font-size: var(--ac-font-size-xs);
}
.plugin-link:focus-visible {
  outline: 2px solid var(--ac-color-primary);
  outline-offset: 2px;
}
.actions {
  display: flex;
  gap: 8px;
}
.danger {
  color: var(--el-color-danger);
  font-weight: 700;
}
.pagination {
  display: flex;
  justify-content: flex-end;
  padding: 16px;
}
@media (max-width: 720px) {
  .page-header {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
