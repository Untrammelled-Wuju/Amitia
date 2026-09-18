<!-- SPDX-FileCopyrightText: 2026 Peng Xu -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<template>
  <el-dialog v-model="visible" title="检索排序诊断" width="720px" destroy-on-close>
    <div class="ranked-toolbar">
      <el-input
        v-model="query"
        clearable
        placeholder="输入用于动态召回的查询文本"
        @keyup.enter="loadRanked"
      />
      <el-button type="primary" :loading="loading" @click="loadRanked">检索</el-button>
    </div>

    <el-alert
      type="info"
      :closable="false"
      title="展示最终分数及向量/关键词分量，用于核对动态记忆召回顺序。"
      style="margin: 12px 0"
    />

    <el-empty v-if="!loading && rows.length === 0" description="暂无排序结果" :image-size="56" />
    <el-table v-else :data="rows" v-loading="loading" max-height="440" size="small">
      <el-table-column label="记忆" min-width="280">
        <template #default="scope">
          <div class="memory-key" v-if="scope.row.memory?.key">{{ scope.row.memory.key }}</div>
          <div class="memory-value">{{ scope.row.memory?.value || "-" }}</div>
        </template>
      </el-table-column>
      <el-table-column label="最终" width="82" align="right">
        <template #default="scope">{{ score(scope.row.finalScore) }}</template>
      </el-table-column>
      <el-table-column label="向量" width="82" align="right">
        <template #default="scope">{{ score(scope.row.vectorScore) }}</template>
      </el-table-column>
      <el-table-column label="关键词" width="82" align="right">
        <template #default="scope">{{ score(scope.row.keywordScore) }}</template>
      </el-table-column>
      <el-table-column label="重要度" width="82" align="right">
        <template #default="scope">{{ score(scope.row.importanceNorm) }}</template>
      </el-table-column>
      <el-table-column label="时间增益" width="88" align="right">
        <template #default="scope">{{ score(scope.row.temporalBoost) }}</template>
      </el-table-column>
    </el-table>

    <template #footer>
      <el-button @click="visible = false">关闭</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { useApi } from "../../../composables/useApi";

const props = defineProps<{
  modelValue: boolean;
  characterId?: string;
  initialQuery?: string;
}>();
const emit = defineEmits<{ "update:modelValue": [value: boolean] }>();
const { get } = useApi();
const query = ref("");
const loading = ref(false);
const rows = ref<any[]>([]);
let requestVersion = 0;

const visible = computed({
  get: () => props.modelValue,
  set: (value) => emit("update:modelValue", value),
});

function score(value: unknown) {
  const number = Number(value);
  return Number.isFinite(number) ? number.toFixed(4) : "-";
}

async function loadRanked() {
  const version = ++requestVersion;
  loading.value = true;
  try {
    const result = await get<any[]>("/api/memories/ranked", {
      characterId: props.characterId || undefined,
      query: query.value.trim() || undefined,
      limit: 30,
    });
    if (version !== requestVersion) return;
    rows.value = Array.isArray(result) ? result : [];
  } catch (error: any) {
    if (version !== requestVersion) return;
    rows.value = [];
    ElMessage.error(error?.message || "检索排序加载失败");
  } finally {
    if (version === requestVersion) loading.value = false;
  }
}

watch(
  () => props.modelValue,
  (opened) => {
    if (!opened) return;
    query.value = props.initialQuery || "";
    void loadRanked();
  },
);
</script>

<style scoped>
.ranked-toolbar {
  display: flex;
  gap: 8px;
}
.memory-key {
  color: var(--ac-color-text-primary);
  font-size: var(--ac-font-size-sm);
  font-weight: 600;
}
.memory-value {
  margin-top: 2px;
  color: var(--ac-color-text-secondary);
  font-size: var(--ac-font-size-xs);
  line-height: 1.45;
  white-space: normal;
  word-break: break-word;
}
</style>
