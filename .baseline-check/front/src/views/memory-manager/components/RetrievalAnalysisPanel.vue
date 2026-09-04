<!-- SPDX-FileCopyrightText: 2026 Peng Xu -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<template>
  <div class="analysis-panel">
    <h3 class="ap-title">检索质量分析</h3>

    <div class="ap-stats-row">
      <el-card shadow="hover" class="ap-stat-card">
        <div class="ap-stat-num">{{ retrievalStats.totalCount }}</div>
        <div class="ap-stat-label">总检索次数</div>
      </el-card>
      <el-card shadow="hover" class="ap-stat-card">
        <div class="ap-stat-num" v-if="retrievalLogs.length > 0">
          {{
            (
              (retrievalLogs.length / (retrievalStats.totalCount || 1)) *
              100
            ).toFixed(1)
          }}%
        </div>
        <div class="ap-stat-num" v-else>--</div>
        <div class="ap-stat-label">最近50条占比</div>
      </el-card>
    </div>

    <h4 class="ap-subtitle">半衰期参数（天）</h4>
    <div class="ap-sliders">
      <div class="ap-slider-item">
        <span class="ap-slider-label">情景记忆</span>
        <el-slider
          :model-value="halflifeEpisodic"
          :min="7"
          :max="90"
          :step="1"
          show-input
          disabled
        />
      </div>
      <div class="ap-slider-item">
        <span class="ap-slider-label">用户画像</span>
        <el-slider
          :model-value="halflifeProfile"
          :min="30"
          :max="180"
          :step="1"
          show-input
          disabled
        />
      </div>
      <div class="ap-slider-item">
        <span class="ap-slider-label">结构化事实</span>
        <el-slider
          :model-value="halflifeFact"
          :min="60"
          :max="365"
          :step="1"
          show-input
          disabled
        />
      </div>
      <div class="ap-slider-item">
        <span class="ap-slider-label">世界书</span>
        <el-slider
          :model-value="halflifeWorldbook"
          :min="180"
          :max="730"
          :step="1"
          show-input
          disabled
        />
      </div>
    </div>

    <h4 class="ap-subtitle">最近检索日志</h4>
    <el-table
      :data="retrievalLogs"
      size="small"
      max-height="300"
      style="width: 100%"
    >
      <el-table-column
        prop="queryText"
        label="查询文本"
        min-width="180"
        show-overflow-tooltip
      />
      <el-table-column label="检索记忆数" width="100">
        <template #default="{ row }">
          {{ parseMemIDs(row.retrievedMemoryIDs).length }}
        </template>
      </el-table-column>
      <el-table-column label="最高分" width="80">
        <template #default="{ row }">
          {{ maxScore(row.scoringDetails) }}
        </template>
      </el-table-column>
      <el-table-column prop="createdAt" label="时间" width="160">
        <template #default="{ row }">{{ fmtDate(row.createdAt) }}</template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  retrievalStats: { totalCount: number };
  retrievalLogs: any[];
  halflifeEpisodic: number;
  halflifeProfile: number;
  halflifeFact: number;
  halflifeWorldbook: number;
}>();

function fmtDate(d: string) {
  if (!d) return "";
  try {
    return new Date(d).toLocaleString("zh-CN");
  } catch {
    return d;
  }
}
function parseMemIDs(raw: string): string[] {
  if (!raw) return [];
  try {
    return JSON.parse(raw);
  } catch {
    return [];
  }
}
function maxScore(raw: string): string {
  if (!raw) return "--";
  try {
    const arr = JSON.parse(raw);
    if (!Array.isArray(arr) || arr.length === 0) return "--";
    const m = Math.max(...arr.map((x: any) => x.score || 0));
    return (m * 100).toFixed(1) + "%";
  } catch {
    return "--";
  }
}
</script>

<style scoped>
.analysis-panel {
  padding: 4px 0;
}
.ap-title {
  font-size: 16px;
  font-weight: 600;
  margin-bottom: 12px;
}
.ap-stats-row {
  display: flex;
  gap: 16px;
  margin-bottom: 20px;
}
.ap-stat-card {
  flex: 1;
  text-align: center;
}
.ap-stat-num {
  font-size: 28px;
  font-weight: 700;
  color: var(--ac-color-primary);
}
.ap-stat-label {
  font-size: 13px;
  color: var(--ac-color-text-muted);
  margin-top: 4px;
}
.ap-subtitle {
  font-size: 14px;
  font-weight: 600;
  margin: 16px 0 10px;
}
.ap-sliders {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 16px;
}
.ap-slider-item {
  flex: 1;
  min-width: 200px;
  display: flex;
  align-items: center;
  gap: 10px;
}
.ap-slider-label {
  font-size: 13px;
  white-space: nowrap;
  min-width: 80px;
}
</style>
