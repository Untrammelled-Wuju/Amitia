<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="psyche-detail">
    <div v-if="psycheLoading" class="psyche-placeholder">
      <el-icon class="is-loading" size="24"><Loading /></el-icon>
      <span>加载心理状态...</span>
    </div>

    <div v-else-if="psycheError" class="psyche-placeholder">
      <el-icon :size="24" color="var(--el-color-danger)"
        ><WarningFilled
      /></el-icon>
      <span>{{ psycheError }}</span>
    </div>

    <template v-else>
      <div class="psyche-top-row">
        <div class="affect-badge" :class="affectClass">
          {{ psycheState.affectLabel || "未初始化" }}
        </div>
        <span class="collect-time"
          >采样时间: {{ formatTime(psycheState.collectedAt) }}</span
        >
      </div>

      <div class="psyche-grid">
        <div class="psyche-panel">
          <div class="panel-header">
            <span class="panel-icon">😊</span>
            <span>情绪状态</span>
          </div>
          <div class="emotion-bars">
            <div class="ebar-row">
              <span class="ebar-label positive-label">积极</span>
              <div class="ebar-track">
                <div
                  class="ebar-fill positive-fill"
                  :style="{ width: pct(psycheState.emotion.positive) }"
                ></div>
              </div>
              <span class="ebar-val">{{
                pct(psycheState.emotion.positive)
              }}</span>
            </div>
            <div class="ebar-row">
              <span class="ebar-label negative-label">消极</span>
              <div class="ebar-track">
                <div
                  class="ebar-fill negative-fill"
                  :style="{ width: pct(psycheState.emotion.negative) }"
                ></div>
              </div>
              <span class="ebar-val">{{
                pct(psycheState.emotion.negative)
              }}</span>
            </div>
            <div class="ebar-row">
              <span class="ebar-label">唤醒度</span>
              <div class="ebar-track">
                <div
                  class="ebar-fill arousal-fill"
                  :style="{ width: pct(psycheState.emotion.arousal) }"
                ></div>
              </div>
              <span class="ebar-val">{{
                pct(psycheState.emotion.arousal)
              }}</span>
            </div>
            <div class="ebar-row">
              <span class="ebar-label">支配度</span>
              <div class="ebar-track">
                <div
                  class="ebar-fill dominance-fill"
                  :style="{ width: pct(psycheState.emotion.dominance) }"
                ></div>
              </div>
              <span class="ebar-val">{{
                pct(psycheState.emotion.dominance)
              }}</span>
            </div>
          </div>
          <div class="panel-desc">{{ emotionDesc }}</div>
        </div>

        <div class="psyche-panel">
          <div class="panel-header">
            <span class="panel-icon">🌊</span>
            <span>心境</span>
          </div>
          <div class="mood-bars">
            <div class="ebar-row">
              <span class="ebar-label">效价</span>
              <div class="ebar-track">
                <div
                  class="ebar-fill valence-fill"
                  :style="{ width: pct(psycheState.mood.valence) }"
                ></div>
              </div>
              <span class="ebar-val">{{ pct(psycheState.mood.valence) }}</span>
            </div>
            <div class="ebar-row">
              <span class="ebar-label">张力</span>
              <div class="ebar-track">
                <div
                  class="ebar-fill tension-fill"
                  :style="{ width: pct(psycheState.mood.tension) }"
                ></div>
              </div>
              <span class="ebar-val">{{ pct(psycheState.mood.tension) }}</span>
            </div>
          </div>
          <div class="panel-desc">{{ moodDesc }}</div>
        </div>

        <div class="psyche-panel">
          <div class="panel-header">
            <span class="panel-icon">⚡</span>
            <span>压力 & 精力</span>
          </div>
          <div class="stress-energy">
            <div class="se-item">
              <div class="se-header">
                <span class="se-label">压力</span>
                <span class="se-val" :class="stressLevel">{{
                  pct(psycheState.stress)
                }}</span>
              </div>
              <div class="se-ring-track">
                <div
                  class="se-ring-fill"
                  :class="stressLevel"
                  :style="{ width: pct(psycheState.stress) }"
                ></div>
              </div>
            </div>
            <div class="se-item">
              <div class="se-header">
                <span class="se-label">精力</span>
                <span class="se-val" :class="energyLevel">{{
                  pct(psycheState.energy)
                }}</span>
              </div>
              <div class="se-ring-track">
                <div
                  class="se-ring-fill energy-fill"
                  :class="energyLevel"
                  :style="{ width: pct(psycheState.energy) }"
                ></div>
              </div>
            </div>
          </div>
          <div class="panel-desc">{{ stressEnergyDesc }}</div>
        </div>
      </div>

      <div class="psyche-panel needs-panel" v-if="needsEntries.length">
        <div class="panel-header">
          <span class="panel-icon">🎯</span>
          <span>心理需求</span>
        </div>
        <div class="needs-grid">
          <div class="need-item" v-for="[k, v] in needsEntries" :key="k">
            <div class="need-header">
              <span class="need-label">{{ formatNeedLabel(k) }}</span>
              <span class="need-val">{{ (v * 10).toFixed(1) }}</span>
            </div>
            <div class="need-bar-track">
              <div
                class="need-bar-fill"
                :style="{
                  width: Math.min(v * 100, 100).toFixed(0) + '%',
                  backgroundColor: needColor(v),
                }"
              ></div>
            </div>
          </div>
        </div>
        <div class="panel-desc">{{ needsDesc }}</div>
      </div>

      <div class="psyche-panel rel-panel">
        <div class="panel-header">
          <span class="panel-icon">🤝</span>
          <span>关系状态</span>
        </div>
        <div class="rel-grid">
          <div class="rel-item" v-for="rel in relEntries" :key="rel.key">
            <div class="rel-header">
              <span class="rel-label">{{ rel.label }}</span>
              <span class="rel-val">{{ pct(rel.value) }}</span>
            </div>
            <div class="rel-bar-track">
              <div
                class="rel-bar-fill"
                :style="{
                  width: pct(rel.value),
                  backgroundColor: relColor(rel.value),
                }"
              ></div>
            </div>
          </div>
        </div>
        <div class="panel-desc">{{ relationDesc }}</div>
      </div>

      <div
        class="psyche-panel beliefs-panel"
        v-if="psycheState.beliefs?.length"
      >
        <div class="panel-header">
          <span class="panel-icon">🧠</span>
          <span>活跃信念 ({{ psycheState.beliefs.length }})</span>
        </div>
        <div class="beliefs-list">
          <div
            class="belief-card"
            v-for="b in psycheState.beliefs"
            :key="b.key"
          >
            <div class="belief-top">
              <span class="belief-key">{{ b.key }}</span>
              <el-tag
                :type="b.conflicted ? 'danger' : 'success'"
                size="small"
                effect="plain"
              >
                {{ b.conflicted ? "冲突" : "一致" }}
              </el-tag>
            </div>
            <div class="belief-value">{{ b.value }}</div>
            <div class="belief-conf-bar">
              <div class="bcb-track">
                <div
                  class="bcb-fill"
                  :class="{
                    'bcb-low': b.confidence < 0.4,
                    'bcb-mid': b.confidence >= 0.4 && b.confidence < 0.7,
                    'bcb-high': b.confidence >= 0.7,
                  }"
                  :style="{ width: pct(b.confidence) }"
                ></div>
              </div>
              <span class="bcb-val"
                >{{ (b.confidence * 100).toFixed(0) }}%</span
              >
            </div>
          </div>
        </div>
        <div class="panel-desc">{{ beliefsDesc }}</div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, inject, watch, onMounted, computed, type Ref } from "vue";
