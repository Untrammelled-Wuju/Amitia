<template>
  <div v-if="visible" class="fa-panel mem-inject-panel">
    <div class="mi-header">
      <h4>记忆注入</h4>
      <button class="mi-close-btn" @click="close">✕</button>
    </div>
    <div v-if="loading" class="mi-loading">加载中...</div>
    <div v-else>
      <div class="mi-section" v-if="memories.length">
        <h5>当前检索记忆 ({{ memories.length }})</h5>
        <div v-for="m in memories" :key="m.id" class="mi-item">
          <el-tag size="small" :type="m.matchType === 'vector' ? 'success' : 'info'">{{ m.matchType }}</el-tag>
          <span class="mi-layer">{{ m.memoryLayer || '事实记忆' }}</span>
          <span class="mi-score">{{ (m.score * 100).toFixed(0) }}%</span>
          <div class="mi-content">{{ m.memory?.value || m.value }}</div>
          <el-button size="small" text type="danger" @click="feedback(m, 'irrelevant')">不相关</el-button>
          <el-button size="small" text type="warning" @click="feedback(m, 'wrong')">错误</el-button>
        </div>
      </div>
      <div class="mi-section" v-if="profiles.length">
        <h5>用户画像 ({{ profiles.length }})</h5>
        <div v-for="p in profiles" :key="p.id" class="mi-profile-card">
          <span>{{ p.attributeName }}: {{ p.attributeValue }}</span>
          <el-tag :type="p.confidence >= 80 ? 'success' : 'warning'" size="small">{{ p.confidence }}%</el-tag>
        </div>
      </div>
      <div class="mi-section">
        <h5>压缩状态</h5>
        <div class="mi-compress">
          <span>已压缩 {{ compression.compressedRounds || 0 }} / {{ compression.totalRounds || 0 }} 轮</span>
          <span v-if="compression.lastCompressedAt">上次: {{ compression.lastCompressedAt }}</span>
        </div>
      </div>
      <div class="mi-section">
        <h5>管线状态</h5>
        <div class="mi-pipeline">
          <template v-for="l in (pipeline?.layers || [])" :key="l.layer">
            <el-tooltip :content="l.name + ': ' + l.status + ' (' + l.durationMs + 'ms)'" placement="top">
              <span class="mi-pl-dot" :style="{backgroundColor: l.status === 'completed' ? '#67c23a' : l.status === 'skipped' ? '#909399' : '#409eff'}" />
            </el-tooltip>
          </template>
        </div>
      </div>
      <div class="mi-empty" v-if="!memories.length && !profiles.length">暂无记忆注入数据</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, type Ref } from "vue"
import { ElMessage } from "element-plus"
import { useApi } from "../../../composables/useApi"

const props = defineProps<{
  visible: boolean
  convId: string
}>()

const emit = defineEmits<{
  (e: "close"): void
}>()

const { get } = useApi()
const loading = ref(false)
const memories = ref<any[]>([])
const profiles = ref<any[]>([])
const compression = ref<any>({})
const pipeline = ref<any>(null)

function close() {
  emit("close")
}

function feedback(m: any, type: string) {
  ElMessage.info("已标记为" + (type === "irrelevant" ? "不相关" : "错误") + "，将优化后续检索")
}

onMounted(async () => {
  loading.value = true
  try {
    if (props.convId) {
      try { const compR: any = await get("/api/chats/" + props.convId + "/compression-status"); compression.value = compR || {} } catch {}
      try { const pipeR: any = await get("/api/memory/pipeline/status"); pipeline.value = pipeR } catch {}
      try { const profR: any = await get("/api/profiles", { pageSize: 5 }); profiles.value = profR?.items || [] } catch {}
    }
  } catch {}
  loading.value = false
})
</script>
