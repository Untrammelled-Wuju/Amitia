<!-- SPDX-FileCopyrightText: 2026 Peng Xu -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<template>
  <div class="memory-status-panel">
    <div class="vector-index-bar" v-if="vectorStatus">
      <div class="vib-info">
        <span class="vib-label">向量索引:</span>
        <el-tag :type="vectorStatus.enabled ? 'success' : 'info'" size="small">
          {{ vectorStatus.enabled ? "已启用" : "已禁用" }}
        </el-tag>
        <span class="vib-provider" v-if="vectorStatus.enabled">
          Provider: {{ vectorStatus.providerName }} | 总向量:
          {{ vectorStatus.totalEmbeddings || vectorStatus.totalEmbedded || 0 }}
        </span>
        <span class="vib-time" v-if="vectorStatus.lastRebuildAt">
          最近重建: {{ fmtDate(vectorStatus.lastRebuildAt) }}
        </span>
      </div>
      <div class="vib-actions">
        <el-button size="small" @click="rebuildIndex" :loading="rebuilding">
          {{ rebuilding ? "重建中..." : "重建索引" }}
        </el-button>
        <el-button size="small" @click="rebuildEmbeddings" :loading="rebuilding">
          重建向量嵌入
        </el-button>
        <el-button
          size="small"
          @click="searchDialogVisible = true"
          :disabled="!vectorStatus.enabled"
        >
          语义搜索
        </el-button>
      </div>
      <el-table
        v-if="vectorStatus.collections && vectorStatus.collections.length"
        :data="vectorStatus.collections"
        size="small"
        class="vector-collection-table"
      >
        <el-table-column prop="label" label="层级" min-width="100" />
        <el-table-column
          prop="name"
          label="Collection"
          min-width="160"
          show-overflow-tooltip
        />
        <el-table-column label="向量数" width="90">
          <template #default="{ row }">{{ row.totalEmbeddings || 0 }}</template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag
              size="small"
              :type="
                row.status === 'ready'
                  ? 'success'
                  : row.status === 'error'
                    ? 'danger'
                    : 'info'
              "
            >
              {{
                row.status === "ready"
                  ? "正常"
                  : row.status === "error"
                    ? "异常"
                    : "未启用"
              }}
            </el-tag>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <MemorySearchDialog v-model="searchDialogVisible" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { ElMessage } from "element-plus";
import { useApi } from "../../../composables/useApi";
import MemorySearchDialog from "./MemorySearchDialog.vue";

const { get, post } = useApi();
const vectorStatus = ref<any>(null);
const rebuilding = ref(false);
const searchDialogVisible = ref(false);

async function loadVectorStatus() {
  try {
    vectorStatus.value = await get<any>("/api/memories/vector-status");
  } catch {}
}

async function rebuildIndex() {
  rebuilding.value = true;
  try {
    const result = await post<any>("/api/memories/rebuild-index", {});
    ElMessage.success(
      "索引重建完成：" +
        (result.embedded ?? result.totalEmbedded ?? 0) +
        " 条记忆已处理",
    );
    await loadVectorStatus();
  } catch (err: any) {
    ElMessage.error(err?.message || "Rebuild failed");
  } finally {
    rebuilding.value = false;
  }
}

async function rebuildEmbeddings() {
  rebuilding.value = true;
  try {
    const result = await post<any>("/api/memories/rebuild-embeddings", {});
    ElMessage.success(
      "向量嵌入重建完成：" +
        (result.embedded ?? result.totalEmbedded ?? result.total ?? 0) +
        " 条记忆已处理",
    );
    await loadVectorStatus();
  } catch (err: any) {
    ElMessage.error(err?.message || "Rebuild embeddings failed");
  } finally {
    rebuilding.value = false;
  }
}

function fmtDate(d: string) {
  if (!d) return "";
  try {
    return new Date(d).toLocaleString("zh-CN");
  } catch {
    return d;
  }
}

onMounted(loadVectorStatus);
</script>

<style scoped>
.vector-index-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  margin: 10px 0;
  border-radius: var(--ac-radius-sm);
  background: var(--ac-color-bg-secondary);
  border: 1px solid var(--ac-color-border-light);
  flex-wrap: wrap;
  gap: 8px;
}
.vib-info {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.vib-label {
  font-weight: 600;
  font-size: var(--ac-font-size-sm);
}
.vib-provider {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
}
.vib-time {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
}
.vib-actions {
  display: flex;
  gap: 6px;
}
.vector-collection-table {
  width: 100%;
  margin-top: 8px;
}
</style>