import { Loading, WarningFilled } from "@element-plus/icons-vue";
import { apiClient } from "@/composables/useApi";
import type { PsycheStateSnapshot } from "../../types";

const currentCharacterId = inject<Ref<string | null>>("currentCharacterId");

const psycheLoading = ref(false);
const psycheError = ref("");
const psycheState = ref<PsycheStateSnapshot>(defaultPsycheState());

function defaultPsycheState(): PsycheStateSnapshot {
  return {
    emotion: { positive: 0.5, negative: 0.5, arousal: 0.5, dominance: 0.5 },
    mood: { valence: 0.5, tension: 0.5, pad: "" },
    stress: 0.0,
    energy: 0.5,
    needs: {},
    beliefs: [],
    relationship: { trust: 0.5, familiarity: 0.5, tension: 0.0, security: 0.5 },
    affectLabel: "",
    collectedAt: "",
  };
}

async function loadPsycheState() {
  if (!currentCharacterId?.value) return;
  psycheLoading.value = true;
  psycheError.value = "";
  try {
    const { data } = await apiClient.get("/api/psyche/snapshot", {
      params: { characterId: currentCharacterId.value },
    });
    if ((data as any)?.data) {
      psycheState.value = (data as any).data;
    } else if (data) {
      psycheState.value = data as PsycheStateSnapshot;
    }
  } catch (e: any) {
    psycheError.value = "心理状态不可用: " + (e?.message || "网络错误");
    psycheState.value = defaultPsycheState();
  } finally {
    psycheLoading.value = false;
  }
}

