<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="scenario-section">
    <h3 class="section-title">用途分配</h3>
    <p class="section-desc">为不同场景指定使用的模型。未分配时回退到默认模型。</p>

    <div class="scenario-grid">
      <div
        v-for="route in scenarioRoutes"
        :key="route.scenario"
        class="scenario-card"
      >
        <div class="sc-card-header">
          <span class="sc-label">{{ label(route.scenario) }}</span>
          <el-tag v-if="!route.modelConfigId" size="small" type="info">使用默认</el-tag>
          <el-tag v-else size="small" type="success">已分配</el-tag>
        </div>
        <div class="sc-card-body">
          <span class="sc-desc">{{ desc(route.scenario) }}</span>
        </div>
        <div class="sc-card-select">
          <el-select
            v-model="routeAssignments[route.scenario]"
            :placeholder="'使用默认模型'"
            clearable
            size="small"
            style="width:100%"
            @change="(val: number|null) => emit('assign', route.scenario, val)"
          >
            <el-option
              v-for="cfg in configs"
              :key="cfg.id"
              :label="cfg.name + ' (' + cfg.modelName + ')'"
              :value="cfg.id"
            />
          </el-select>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  configs: any[]
  scenarioRoutes: any[]
  routeAssignments: Record<string, number | null>
}>()

const emit = defineEmits<{
  (e: "assign", scenario: string, modelConfigId: number | null): void
}>()

function label(scenario: string): string {
  const labels: Record<string, string> = {
    chat: "聊天对话", summary: "会话摘要", memory_extract: "记忆提取",
    safety_rewrite: "安全改写", import_parse: "导入解析", reply_timing_check: "完整性判断",
  }
  return labels[scenario] || scenario
}

function desc(scenario: string): string {
  const descs: Record<string, string> = {
    chat: "日常聊天和对话回复", summary: "生成对话历史摘要",
    memory_extract: "从对话中提取用户记忆", safety_rewrite: "安全边界内容改写",
    import_parse: "解析导入的聊天记录文本", reply_timing_check: "判断回复用户是否发送完成完整信息",
  }
  return descs[scenario] || ""
}
</script>

<style scoped>
.scenario-section {
  margin-top: 18px;
}

.section-title {
  font-size: var(--ac-font-size-base);
  font-weight: 600;
  margin-bottom: 4px;
  color: var(--ac-color-text);
}

.section-desc {
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-text-muted);
  margin-bottom: 10px;
}

.scenario-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 10px;
}

.scenario-card {
  background: var(--ac-color-surface);
  border: 1px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-md);
  padding: 12px 14px;
}

.sc-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 4px;
}

.sc-label {
  font-size: var(--ac-font-size-sm);
  font-weight: 600;
  color: var(--ac-color-text);
}

.sc-desc {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
  display: block;
  margin-bottom: 8px;
}

@media (max-width: 640px) {
  .scenario-grid {
    grid-template-columns: 1fr;
  }
}
</style>
