<template>
  <div class="episodic-page">
    <div class="page-header">
      <h2>情景记忆</h2>
      <select v-model="filterType" @change="onFilterChange">
        <option value="">全部类型</option>
        <option v-for="(label, key) in typeMap" :key="key" :value="key">{{ label }}</option>
      </select>
    </div>

    <div v-if="loading" class="loading">加载中...</div>

    <div v-else class="timeline">
      <div v-for="m in memories" :key="m.id" class="timeline-item" @click="showDetail(m)">
        <div class="timeline-marker" :style="{ background: sentimentColor(m.sentimentScore) }"></div>
        <div class="timeline-content">
          <div class="item-header">
            <span class="scene-emoji">{{ sceneEmoji(m.sceneType) }}</span>
            <span class="scene-type">{{ sceneLabel(m.sceneType) }}</span>
            <span class="sentiment-badge" :style="{ background: sentimentColor(m.sentimentScore) }">
              {{ m.sentimentScore > 0 ? '+' : '' }}{{ m.sentimentScore }}
            </span>
          </div>
          <div class="item-title">{{ m.title }}</div>
          <div class="item-content">{{ m.content }}</div>
          <div class="item-footer">
            <span v-if="m.triggerKeywords" class="keywords">{{ m.triggerKeywords }}</span>
            <span class="time">{{ m.createdAt }}</span>
            <button class="btn-del" @click.stop="handleDelete(m.id)">删除</button>
          </div>
        </div>
      </div>

      <div v-if="memories.length === 0" class="empty">暂无情景记忆</div>
    </div>

    <div v-if="detailMemory" class="modal-overlay" @click.self="detailMemory = null">
      <div class="modal modal-detail">
        <h3>{{ sceneEmoji(detailMemory.sceneType) }} {{ detailMemory.title }}</h3>
        <p class="detail-content">{{ detailMemory.content }}</p>
        <div class="detail-meta">
          <span>类型: {{ sceneLabel(detailMemory.sceneType) }}</span>
          <span>情感: {{ detailMemory.sentimentScore }}</span>
          <span v-if="detailMemory.triggerKeywords">触发词: {{ detailMemory.triggerKeywords }}</span>
        </div>
        <div v-if="detailMessages.length > 0" class="context-bubbles">
          <h4>对话上下文</h4>
          <div v-for="msg in detailMessages" :key="msg.id" class="context-bubble" :class="'role-' + msg.role">
            <span class="bubble-role">{{ msg.role === 'user' ? '用户' : 'AI' }}</span>
            <span class="bubble-text">{{ msg.content }}</span>
          </div>
        </div>
        <button class="btn" @click="detailMemory = null">关闭</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue"
import { useEpisodic, type EpisodicMemory } from "@/composables/useEpisodic"

const {
  memories, loading,
  fetchMemories, deleteMemory, getDetail,
  sceneLabel, sceneEmoji, sentimentColor,
} = useEpisodic()

const typeMap: Record<string, string> = {
  insight: "💡 感悟", joke: "😂 笑话", milestone: "🏆 里程碑",
  emotional_peak: "💗 情感峰值", confession: "🗣️ 坦白",
}

const filterType = ref("")
const detailMemory = ref<EpisodicMemory | null>(null)
const detailMessages = ref<any[]>([])

onMounted(() => { fetchMemories() })

function onFilterChange() {
  fetchMemories({ sceneType: filterType.value || undefined })
}

async function showDetail(m: EpisodicMemory) {
  detailMemory.value = m
  try {
    const data = await getDetail(m.id)
    detailMessages.value = data.messages || []
  } catch { detailMessages.value = [] }
}

async function handleDelete(id: string) {
  if (confirm("确定删除？")) { await deleteMemory(id) }
}
</script>

<style scoped>
.episodic-page { padding: 24px; max-width: 800px; margin: 0 auto; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px; }
.page-header h2 { margin: 0; font-size: 24px; }
.page-header select { padding: 8px 12px; border: 1px solid #ddd; border-radius: 6px; }
.timeline { position: relative; padding-left: 24px; }
.timeline::before { content: ''; position: absolute; left: 8px; top: 0; bottom: 0; width: 2px; background: #e0e0e0; }
.timeline-item { position: relative; margin-bottom: 20px; cursor: pointer; display: flex; gap: 16px; }
.timeline-marker { width: 12px; height: 12px; border-radius: 50%; border: 2px solid #fff; box-shadow: 0 0 0 2px #e0e0e0; flex-shrink: 0; margin-top: 4px; }
.timeline-content { background: #fff; border-radius: 10px; padding: 14px; box-shadow: 0 1px 4px rgba(0,0,0,0.06); flex: 1; }
.item-header { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; }
.scene-emoji { font-size: 16px; }
.scene-type { font-size: 12px; color: #999; }
.sentiment-badge { font-size: 11px; color: #fff; padding: 1px 6px; border-radius: 10px; }
.item-title { font-size: 16px; font-weight: 600; margin-bottom: 4px; }
.item-content { font-size: 14px; color: #555; margin-bottom: 8px; }
.item-footer { display: flex; align-items: center; gap: 12px; font-size: 12px; color: #bbb; }
.keywords { color: #1976d2; }
.btn-del { background: none; border: none; color: #f44336; cursor: pointer; font-size: 12px; }
.loading, .empty { text-align: center; padding: 48px; color: #999; }
.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.4); display: flex; justify-content: center; align-items: center; z-index: 1000; }
.modal { background: #fff; border-radius: 12px; padding: 24px; max-width: 600px; width: 90vw; max-height: 80vh; overflow-y: auto; }
.modal-detail h3 { margin: 0 0 12px; }
.detail-content { color: #555; line-height: 1.6; margin-bottom: 12px; }
.detail-meta { display: flex; gap: 16px; font-size: 12px; color: #999; margin-bottom: 16px; }
.context-bubbles { border-top: 1px solid #eee; padding-top: 12px; }
.context-bubbles h4 { font-size: 14px; margin: 0 0 8px; }
.context-bubble { padding: 8px 12px; border-radius: 8px; margin-bottom: 8px; font-size: 13px; }
.role-user { background: #e3f2fd; }
.role-assistant { background: #f5f5f5; }
.bubble-role { font-weight: 600; margin-right: 8px; font-size: 11px; color: #999; }
.bubble-text { color: #333; }
.btn { padding: 8px 16px; border: 1px solid #ddd; border-radius: 6px; background: #fff; cursor: pointer; margin-top: 12px; }
</style>