onMounted(() => {
  loadPsycheState();
});

watch(currentCharacterId as Ref<string | null>, () => {
  loadPsycheState();
});

const needsEntries = computed(() => {
  return Object.entries(psycheState.value.needs).sort((a, b) => b[1] - a[1]);
});

const relEntries = computed(() => {
  const r = psycheState.value.relationship;
  return [
    { key: "trust", label: "信任", value: r.trust },
    { key: "familiarity", label: "熟悉", value: r.familiarity },
    { key: "tension", label: "张力", value: r.tension },
    { key: "security", label: "安全感", value: r.security },
  ];
});

const emotionDesc = computed(() => {
  const e = psycheState.value.emotion;
  const p = e.positive;
  const n = e.negative;
  const a = e.arousal;
  const d = e.dominance;
  let desc = "";
  if (p > 0.65 && n < 0.35) desc = "当前情绪偏向积极，整体愉悦感较强。";
  else if (n > 0.65 && p < 0.35)
    desc = "当前情绪偏向消极，整体低落感较为明显。";
  else if (p > 0.5 && n > 0.5) desc = "情绪呈现矛盾状态，积极与消极并存。";
  else desc = "情绪处于中性平缓状态。";
  if (a > 0.65) desc += " 唤醒度较高，情绪反应活跃。";
  else if (a < 0.35) desc += " 唤醒度较低，情绪趋于平缓。";
  if (d > 0.65) desc += " 支配感较强。";
  else if (d < 0.35) desc += " 支配感较弱。";
  return desc;
});

const moodDesc = computed(() => {
  const m = psycheState.value.mood;
  const v = m.valence;
  const t = m.tension;
  let desc = "";
  if (v > 0.65) desc = "心境偏向愉悦舒适，整体感觉良好。";
  else if (v < 0.35) desc = "心境偏向低沉不适，整体感觉欠佳。";
  else desc = "心境处于中性范围，无明显偏好。";
  if (t > 0.65) desc += " 内心张力较高，可能存在焦虑或紧张。";
  else if (t < 0.35) desc += " 内心张力较低，处于放松状态。";
  return desc;
});

const stressEnergyDesc = computed(() => {
  const s = psycheState.value.stress;
  const e = psycheState.value.energy;
  let desc = "";
  if (s > 0.65) desc = "压力水平较高，对外部压力源较为敏感。";
  else if (s < 0.35) desc = "压力水平较低，心态放松。";
  else desc = "压力水平适中，处于可接受范围。";
  if (e > 0.65) desc += " 精力充沛，具备较高的活跃能力。";
  else if (e < 0.35) desc += " 精力不足，可能出现倦怠感。";
  else desc += " 精力处于正常水平。";
  return desc;
});

const needsDesc = computed(() => {
  const entries = needsEntries.value;
  if (!entries.length) return "";
  const high = entries
    .filter(([, v]) => v > 0.65)
    .map(([k]) => formatNeedLabel(k));
  const low = entries
    .filter(([, v]) => v < 0.35)
    .map(([k]) => formatNeedLabel(k));
  let desc = "";
  if (high.length) desc = `当前亟需满足的需求：${high.join("、")}。`;
  if (low.length) desc += ` 已充分满足的需求：${low.join("、")}。`;
  if (!high.length && !low.length)
    desc = "各需求处于均衡状态，无明显缺口或溢出。";
  return desc;
});

const relationDesc = computed(() => {
  const r = psycheState.value.relationship;
  let desc = "";
  if (r.trust > 0.65) desc = "信任度较高，关系基础稳固。";
  else if (r.trust < 0.35) desc = "信任度较低，关系存在信任危机。";
  else desc = "信任度适中，关系正在建设阶段。";
  if (r.tension > 0.65) desc += " 关系张力较高，可能存在冲突或对立。";
  else if (r.tension < 0.35) desc += " 关系和谐，无明显对立。";
  if (r.security > 0.65) desc += " 安全感较足。";
  else if (r.security < 0.35) desc += " 安全感不足。";
  return desc;
});

