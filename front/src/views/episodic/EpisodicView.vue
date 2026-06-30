<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="episodic-page">
    <div class="page-header">
      <h2>情景记忆</h2>
      <el-select v-model="filterType" placeholder="全部类型" clearable size="small" style="width:150px" @change="onFilterChange">
        <el-option label="全部类型" value="" />
        <el-option v-for="(label, key) in typeMap" :key="key" :label="label" :value="key" />
      </el-select>
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
            <el-tag v-if="m.triggerKeywords" size="small" type="info">{{ m.triggerKeywords }}</el-tag>
            <span class="time">{{ m.createdAt }}</span>
            <el-button size="small" text type="danger" @click.stop="handleDelete(m.id)">删除</el-button>
          </div>
        </div>
      </div>

      <div v-if="memories.length === 0" class="empty">暂无情景记忆</div>
    </div>

    <el-dialog v-model="drawerVisible" title="情景详情" width="520px" align-center @close="detailMemory = null" destroy-on-close>
      <template v-if="detailMemory">
        <br>
        <h3>{{ sceneEmoji(detailMemory.sceneType) }} {{ detailMemory.title }}</h3>
        <br>
        <p class="detail-content">{{ detailMemory.content }}</p>
        <br>
        <div class="detail-meta">
          <el-tag size="small">{{ sceneLabel(detailMemory.sceneType) }}</el-tag>
          <el-tag size="small" type="warning">情感 {{ detailMemory.sentimentScore }}</el-tag>
          <el-tag v-if="detailMemory.triggerKeywords" size="small" type="info">{{ detailMemory.triggerKeywords }}</el-tag>
        </div>
        <div v-if="detailMessages.length > 0" class="context-bubbles">
          <h4>对话上下文</h4>
          <div v-for="msg in detailMessages" :key="msg.id" class="context-bubble" :class="'role-' + msg.role">
            <span class="bubble-role">{{ msg.role === 'user' ? '用户' : 'AI' }}</span>
            <span class="bubble-text">{{ msg.content }}</span>
          </div>
        </div>
      </template>
      <template #footer>
        <el-button type="primary" @click="drawerVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue"
import { ElMessageBox } from "element-plus"
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
const drawerVisible = ref(false)
const detailMemory = ref<EpisodicMemory | null>(null)
const detailMessages = ref<any[]>([])

onMounted(() => { fetchMemories() })

function onFilterChange() {
  fetchMemories({ sceneType: filterType.value || undefined })
}

async function showDetail(m: EpisodicMemory) {
  detailMemory.value = m
  detailMessages.value = []
  drawerVisible.value = true
  try {
    const data = await getDetail(m.id)
    detailMessages.value = data.messages || []
  } catch { detailMessages.value = [] }
}

async function handleDelete(id: string) {
  try {
    await ElMessageBox.confirm("确定删除这条情景记忆？", "删除确认", {
      confirmButtonText: "确定",
      cancelButtonText: "取消",
      type: "warning",
    })
    await deleteMemory(id)
  } catch {}
}
</script>

<style scoped>
.episodic-page { padding: 24px; max-width: 800px; margin: 0 auto; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px; }
.page-header h2 { margin: 0; font-size: 24px; }

.timeline { position: relative; padding-left: 24px; }
.timeline::before { content: ''; position: absolute; left: 8px; top: 0; bottom: 0; width: 2px; background: #e0e0e0; }
.timeline-item { position: relative; margin-bottom: 20px; cursor: pointer; display: flex; gap: 16px; }
.timeline-marker { width: 12px; height: 12px; border-radius: 50%; border: 2px solid var(--ac-color-text-primary); box-shadow: 0 0 0 2px #e0e0e0; flex-shrink: 0; margin-top: 4px; }
.timeline-content { background: var(--ac-color-bg-secondary); border-radius: 10px; padding: 14px; box-shadow: 0 1px 4px rgba(0,0,0,0.06); flex: 1; }
.item-header { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; color: ; }
.scene-emoji { font-size: 16px; }
.scene-type { font-size: 12px; color: var(--ac-color-text-primary); }

.item-title { font-size: 16px; font-weight: 600; margin-bottom: 4px; color: var(--ac-color-text-primary); }
.item-content { font-size: 14px; color: var(--ac-color-text-secondary); margin-bottom: 8px; }
.item-footer { display: flex; align-items: center; gap: 12px; font-size: 12px; color: var(--ac-color-text-primary); }


.loading, .empty { text-align: center; padding: 48px; color: #999; }
</style>