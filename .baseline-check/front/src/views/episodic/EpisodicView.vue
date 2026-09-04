<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="episodic-page">
    <div class="page-header">
      <h2>情景记忆</h2>
      <div class="header-controls">
        <el-select
          v-model="filterType"
          placeholder="全部类型"
          clearable
          size="small"
          style="width: 150px"
          @change="onFilterChange"
        >
          <el-option label="全部类型" value="" />
          <el-option
            v-for="(label, key) in typeMap"
            :key="key"
            :label="label"
            :value="key"
          />
        </el-select>
        <el-select
          v-model="filterRetention"
          placeholder="全部层级"
          size="small"
          style="width: 130px"
          @change="onFilterChange"
        >
          <el-option label="全部层级" :value="0" />
          <el-option v-for="level in 5" :key="level" :label="`L${level}`" :value="level" />
        </el-select>
        <el-select
          v-model="filterDecay"
          placeholder="全部状态"
          size="small"
          style="width: 130px"
          @change="onFilterChange"
        >
          <el-option label="全部状态" value="" />
          <el-option label="活跃" value="active" />
          <el-option label="正在淡化" value="fading" />
          <el-option label="已归档" value="archived" />
        </el-select>
        <el-tag size="small" type="info" effect="plain"
          >共 {{ total }} 条</el-tag
        >
      </div>
    </div>

    <div v-if="loading" class="loading">加载中...</div>

    <div v-else class="timeline">
      <div
        v-for="m in memories"
        :key="m.id"
        class="timeline-item"
        @click="showDetail(m)"
      >
        <div
          class="timeline-marker"
          :style="{ background: sentimentColor(m.sentimentScore) }"
        ></div>
        <div class="timeline-content">
          <div class="item-header">
            <span class="scene-emoji">{{ sceneEmoji(m.sceneType) }}</span>
            <span class="scene-type">{{ sceneLabel(m.sceneType) }}</span>
            <el-tag size="small" effect="plain" :type="retentionTagType(m.retentionLevel)">
              L{{ normalizeRetention(m.retentionLevel) }} · {{ strengthPercent(m.memoryStrength) }}%
            </el-tag>
            <el-tag v-if="m.decayState === 'archived'" size="small" type="info">已归档</el-tag>
            <el-tag v-else-if="m.decayState === 'fading'" size="small" type="warning">淡化中</el-tag>
            <span
              class="sentiment-badge"
              :style="{ background: sentimentColor(m.sentimentScore) }"
            >
              {{ m.sentimentScore > 0 ? "+" : "" }}{{ m.sentimentScore }}
            </span>
          </div>
          <div class="item-title">{{ m.title }}</div>
          <div class="item-content">{{ m.content }}</div>
          <div class="item-sequence" v-if="m.messageIdStart || m.messageIdEnd">
            <el-icon size="12"><ChatLineSquare /></el-icon>
            <span class="seq-label">消息范围</span>
            <span class="seq-value"
              >{{ shortId(m.messageIdStart) }} ~
              {{ shortId(m.messageIdEnd) }}</span
            >
          </div>
          <div class="item-footer">
            <el-tag v-if="m.triggerKeywords" size="small" type="info">{{
              m.triggerKeywords
            }}</el-tag>
            <span class="conv-source" v-if="m.sourceConvId">
              <el-icon size="12"><Message /></el-icon>
              {{ shortId(m.sourceConvId) }}
            </span>
            <span class="time">{{ m.createdAt }}</span>
            <el-dropdown trigger="click" @command="(level) => handleRetention(m, Number(level))">
              <el-button size="small" text @click.stop>调整层级</el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item v-for="level in 5" :key="level" :command="level">L{{ level }}</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
            <el-button v-if="m.decayState === 'archived'" size="small" text type="success" @click.stop="handleRestore(m.id)">恢复</el-button>
            <el-button
              size="small"
              text
              type="danger"
              @click.stop="handleDelete(m.id)"
              >删除</el-button
            >
          </div>
          <div class="sentiment-bar-track">
            <div
              class="sentiment-bar-fill"
              :style="{
                width: sentimentIntensity(m.sentimentScore).percent + '%',
                background: sentimentColor(m.sentimentScore),
              }"
            ></div>
          </div>
        </div>
      </div>

      <div v-if="memories.length === 0" class="empty">暂无情景记忆</div>
    </div>

    <el-dialog
      v-model="drawerVisible"
      title="情景详情"
      width="560px"
      align-center
      @close="detailMemory = null"
      destroy-on-close
    >
      <template v-if="detailMemory">
        <div class="detail-section">
          <h3 class="detail-title">
            {{ sceneEmoji(detailMemory.sceneType) }} {{ detailMemory.title }}
          </h3>
          <p class="detail-content">{{ detailMemory.content }}</p>
          <div class="detail-meta">
            <el-tag size="small">{{
              sceneLabel(detailMemory.sceneType)
            }}</el-tag>
            <el-tag
              size="small"
              :color="sentimentColor(detailMemory.sentimentScore)"
              effect="dark"
            >
              情感 {{ detailMemory.sentimentScore > 0 ? "+" : ""
              }}{{ detailMemory.sentimentScore }} ({{
                sentimentIntensity(detailMemory.sentimentScore).label
              }})
            </el-tag>
            <el-tag
              v-if="detailMemory.triggerKeywords"
              size="small"
              type="info"
              >{{ detailMemory.triggerKeywords }}</el-tag
            >
            <el-tag size="small" :type="retentionTagType(detailMemory.retentionLevel)">
              L{{ normalizeRetention(detailMemory.retentionLevel) }} · {{ strengthPercent(detailMemory.memoryStrength) }}%
            </el-tag>
            <el-tag v-if="detailMemory.decayState === 'archived'" size="small" type="info">已归档</el-tag>
            <el-tag v-else-if="detailMemory.decayState === 'fading'" size="small" type="warning">正在淡化</el-tag>
          </div>
          <div class="detail-row">
            <span class="detail-row-label">记忆保持</span>
            <el-select
              :model-value="normalizeRetention(detailMemory.retentionLevel)"
              size="small"
              style="width: 150px"
              @change="(level) => handleRetention(detailMemory, Number(level))"
            >
              <el-option v-for="level in 5" :key="level" :label="`L${level}`" :value="level" />
            </el-select>
            <span class="detail-row-value">强化 {{ detailMemory.reinforceCount || 0 }} 次</span>
            <el-button v-if="detailMemory.decayState === 'archived'" size="small" type="success" plain @click="handleRestore(detailMemory.id)">恢复归档</el-button>
          </div>
          <div
            v-if="detailMemory.messageIdStart || detailMemory.messageIdEnd"
            class="detail-row"
          >
            <span class="detail-row-label">消息序列</span>
            <span class="detail-row-value"
              >{{ shortId(detailMemory.messageIdStart) }} ~
              {{ shortId(detailMemory.messageIdEnd) }}</span
            >
          </div>
          <div v-if="detailMemory.sourceConvId" class="detail-row">
            <span class="detail-row-label">来源会话</span>
            <span class="detail-row-value">{{
              detailMemory.sourceConvId
            }}</span>
          </div>
          <div class="detail-sentiment-bar">
            <span class="sentiment-label">情感强度</span>
            <div class="sentiment-bar-track detail-sentiment-track">
              <div
                class="sentiment-bar-fill"
                :style="{
                  width:
                    sentimentIntensity(detailMemory.sentimentScore).percent +
                    '%',
                  background: sentimentColor(detailMemory.sentimentScore),
                }"
              ></div>
            </div>
            <span class="sentiment-value"
              >{{ detailMemory.sentimentScore > 0 ? "+" : ""
              }}{{ detailMemory.sentimentScore }}</span
            >
          </div>
        </div>
        <div v-if="detailMessages.length > 0" class="context-bubbles">
          <h4>对话上下文 ({{ detailMessages.length }} 条)</h4>
          <div
            v-for="msg in detailMessages"
            :key="msg.id"
            class="context-bubble"
            :class="'role-' + msg.role"
          >
            <span class="bubble-role">{{
              msg.role === "user" ? "用户" : "AI"
            }}</span>
            <span v-if="msg.sequence" class="bubble-seq"
              >#{{ msg.sequence }}</span
            >
            <span class="bubble-text">{{ msg.content }}</span>
          </div>
        </div>
      </template>
      <template #footer>
        <el-button type="primary" @click="drawerVisible = false"
          >关闭</el-button
        >
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { ElMessageBox } from "element-plus";
import { ChatLineSquare, Message } from "@element-plus/icons-vue";
import { useEpisodic, type EpisodicMemory } from "@/composables/useEpisodic";