const beliefsDesc = computed(() => {
  const beliefs = psycheState.value.beliefs;
  if (!beliefs?.length) return "";
  const high = beliefs.filter((b) => b.confidence > 0.65);
  const low = beliefs.filter((b) => b.confidence < 0.35);
  const conflicted = beliefs.filter((b) => b.conflicted);
  let desc = `共${beliefs.length}条活跃信念。`;
  if (high.length) desc += ` 其中${high.length}条确信度较高。`;
  if (low.length) desc += ` ${low.length}条确信度较低。`;
  if (conflicted.length) desc += ` ${conflicted.length}条信念存在冲突。`;
  return desc;
});

const affectClass = computed(() => {
  const l = psycheState.value.affectLabel;
  if (!l) return "affect-default";
  if (l === "积极") return "affect-positive";
  if (l === "消极") return "affect-negative";
  if (l === "紧张") return "affect-tense";
  return "affect-default";
});

const stressLevel = computed(() => {
  const s = psycheState.value.stress;
  if (s >= 0.7) return "level-high";
  if (s >= 0.4) return "level-mid";
  return "level-low";
});

const energyLevel = computed(() => {
  const e = psycheState.value.energy;
  if (e >= 0.7) return "level-high";
  if (e >= 0.4) return "level-mid";
  return "level-low";
});

function pct(v: number): string {
  return (v * 100).toFixed(0) + "%";
}

function formatTime(d: string): string {
  if (!d) return "—";
  try {
    const date = new Date(d);
    return (
      date.toLocaleDateString("zh-CN", { month: "numeric", day: "numeric" }) +
      " " +
      date.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" })
    );
  } catch {
    return d;
  }
}

function formatNeedLabel(k: string): string {
  const map: Record<string, string> = {
    reassurance: "安心",
    connection: "连接",
    autonomy: "自主",
    clarity: "清晰",
    rest: "休息",
    expression: "表达",
    novelty: "新鲜",
  };
  return map[k] || k;
}

function needColor(v: number): string {
  if (v < 0.3) return "var(--ac-color-danger)";
  if (v < 0.6) return "var(--ac-color-warning)";
  return "var(--ac-color-success)";
}

function relColor(v: number): string {
  if (v < 0.3) return "var(--ac-color-danger)";
  if (v < 0.6) return "var(--ac-color-warning)";
  return "var(--ac-color-primary)";
}
</script>

<style scoped>
.psyche-detail {
  padding: 4px 0;
}

.psyche-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 40px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.psyche-top-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.affect-badge {
  display: inline-flex;
  align-items: center;
  padding: 4px 16px;
  border-radius: 20px;
  font-size: 14px;
  font-weight: 600;
  letter-spacing: 1px;
}

.affect-default {
  background: var(--el-fill-color);
  color: var(--el-text-color-regular);
}
.affect-positive {
  background: var(--ac-color-success-bg);
  color: var(--ac-color-success);
}
.affect-negative {
  background: var(--ac-color-danger-bg);
  color: var(--ac-color-danger);
}
.affect-tense {
  background: var(--ac-color-warning-bg);
  color: var(--ac-color-warning);
}

.collect-time {
  font-size: 11px;
  color: var(--el-text-color-placeholder);
}

.psyche-grid {
  display: grid;
  grid-template-columns: 1.2fr 0.8fr 1fr;
  gap: 14px;
  margin-bottom: 14px;
}

@media (max-width: 860px) {
  .psyche-grid {
    grid-template-columns: 1fr 1fr;
  }
}

@media (max-width: 560px) {
  .psyche-grid {
    grid-template-columns: 1fr;
  }
}

.psyche-panel {
  background: var(--el-fill-color-lighter);
  border-radius: 8px;
  padding: 14px;
}

.panel-header {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  margin-bottom: 12px;
}

.panel-icon {
  font-size: 16px;
}

.ebar-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
}

.ebar-row:last-child {
  margin-bottom: 0;
}

.ebar-label {
  width: 42px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  flex-shrink: 0;
}

.ebar-track {
  flex: 1;
  height: 10px;
  border-radius: 5px;
  background: var(--el-bg-color);
  overflow: hidden;
}

