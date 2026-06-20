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