const {
  memories,
  loading,
  total,
  fetchMemories,
  deleteMemory,
  updateRetention,
  restoreMemory,
  getDetail,
  sceneLabel,
  sceneEmoji,
  sentimentColor,
  sentimentIntensity,
} = useEpisodic();

const typeMap: Record<string, string> = {
  insight: "💡 感悟",
  joke: "😂 笑话",
  milestone: "🏆 里程碑",
  emotional_peak: "💗 情感峰值",
  confession: "🗣️ 坦白",
};

const filterType = ref("");
const filterRetention = ref(0);
const filterDecay = ref("");
const drawerVisible = ref(false);
const detailMemory = ref<EpisodicMemory | null>(null);
const detailMessages = ref<any[]>([]);

onMounted(() => {
  fetchMemories();
});

function onFilterChange() {
  fetchMemories({
    sceneType: filterType.value || undefined,
    retentionLevel: filterRetention.value || undefined,
    decayState: filterDecay.value || undefined,
  });
}

function normalizeRetention(level: number | undefined): number {
  const value = Number(level || 4);
  return Math.min(5, Math.max(1, Math.round(Number.isFinite(value) ? value : 4)));
}

function strengthPercent(strength: number | undefined): number {
  const value = Number(strength ?? 0.5);
  return Math.round(Math.min(1, Math.max(0, Number.isFinite(value) ? value : 0.5)) * 100);
}