.ebar-fill {
  height: 100%;
  border-radius: 5px;
  transition: width 0.6s ease;
}

.positive-fill {
  background: linear-gradient(
    90deg,
    var(--ac-color-success-bg),
    var(--ac-color-success)
  );
}
.negative-fill {
  background: linear-gradient(
    90deg,
    var(--ac-color-danger-bg),
    var(--ac-color-danger)
  );
}
.arousal-fill {
  background: linear-gradient(
    90deg,
    var(--ac-color-warning-bg),
    var(--ac-color-warning)
  );
}
.dominance-fill {
  background: linear-gradient(
    90deg,
    var(--ac-color-primary-bg),
    var(--ac-color-primary)
  );
}
.valence-fill {
  background: linear-gradient(
    90deg,
    var(--ac-color-primary-bg),
    var(--ac-color-primary)
  );
}
.tension-fill {
  background: linear-gradient(
    90deg,
    var(--ac-color-danger-bg),
    var(--ac-color-danger)
  );
}

.ebar-val {
  width: 36px;
  font-size: 11px;
  color: var(--el-text-color-secondary);
  text-align: right;
  flex-shrink: 0;
}

.stress-energy {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.se-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.se-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.se-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.se-val {
  font-size: 13px;
  font-weight: 600;
}

.level-low {
  color: var(--ac-color-success);
}
.level-mid {
  color: var(--ac-color-warning);
}
.level-high {
  color: var(--ac-color-danger);
}

.se-ring-track {
  height: 8px;
  border-radius: 4px;
  background: var(--el-bg-color);
  overflow: hidden;
}

.se-ring-fill {
  height: 100%;
  border-radius: 4px;
  transition: width 0.6s ease;
}

.se-ring-fill.level-low {
  background: var(--ac-color-success);
}
.se-ring-fill.level-mid {
  background: var(--ac-color-warning);
}
.se-ring-fill.level-high {
  background: var(--ac-color-danger);
}

.se-ring-fill.energy-fill.level-low {
  background: var(--ac-color-danger);
}
.se-ring-fill.energy-fill.level-mid {
  background: var(--ac-color-warning);
}
.se-ring-fill.energy-fill.level-high {
  background: var(--ac-color-success);
}

.needs-panel,
.rel-panel,
.beliefs-panel {
  margin-bottom: 14px;
}

.needs-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px 24px;
}

@media (max-width: 560px) {
  .needs-grid {
    grid-template-columns: 1fr;
  }
}

.need-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.need-header,
.rel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.need-label,
.rel-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.need-val,
.rel-val {
  font-size: 12px;
  font-weight: 500;
  color: var(--el-text-color-regular);
}

.need-bar-track,
.rel-bar-track {
  height: 6px;
  border-radius: 3px;
  background: var(--el-bg-color);
  overflow: hidden;
}

.need-bar-fill {
  height: 100%;
  border-radius: 3px;
  transition: width 0.5s ease;
}

.rel-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px 24px;
}

@media (max-width: 560px) {
  .rel-grid {
    grid-template-columns: 1fr;
  }
}

.rel-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.rel-bar-fill {
  height: 100%;
  border-radius: 3px;
  transition: width 0.5s ease;
}

.beliefs-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.belief-card {
  background: var(--el-bg-color);
  border-radius: 6px;
  padding: 10px 12px;
}

.panel-desc {
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--el-border-color-lighter);
  font-size: 12px;
  line-height: 1.5;
  color: var(--el-text-color-secondary);
}

.belief-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 4px;
}

.belief-key {
  font-size: 13px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.belief-value {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-bottom: 8px;
}

.belief-conf-bar {
  display: flex;
  align-items: center;
  gap: 8px;
}

.bcb-track {
  flex: 1;
  height: 5px;
  border-radius: 3px;
  background: var(--el-fill-color);
  overflow: hidden;
}

.bcb-fill {
  height: 100%;
  border-radius: 3px;
  transition: width 0.5s ease;
}

.bcb-low {
  background: var(--ac-color-danger);
}
.bcb-mid {
  background: var(--ac-color-warning);
}
.bcb-high {
  background: var(--ac-color-success);
}

.bcb-val {
  font-size: 11px;
  color: var(--el-text-color-secondary);
  width: 34px;
  text-align: right;
}
</style>
