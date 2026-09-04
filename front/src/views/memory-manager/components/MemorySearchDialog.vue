<!-- SPDX-FileCopyrightText: 2026 Peng Xu -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<template>
  <el-dialog v-model="visible" title="记忆搜索" width="560px" @closed="reset">
    <div class="search-controls">
      <el-select v-model="searchMode" style="width: 116px">
        <el-option label="混合" value="hybrid" />
        <el-option label="向量" value="vector" />
        <el-option label="关键词" value="keyword" />
      </el-select>
      <el-input
        v-model="searchQuery"
        placeholder="输入搜索词..."
        clearable
        @keyup.enter="doSearch"
      />
      <el-button type="primary" :loading="searching" @click="doSearch">搜索</el-button>
    </div>

    <div class="search-hint">
      {{ modeHint }}
    </div>

    <div class="search-results" v-loading="searching">
      <div v-for="result in searchResults" :key="result.id || result.key" class="search-result-item">
        <div class="sri-header">
          <div class="sri-tags">
            <el-tag size="small">{{ typeLabel(result.memoryType) }}</el-tag>
            <el-tag size="small" type="info" effect="plain">{{ matchLabel(result.matchType) }}</el-tag>
            <el-tag v-if="result.memoryLayer" size="small" type="info" effect="plain">{{ result.memoryLayer }}</el-tag>
          </div>
          <span v-if="searchMode !== 'keyword'" class="sri-score">
            Score: {{ (result.score * 100).toFixed(1) }}%
          </span>
        </div>
        <div class="sri-key">{{ result.key }}</div>
        <div class="sri-value">{{ result.value }}</div>
      </div>
      <el-empty v-if="searchResults.length === 0 && searched && !searching" description="无结果" />
      <div v-if="!searched && !searching" class="search-placeholder">输入关键词开始搜索</div>
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { ElMessage } from "element-plus";
import {
  searchMemories,
  type MemorySearchMode,
  type MemorySearchResult,
} from "../composables/memorySearchApi";

const props = defineProps<{ modelValue: boolean }>();
const emit = defineEmits<{ "update:modelValue": [value: boolean] }>();
const visible = computed({
  get: () => props.modelValue,
  set: (value) => emit("update:modelValue", value),
});

const searchQuery = ref("");
const searchMode = ref<MemorySearchMode>("hybrid");
const searchResults = ref<MemorySearchResult[]>([]);
const searched = ref(false);
const searching = ref(false);

const TYPES = [
  { value: "personal_info", label: "个人信息" },
  { value: "hobby", label: "爱好" },
  { value: "preference", label: "偏好" },
  { value: "fact", label: "事实" },
  { value: "plan", label: "计划" },
  { value: "habit", label: "习惯" },
  { value: "relationship", label: "关系" },
  { value: "custom", label: "自定义" },
];

const modeHint = computed(() => {
  if (searchMode.value === "vector") return "向量搜索只按语义相似度召回。";
  if (searchMode.value === "keyword") return "关键词搜索使用结构化记忆的普通检索能力。";
  return "混合搜索综合向量、关键词与记忆检索评分。";
});

function typeLabel(type: string) {
  return TYPES.find((item) => item.value === type)?.label || type;
}

function matchLabel(matchType: string) {
  const value = String(matchType || searchMode.value).toLowerCase();
  if (value.includes("vector")) return "向量";
  if (value.includes("keyword")) return "关键词";
  return "混合";
}

async function doSearch() {
  const query = searchQuery.value.trim();
  if (!query || searching.value) return;
  searching.value = true;
  try {
    searchResults.value = await searchMemories(query, searchMode.value, 20);
    searched.value = true;
  } catch (e: any) {
    searchResults.value = [];
    searched.value = true;
    ElMessage.error(e?.message || "记忆搜索失败");
  } finally {
    searching.value = false;
  }
}

function reset() {
  searchQuery.value = "";
  searchResults.value = [];
  searched.value = false;
  searching.value = false;
}
</script>

<style scoped>
.search-controls {
  display: flex;
  gap: 8px;
}
.search-hint {
  margin-top: 8px;
  color: var(--ac-color-text-muted);
  font-size: var(--ac-font-size-xs);
}
.search-results {
  min-height: 160px;
  max-height: 360px;
  overflow-y: auto;
  margin-top: 12px;
}
.search-result-item {
  padding: 10px;
  border-bottom: 1px solid var(--ac-color-border-light);
}
.search-result-item:last-child {
  border-bottom: none;
}
.sri-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 5px;
}
.sri-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
}
.sri-score {
  flex-shrink: 0;
  color: var(--ac-color-primary);
  font-size: var(--ac-font-size-xs);
  font-weight: 600;
}
.sri-key {
  font-size: var(--ac-font-size-sm);
  font-weight: 600;
}
.sri-value {
  margin-top: 2px;
  color: var(--ac-color-text-secondary);
  font-size: var(--ac-font-size-sm);
  white-space: pre-wrap;
}
.search-placeholder {
  padding: 28px;
  color: var(--ac-color-text-muted);
  text-align: center;
}
@media (max-width: 640px) {
  .search-controls {
    flex-wrap: wrap;
  }
  .search-controls .el-input {
    min-width: 100%;
  }
}
</style>
