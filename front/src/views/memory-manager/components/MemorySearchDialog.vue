<!-- SPDX-FileCopyrightText: 2026 Peng Xu -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<template>
    <el-dialog v-model="visible" title="语义搜索" width="500px">
      <el-input v-model="searchQuery" placeholder="输入搜索词..." @keyup.enter="doSearch" />
      <div style="margin-top:12px;max-height:300px;overflow-y:auto">
        <div v-for="r in searchResults" :key="r.id" class="search-result-item">
          <div class="sri-header">
            <el-tag size="small">{{ typeLabel(r.memoryType) }}</el-tag>
            <span class="sri-score">Score: {{ (r.score * 100).toFixed(1) }}%</span>
          </div>
          <div class="sri-key">{{ r.key }}</div>
          <div class="sri-value">{{ r.value }}</div>
        </div>
        <el-empty v-if="searchResults.length === 0 && searched" description="无结果" />
        <div v-if="!searched" style="color:var(--ac-color-text-muted);text-align:center;padding:20px">
          输入关键词进行语义搜索
        </div>
      </div>
    </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed } from "vue"
import { ElMessage } from "element-plus"
import { useApi } from "../../../composables/useApi"
const { post, get } = useApi()
const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{ "update:modelValue": [value: boolean] }>()
const visible = computed({ get: () => props.modelValue, set: (v) => emit("update:modelValue", v) })
const searchQuery = ref("")
const searchResults = ref<any[]>([])
const searched = ref(false)
const TYPES = [{ value: "custom", label: "自定义" }, { value: "fact", label: "事实" }, { value: "preference", label: "偏好" }, { value: "experience", label: "经历" }, { value: "rule", label: "规则" }, { value: "belief", label: "信念" }, { value: "emotion", label: "情感" }, { value: "skill", label: "技能" }]
function typeLabel(t: string) { return TYPES.find(x => x.value === t)?.label || t }
async function doSearch() {
  if (!searchQuery.value.trim()) return
  try {
    const result = await post<any>("/api/memories/hybrid-search", {
      keyword: searchQuery.value.trim(),
      limit: 10,
    })
    const items = result?.items || []
    searchResults.value = items.map((r: any) => ({
      id: r.memory?.id || r.id,
      key: r.memory?.key || r.key,
      value: r.memory?.value || r.value,
      memoryType: r.memory?.memoryType || r.memoryType,
      score: r.score ?? 0,
      matchType: r.matchType || "hybrid",
      memoryLayer: r.memoryLayer || "",
    }))
    searched.value = true
  } catch {
    try {
      const result = await post<any>("/api/memories/search", {
        keyword: searchQuery.value.trim(),
        limit: 10,
      })
      searchResults.value = (result?.items || []).map((r: any) => ({ ...r, score: 0 }))
      searched.value = true
    } catch {
      searchResults.value = []
      searched.value = true
    }
  }
}
</script>

<style scoped>
.search-result-item { padding: 8px 10px; border-bottom: 1px solid var(--ac-color-border-light); }
.search-result-item:last-child { border-bottom: none; }
.sri-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 4px; }
.sri-score { font-size: var(--ac-font-size-xs); color: var(--ac-color-primary); font-weight: 600; }
.sri-key { font-weight: 600; font-size: var(--ac-font-size-sm); }
.sri-value { font-size: var(--ac-font-size-sm); color: var(--ac-color-text-secondary); }
</style>