function retentionTagType(level: number | undefined) {
  const value = normalizeRetention(level);
  return value <= 2 ? "success" : value === 3 ? "primary" : value === 4 ? "warning" : "info";
}

async function handleRetention(memory: EpisodicMemory, level: number) {
  try {
    await updateRetention(memory.id, level);
    if (detailMemory.value?.id === memory.id) {
      detailMemory.value = { ...detailMemory.value, retentionLevel: level, decayState: "active" };
    }
    await onFilterChangeAsync();
  } catch (error: any) {
    console.error("更新情景记忆层级失败", error);
  }
}

async function handleRestore(id: string) {
  try {
    await restoreMemory(id);
    if (detailMemory.value?.id === id) {
      detailMemory.value = { ...detailMemory.value, decayState: "active" };
    }
    await onFilterChangeAsync();
  } catch (error: any) {
    console.error("恢复情景记忆失败", error);
  }
}

async function onFilterChangeAsync() {
  await fetchMemories({
    sceneType: filterType.value || undefined,
    retentionLevel: filterRetention.value || undefined,
    decayState: filterDecay.value || undefined,
  });
}

function shortId(id: string): string {
  if (!id || id.length < 10) return id || "—";
  return id.slice(0, 8) + "…";
}

async function showDetail(m: EpisodicMemory) {
  detailMemory.value = m;
  detailMessages.value = [];
  drawerVisible.value = true;
  try {
    const data = await getDetail(m.id);
    detailMessages.value = data.messages || [];
  } catch {
    detailMessages.value = [];
  }
}

async function handleDelete(id: string) {
  try {
    await ElMessageBox.confirm("确定删除这条情景记忆？", "删除确认", {
      confirmButtonText: "确定",
      cancelButtonText: "取消",
      type: "warning",
    });
    await deleteMemory(id);
    await onFilterChangeAsync();
  } catch {}
}
</script>

<style scoped>
.episodic-page {
  max-width: 100%;
}
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}
.page-header h2 {
  margin: 0;
  font-size: 24px;
}
.header-controls {
  display: flex;
  align-items: center;
  gap: 10px;
}

