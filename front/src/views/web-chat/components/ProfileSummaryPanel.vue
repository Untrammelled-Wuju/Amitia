<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div v-if="visible" class="fa-panel profile-summary-panel">
    <div class="profile-panel-header">
      <h4>用户画像摘要</h4>
      <button class="profile-close-btn" @click="close">✕</button>
    </div>
    <div v-if="loading" class="profile-loading">加载中...</div>
    <div v-else-if="items.length === 0" class="profile-empty">暂无画像</div>
    <div v-else class="profile-items">
      <div v-for="p in items" :key="p.id" class="profile-item">
        <span class="profile-cat">{{ categoryLabel(p.category) }}</span>
        <span class="profile-name">{{ p.attributeName }}</span>
        <span class="profile-val">{{ p.attributeValue }}</span>
        <span class="profile-conf" :class="confClass(p.confidence)">{{ p.confidence }}%</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue"
import { useProfile } from "@/composables/useProfile"

defineProps<{
  visible: boolean
}>()

const emit = defineEmits<{
  (e: "close"): void
}>()

const { profiles: profData, fetchProfiles, categoryLabel } = useProfile()

const loading = ref(false)
const items = ref<any[]>([])

const PROFILE_CAT_MAP: Record<string, string> = {
  personal_info: "个人信息", preference: "偏好", habit: "习惯",
  fear: "恐惧", relationship: "关系", health: "健康", plan: "计划",
}

function catLabel(cat: string): string {
  return PROFILE_CAT_MAP[cat] || cat
}

function confClass(c: number): string {
  if (c >= 80) return "conf-high"
  if (c >= 50) return "conf-mid"
  return "conf-low"
}

function close() {
  emit("close")
}

onMounted(async () => {
  loading.value = true
  try {
    await fetchProfiles({ pageSize: 10 })
    items.value = profData.value
  } catch { } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.fa-panel {
  position: absolute;
  right: 16px;
  top: 52px;
  z-index: 19;
  background: var(--ac-color-surface, #fff);
  border-radius: 12px;
  box-shadow: 0 4px 20px rgba(0,0,0,0.12);
  overflow-y: auto;
}
.profile-summary-panel {
  width: 280px; max-height: 60vh;
}
.profile-panel-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: 12px 16px; border-bottom: 1px solid var(--ac-color-border-light);
}
.profile-panel-header h4 { margin: 0; font-size: 15px; }
.profile-close-btn { background: none; border: none; font-size: 16px; cursor: pointer; color: var(--ac-color-text-muted); }
.profile-loading, .profile-empty { padding: 24px; text-align: center; color: var(--ac-color-text-muted); font-size: 13px; }
.profile-items { padding: 8px 0; }
.profile-item {
  display: flex; align-items: center; gap: 8px; padding: 6px 16px;
  font-size: 13px; border-bottom: 1px solid var(--ac-color-border-light);
}
.profile-cat { color: var(--ac-color-text-muted); font-size: 11px; min-width: 48px; }
.profile-name { color: var(--ac-color-text-secondary); min-width: 48px; }
.profile-val { color: var(--ac-color-text); flex: 1; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.profile-conf { font-size: 11px; font-weight: 600; min-width: 36px; text-align: right; }
.conf-high { color: var(--ac-color-success); }
.conf-mid { color: var(--ac-color-warning); }
.conf-low { color: var(--ac-color-danger); }
</style>