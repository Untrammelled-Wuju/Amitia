<template>
  <div class="decision-viz-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">BDI 决策可视化</h2>
        <p class="page-subtitle">
          查看 BehaviorPlan 意图策略与 ExpressionPlan 表达参数
        </p>
      </div>
      <div class="page-actions">
        <el-tag
          v-if="hasPlan"
          :type="snapshot.meta.degraded ? 'warning' : 'success'"
          effect="plain"
        >
          {{ snapshot.meta.degraded ? "降级中" : "运行正常" }}
        </el-tag>
        <el-button :loading="loading" type="primary" @click="load"
          >刷新</el-button
        >
      </div>
    </div>
    <el-alert
      v-if="loadError"
      type="warning"
      :closable="false"
      show-icon
      :title="loadError"
      class="top-alert"
    />
    <template v-if="hasPlan && snapshot.behaviorPlan">
      <el-card shadow="hover" class="panel-card">
        <template #header>
          <span>BehaviorPlan — 行为计划</span>
          <el-tag size="small" style="margin-left: 10px"
            >stateVersion: {{ snapshot.behaviorPlan.stateVersion }}</el-tag
          >
        </template>
        <div class="plan-grid">
          <div class="plan-field">
            <label>意图 (Intention)</label>
            <span>{{ snapshot.behaviorPlan.intention }}</span>
          </div>
          <div class="plan-field">
            <label>策略 (Strategy)</label>
            <span>{{ snapshot.behaviorPlan.strategy }}</span>
          </div>
          <div class="plan-field">
            <label>胜出候选</label>
            <el-tag type="success" size="small">{{
              snapshot.behaviorPlan.winnerCandidate
            }}</el-tag>
          </div>
          <div class="plan-field">
            <label>被拒候选</label>
            <span>{{
              snapshot.behaviorPlan.rejectedCandidates?.join(", ") || "无"
            }}</span>
          </div>
        </div>
        <el-divider />
        <div class="section-title">内容约束</div>
        <div class="constraint-grid">
          <div class="constraint-col">
            <h4>必须包含</h4>
            <ul>
              <li v-for="item in snapshot.behaviorPlan.mustInclude" :key="item">
                {{ item }}
              </li>
              <li
                v-if="!snapshot.behaviorPlan.mustInclude?.length"
                class="empty"
              >
                无
              </li>
            </ul>
          </div>
          <div class="constraint-col">
            <h4>可以包含</h4>
            <ul>
              <li v-for="item in snapshot.behaviorPlan.mayInclude" :key="item">
                {{ item }}
              </li>
              <li
                v-if="!snapshot.behaviorPlan.mayInclude?.length"
                class="empty"
              >
                无
              </li>
            </ul>
          </div>
          <div class="constraint-col">
            <h4>必须避免</h4>
            <ul>
              <li v-for="item in snapshot.behaviorPlan.mustAvoid" :key="item">
                {{ item }}
              </li>
              <li v-if="!snapshot.behaviorPlan.mustAvoid?.length" class="empty">
                无
              </li>
            </ul>
          </div>
        </div>
        <el-divider />
        <div class="policy-row">
          <div class="policy-item">
            <label>提问策略</label
            ><el-tag size="small">{{
              snapshot.behaviorPlan.questionPolicy
            }}</el-tag>
          </div>
          <div class="policy-item">
            <label>建议策略</label
            ><el-tag size="small">{{
              snapshot.behaviorPlan.advicePolicy
            }}</el-tag>
          </div>
          <div class="policy-item">
            <label>投递策略</label
            ><el-tag size="small">{{
              snapshot.behaviorPlan.deliveryPolicy
            }}</el-tag>
          </div>
        </div>
      </el-card>
      <el-card v-if="snapshot.expressionPlan" shadow="hover" class="panel-card">
        <template #header>ExpressionPlan — 表达计划</template>
        <div class="plan-grid">
          <div class="plan-field">
            <label>句子数</label
            ><span>{{ snapshot.expressionPlan.sentenceCount }}</span>
          </div>
          <div class="plan-field">
            <label>最大长度</label
            ><span>{{ snapshot.expressionPlan.maxLength }}</span>
          </div>
          <div class="plan-field">
            <label>直接度</label
            ><el-progress
              :percentage="snapshot.expressionPlan.directness * 100"
              :stroke-width="14"
              :text-inside="true"
              :status="
                snapshot.expressionPlan.directness > 0.7
                  ? 'success'
                  : snapshot.expressionPlan.directness > 0.4
                    ? 'warning'
                    : 'exception'
              "
            />
          </div>
          <div class="plan-field">
            <label>温暖度</label
            ><el-progress
              :percentage="snapshot.expressionPlan.warmth * 100"
              :stroke-width="14"
              :text-inside="true"
              :status="
                snapshot.expressionPlan.warmth > 0.6 ? 'success' : 'warning'
              "
            />
          </div>
          <div class="plan-field">
            <label>情绪展示</label
            ><el-tag
              size="small"
              :type="emotionTagType(snapshot.expressionPlan.emotionDisplay)"
              >{{ snapshot.expressionPlan.emotionDisplay }}</el-tag
            >
          </div>
          <div class="plan-field">
            <label>使用提问</label
            ><el-switch
              :model-value="snapshot.expressionPlan.useQuestion"
              disabled
            />
          </div>
        </div>
        <el-divider />
        <div class="section-title">避免话题</div>
        <div class="tag-list">
          <el-tag
            v-for="topic in snapshot.expressionPlan.avoidTopics"
            :key="topic"
            size="small"
            type="danger"
            effect="plain"
            >{{ topic }}</el-tag
          >
          <span
            v-if="!snapshot.expressionPlan.avoidTopics?.length"
            class="empty-hint"
            >无</span
          >
        </div>
        <el-divider v-if="snapshot.expressionPlan.voiceParams" />
        <div v-if="snapshot.expressionPlan.voiceParams" class="section-title">
          语音参数
        </div>
        <code v-if="snapshot.expressionPlan.voiceParams" class="voice-params">{{
          snapshot.expressionPlan.voiceParams
        }}</code>
      </el-card>
    </template>
    <el-empty
      v-else-if="!loading && !loadError"
      description="暂无决策数据，请稍后刷新"
    />
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted } from "vue";
import { useDecisionViz } from "./useDecisionViz";
const { loading, loadError, snapshot, load } = useDecisionViz();
const hasPlan = computed(
  () => !!snapshot.behaviorPlan || !!snapshot.expressionPlan,
);
function emotionTagType(emotion: string) {
  if (["positive", "joy", "love", "excited", "grateful"].includes(emotion))
    return "success";
  if (["negative", "sad", "angry", "anxious", "fear"].includes(emotion))
    return "danger";
  if (["neutral", "calm", "thoughtful"].includes(emotion)) return "info";
  return "warning";
}
onMounted(load);
</script>
<style scoped>
.decision-viz-page {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  flex-wrap: wrap;
}
.page-title {
  margin: 0;
  font-size: 20px;
  line-height: 28px;
  color: var(--ac-color-text);
}
.page-subtitle {
  margin: 6px 0 0;
  color: var(--ac-color-text-secondary);
  font-size: 13px;
  line-height: 20px;
}
.page-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}
.top-alert {
  margin-bottom: 4px;
}
.panel-card :deep(.el-card__header) {
  font-weight: 600;
  display: flex;
  align-items: center;
}
.plan-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}
.plan-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.plan-field label {
  font-size: 12px;
  color: var(--ac-color-text-muted);
  line-height: 18px;
}
.plan-field span:not(.el-tag) {
  font-size: 14px;
  color: var(--ac-color-text);
  line-height: 22px;
}
.section-title {
  font-size: 14px;
  font-weight: 500;
  margin-bottom: 10px;
  color: var(--ac-color-text);
}
.constraint-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}
.constraint-col h4 {
  margin: 0 0 8px;
  font-size: 13px;
  font-weight: 500;
  color: var(--ac-color-text);
}
.constraint-col ul {
  margin: 0;
  padding-left: 18px;
  list-style: disc;
}
.constraint-col li {
  font-size: 13px;
  line-height: 22px;
  color: var(--ac-color-text);
}
.constraint-col li.empty {
  list-style: none;
  color: var(--ac-color-text-muted);
  font-style: italic;
  padding-left: 0;
}
.policy-row {
  display: flex;
  gap: 24px;
  flex-wrap: wrap;
}
.policy-item {
  display: flex;
  align-items: center;
  gap: 8px;
}
.policy-item label {
  font-size: 13px;
  color: var(--ac-color-text-muted);
}
.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.empty-hint {
  color: var(--ac-color-text-muted);
  font-size: 13px;
  font-style: italic;
}
.voice-params {
  display: block;
  padding: 10px 14px;
  background: var(--ac-color-bg-secondary);
  border-radius: 6px;
  font-size: 13px;
  line-height: 20px;
  white-space: pre-wrap;
  word-break: break-all;
}
@media (max-width: 900px) {
  .constraint-grid {
    grid-template-columns: minmax(0, 1fr);
  }
  .plan-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
@media (max-width: 720px) {
  .decision-viz-page {
    padding: 16px;
  }
  .page-actions {
    width: 100%;
    justify-content: space-between;
  }
}
</style>
