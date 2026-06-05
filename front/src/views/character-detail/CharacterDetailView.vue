<template>
  <div class="char-detail">
    <div class="detail-header">
      <el-button text @click="$router.push('/character')">
        <el-icon><ArrowLeft /></el-icon> 返回角色列表
      </el-button>
      <h2 v-if="character">{{ character.name }}</h2>
    </div>

    <el-tabs :model-value="activeTab" @tab-change="onTabChange" type="border-card">
      <el-tab-pane label="生活规则" name="life-rules">
        <AiCharacterSettingsView v-if="activeTab==='life-rules'" />
      </el-tab-pane>
      <el-tab-pane label="记忆管理" name="memory">
        <MemoryManagerView v-if="activeTab==='memory'" />
      </el-tab-pane>
      <el-tab-pane label="记忆时间线" name="timeline">
        <MemoryTimelineView v-if="activeTab==='timeline'" />
      </el-tab-pane>
      <el-tab-pane label="主动消息" name="proactive">
        <ProactiveRulesView v-if="activeTab==='proactive'" />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, provide } from "vue"
import { useRouter, useRoute } from "vue-router"
import { ArrowLeft } from "@element-plus/icons-vue"
import { apiClient } from "../../ui-index"
import { AiCharacterSettingsView, MemoryManagerView, MemoryTimelineView, ProactiveRulesView } from "../../ui-index"

const router = useRouter()
const route = useRoute()
const character = ref<any>(null)
const currentId = ref(route.params.id as string)
provide("currentCharacterId", currentId)

const activeTab = computed(() => {
  const p = route.path
  if (p.endsWith("/memory")) return "memory"
  if (p.endsWith("/timeline")) return "timeline"
  if (p.endsWith("/proactive")) return "proactive"
  return "life-rules"
})

onMounted(async () => {
  const id = route.params.id as string
  try {
    const r = await apiClient.get(`/api/characters/${id}`)
    character.value = r.data?.data || r.data
  } catch {}
})

function onTabChange(tab: string) {
  const id = route.params.id as string
  router.push(`/character/${id}/${tab}`)
}
</script>

<style scoped>
.char-detail { padding: 0; }
.detail-header { display:flex; align-items:center; gap:12px; margin-bottom:12px; }
.detail-header h2 { font-size:18px; font-weight:600; margin:0; }
</style>