.timeline {
  position: relative;
  padding-left: 24px;
}
.timeline::before {
  content: "";
  position: absolute;
  left: 8px;
  top: 0;
  bottom: 0;
  width: 2px;
  background: var(--ac-color-border);
}
.timeline-item {
  position: relative;
  margin-bottom: 20px;
  cursor: pointer;
  display: flex;
  gap: 16px;
}
.timeline-marker {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  border: 2px solid var(--ac-color-text);
  box-shadow: 0 0 0 2px var(--ac-color-border);
  flex-shrink: 0;
  margin-top: 4px;
}
.timeline-content {
  background: var(--ac-color-bg-secondary);
  border-radius: 10px;
  padding: 14px;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.06);
  flex: 1;
}
.item-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}
.scene-emoji {
  font-size: 16px;
}
.scene-type {
  font-size: 12px;
  color: var(--ac-color-text-primary);
}
.sentiment-badge {
  font-size: 11px;
  color: var(--tp-text-on-status);
  padding: 1px 6px;
  border-radius: 8px;
}
.item-title {
  font-size: 16px;
  font-weight: 600;
  margin-bottom: 4px;
  color: var(--ac-color-text-primary);
}
.item-content {
  font-size: 14px;
  color: var(--ac-color-text-secondary);
  margin-bottom: 6px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.item-sequence {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: var(--ac-color-text-secondary);
  margin-bottom: 4px;
}
.seq-label {
  opacity: 0.7;
}
.seq-value {
  font-family: monospace;
  font-size: 11px;
}
.item-footer {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 12px;
  color: var(--ac-color-text-primary);
}
.conv-source {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  font-size: 11px;
  color: var(--ac-color-text-secondary);
  font-family: monospace;
}
.time {
  margin-left: auto;
}

.sentiment-bar-track {
  height: 3px;
  background: var(--ac-color-border-light);
  border-radius: 2px;
  margin-top: 8px;
  overflow: hidden;
}
.sentiment-bar-fill {
  height: 100%;
  border-radius: 2px;
  transition: width 0.3s ease;
}

.loading,
.empty {
  text-align: center;
  padding: 48px;
  color: var(--ac-color-text-muted);
}

.detail-section {
  padding: 0 4px;
}
.detail-title {
  font-size: 18px;
  margin: 0 0 12px 0;
}
.detail-content {
  font-size: 14px;
  line-height: 1.7;
  color: var(--ac-color-text-secondary);
  margin-bottom: 16px;
  white-space: pre-wrap;
}
.detail-meta {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}
.detail-row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  margin-bottom: 8px;
}
.detail-row-label {
  font-weight: 500;
  color: var(--ac-color-text-primary);
  min-width: 70px;
}
.detail-row-value {
  font-family: monospace;
  font-size: 12px;
  color: var(--ac-color-text-secondary);
  word-break: break-all;
}
.detail-sentiment-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 6px;
  margin-bottom: 12px;
}
.sentiment-label {
  font-size: 12px;
  color: var(--ac-color-text-secondary);
  min-width: 60px;
}
.detail-sentiment-track {
  flex: 1;
  margin-top: 0;
  height: 5px;
}
.sentiment-value {
  font-size: 12px;
  font-weight: 600;
  min-width: 36px;
  text-align: right;
}

.context-bubbles {
  margin-top: 20px;
  border-top: 1px solid var(--ac-color-border);
  padding-top: 16px;
}
.context-bubbles h4 {
  font-size: 14px;
  margin: 0 0 12px 0;
  font-weight: 600;
}
.context-bubble {
  padding: 8px 12px;
  border-radius: 8px;
  margin-bottom: 8px;
  font-size: 13px;
  display: flex;
  align-items: flex-start;
  gap: 8px;
}
.context-bubble.role-user {
  background: var(--ac-color-bg-secondary);
}
.context-bubble.role-assistant {
  background: var(--ac-color-primary-bg);
}
.bubble-role {
  font-weight: 600;
  font-size: 12px;
  white-space: nowrap;
  min-width: 36px;
}
.bubble-seq {
  font-family: monospace;
  font-size: 11px;
  color: var(--ac-color-text-secondary);
  white-space: nowrap;
}
.bubble-text {
  color: var(--ac-color-text-primary);
}
</style>
