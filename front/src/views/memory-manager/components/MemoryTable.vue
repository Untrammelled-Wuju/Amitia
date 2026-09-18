<!-- SPDX-FileCopyrightText: 2026 Peng Xu -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<template>
  <el-table
    ref="tableRef"
    :data="memories"
    stripe
    size="small"
    style="margin-top: 10px"
    @selection-change="emit('selection-change', $event)"
  >
    <el-table-column type="selection" width="36" />
    <el-table-column
      prop="key"
      label="关键词"
      width="140"
      show-overflow-tooltip
    />
    <el-table-column prop="value" label="内容" show-overflow-tooltip />
    <el-table-column label="类型" width="90">
      <template #default="{ row }">
        <el-tag size="small" type="info">{{
          typeLabel(row.memoryType)
        }}</el-tag>
      </template>
    </el-table-column>
    <el-table-column label="记忆保持" width="165" sortable prop="retentionLevel">
      <template #default="{ row }">
        <div class="retention-cell">
          <el-tag size="small" :type="retentionTagType(row)">L{{ retentionLevel(row) }}</el-tag>
          <span>{{ retentionStrength(row) }}%</span>
          <el-tag v-if="row.pinned" size="small" type="danger">固定</el-tag>
          <el-tag v-else-if="row.decayState === 'archived'" size="small" type="info">归档</el-tag>
          <el-tag v-else-if="row.decayState === 'fading'" size="small" type="warning">淡化</el-tag>
        </div>
      </template>
    </el-table-column>
    <el-table-column label="记忆活动" width="155">
      <template #default="{ row }">
        <div class="activity-cell">
          <span>强化 {{ Number(row.reinforceCount ?? row.reinforce_count ?? 0) }} 次</span>
          <span>召回 {{ Number(row.retrievedCount ?? row.retrieved_count ?? 0) }} · 注入 {{ Number(row.injectedCount ?? row.injected_count ?? 0) }}</span>
          <span v-if="row.lastReinforcedAt || row.last_reinforced_at" class="activity-time">上次 {{ fmtDate(row.lastReinforcedAt || row.last_reinforced_at) }}</span>
        </div>
      </template>
    </el-table-column>
    <el-table-column label="来源" width="80">
      <template #default="{ row }">
        <span class="source-badge" :class="row.source">{{
          sourceLabel(row.source)
        }}</span>
      </template>
    </el-table-column>
    <el-table-column label="重要度" width="90" sortable prop="importance">
      <template #default="{ row }">
        <el-progress
          :percentage="row.importance * 10"
          :stroke-width="6"
          :show-text="false"
          :color="importanceColor(row.importance)"
        />
        <span style="font-size: 11px; margin-left: 4px"
          >{{ row.importance }}/10</span
        >
      </template>
    </el-table-column>
    <el-table-column label="置信度" width="100" sortable prop="confidence">
      <template #default="{ row }">
        <div style="display: flex; align-items: center; gap: 4px">
          <el-progress
            :percentage="row.confidence ?? 50"
            :stroke-width="6"
            :show-text="false"
            :color="
              (row.confidence ?? 50) >= 80
                ? 'var(--ac-color-success)'
                : (row.confidence ?? 50) >= 50
                  ? 'var(--ac-color-warning)'
                  : 'var(--ac-color-danger)'
            "
          />
          <span style="font-size: 11px">{{ row.confidence ?? 50 }}%</span>
        </div>
      </template>
    </el-table-column>
    <el-table-column label="核实状态" width="90" sortable prop="verifiedStatus">
      <template #default="{ row }">
        <el-tag v-if="isExpired(row.expiresAt)" type="info" size="small"
          >已过期</el-tag
        >
        <el-tag
          v-else-if="row.verifiedStatus === 'user_verified'"
          type="success"
          size="small"
          >已确认</el-tag
        >
        <el-tag
          v-else-if="row.verifiedStatus === 'auto_confirmed'"
          type="warning"
          size="small"
          >自动确认</el-tag
        >
        <el-tag
          v-else-if="row.verifiedStatus === 'contradicted'"
          type="danger"
          size="small"
          >有矛盾</el-tag
        >
        <el-tag v-else type="info" size="small">未核实</el-tag>
      </template>
    </el-table-column>
    <el-table-column label="范围" width="220">
      <template #default="{ row }">
        <el-tag size="small" :type="scopeTypeTagType(row)">{{
          scopeTypeLabel(row)
        }}</el-tag>
        <span
          v-if="rowScopeType(row) === 'user_character' && row.characterId"
          class="scope-char-name"
          >{{ charName(row.characterId) }}</span
        >
        <el-button
          v-if="rowScopeType(row) === 'user_character'"
          text
          size="small"
          type="warning"
          class="scope-toggle-btn"
          @click="emit('toggle-scope', row)"
          >升级为全局</el-button
        >
        <el-button
          v-if="rowScopeType(row) === 'user_global'"
          text
          size="small"
          type="info"
          class="scope-toggle-btn"
          @click="emit('toggle-scope', row)"
          >降级为角色</el-button
        >
      </template>
    </el-table-column>
    <el-table-column label="权限" width="180">
      <template #default="{ row }">
        <div class="permission-tags">
          <el-tag
            size="small"
            :type="sensitivityTagType(rowSensitivity(row))"
            >{{ sensitivityLabel(rowSensitivity(row)) }}</el-tag
          >
          <el-tag
            size="small"
            :type="rowAllowContextUse(row) ? 'success' : 'info'"
            >{{ rowAllowContextUse(row) ? "可理解" : "禁上下文" }}</el-tag
          >
          <el-tag
            size="small"
            :type="rowAllowProactiveMention(row) ? 'warning' : 'info'"
            >{{
              rowAllowProactiveMention(row) ? "可主动提" : "禁主动提"
            }}</el-tag
          >
          <el-tag v-if="rowRequiresConfirmation(row)" size="small" type="danger"
            >需确认</el-tag
          >
        </div>
      </template>
    </el-table-column>
    <el-table-column label="操作" width="245" fixed="right">
      <template #default="{ row }">
        <el-button text size="small" @click="emit('edit', row)">编辑</el-button>
        <el-button
          text
          size="small"
          :type="row.pinned ? 'info' : 'warning'"
          @click="emit('toggle-pin', row)"
          >{{ row.pinned ? "取消固定" : "固定" }}</el-button
        >
        <el-button
          v-if="row.decayState === 'archived' || row.decay_state === 'archived'"
          text
          size="small"
          type="success"
          @click="emit('restore', row)"
          >恢复</el-button
        >
        <el-button
          text
          size="small"
          type="danger"
          @click="emit('delete', row.id)"
          >删除</el-button
        >
      </template>
    </el-table-column>
  </el-table>

  <el-pagination
    v-if="total > pageSize"
    :current-page="page"
    :page-size="pageSize"
    :total="total"
    layout="prev,pager,next"
    @current-change="emit('page-change', $event)"
    style="margin-top: 12px; justify-content: center"
  />
</template>

<script setup lang="ts">
import { ref } from "vue";

const props = defineProps<{
  memories: any[];
  selectedIds: string[];
  page: number;
  pageSize: number;
  total: number;
  characters?: any[];
}>();
const emit = defineEmits<{
  "selection-change": [rows: any[]];
  edit: [row: any];
  delete: [id: string];
  restore: [row: any];
  "toggle-pin": [row: any];
  "toggle-scope": [row: any];
  "page-change": [page: number];
}>();

const tableRef = ref<any>(null);

const TYPES = [
  { value: "personal_info", label: "个人信息" },
  { value: "hobby", label: "爱好" },
  { value: "preference", label: "偏好" },
  { value: "fact", label: "事实" },
  { value: "plan", label: "计划" },
  { value: "habit", label: "习惯" },
  { value: "relationship", label: "关系" },
  { value: "custom", label: "自定义" },
];
const SOURCES = [
  { value: "manual", label: "\u624b\u52a8" },
  { value: "chat", label: "\u804a\u5929" },
  { value: "system", label: "\u7cfb\u7edf" },
  { value: "import", label: "\u5bfc\u5165" },
  { value: "candidate", label: "\u5019\u9009" },
];
const SCOPE_TYPES = [
  { value: "user_character", label: "\u89d2\u8272\u8bb0\u5fc6" },
  { value: "user_global", label: "\u5168\u5c40\u8bb0\u5fc6" },
  { value: "world", label: "\u4e16\u754c\u89c4\u5219" },
  { value: "character_self", label: "\u89d2\u8272\u81ea\u8bc6" },
];
const SENSITIVITY_OPTIONS = [
  { value: "public", label: "公开" },
  { value: "internal", label: "内部" },
  { value: "private", label: "私密" },
  { value: "secret", label: "高度敏感" },
];

function retentionLevel(row: any) {
  const level = Number(row?.retentionLevel ?? row?.retention_level ?? 3);
  return Math.min(5, Math.max(1, Number.isFinite(level) ? level : 3));
}
function retentionStrength(row: any) {
  const raw = Number(row?.memoryStrength ?? row?.memory_strength ?? 0.68);
  return Math.round(Math.min(1, Math.max(0, Number.isFinite(raw) ? raw : 0.68)) * 100);
}
function retentionTagType(row: any) {
  const level = retentionLevel(row);
  return level <= 2 ? "success" : level === 3 ? "primary" : level === 4 ? "warning" : "info";
}
function typeLabel(t: string) {
  return TYPES.find((x) => x.value === t)?.label || t;
}
function charName(id: string) {
  return props.characters?.find((x: any) => x.id === id)?.name || id;
}
function sourceLabel(s: string) {
  return SOURCES.find((x) => x.value === s)?.label || s;
}
function importanceColor(v: number) {
  return v >= 8
    ? "var(--ac-color-danger)"
    : v >= 5
      ? "var(--ac-color-warning)"
      : "var(--ac-color-primary)";
}
function isExpired(exp?: string) {
  return !!exp && new Date(exp).getTime() < Date.now();
}
function rowScopeType(row: any) {
  return (
    row.scopeType ||
    row.scope_type ||
    (row.scope === "user" ? "user_global" : "user_character")
  );
}
function scopeTypeLabel(row: any) {
  return (
    SCOPE_TYPES.find((x) => x.value === rowScopeType(row))?.label ||
    rowScopeType(row)
  );
}
function rowSensitivity(row: any) {
  return (
    row.sensitivity || row.sensitivityLevel || row.sensitivity_level || "internal"
  );
}
function sensitivityLabel(v: string) {
  return SENSITIVITY_OPTIONS.find((x) => x.value === v)?.label || v;
}
function sensitivityTagType(v: string) {
  return v === "secret" ? "danger" : v === "private" ? "warning" : v === "public" ? "success" : "info";
}
function readBooleanFlag(row: any, keys: string[], def: boolean): boolean {
  for (const key of keys) {
    const v = row?.[key];
    if (typeof v === "boolean") return v;
    if (typeof v === "number") return v !== 0;
    if (typeof v === "string") {
      const n = v.trim().toLowerCase();
      if (["true", "1"].includes(n)) return true;
      if (["false", "0"].includes(n)) return false;
    }
  }
  return def;
}
function rowAllowContextUse(row: any) {
  return readBooleanFlag(row, ["allowContextUse", "allow_context_use"], true);
}
function rowAllowProactiveMention(row: any) {
  return readBooleanFlag(
    row,
    ["allowProactiveMention", "allow_proactive_mention"],
    false,
  );
}
function rowRequiresConfirmation(row: any) {
  return readBooleanFlag(
    row,
    ["requiresConfirmation", "requires_confirmation"],
    false,
  );
}
function scopeTypeTagType(row: any) {
  return rowScopeType(row) === "user_global" || rowScopeType(row) === "world"
    ? "success"
    : rowScopeType(row) === "character_self"
      ? "warning"
      : "info";
}
function fmtDate(d: string) {
  if (!d) return "";
  try {
    return new Date(d).toLocaleString("zh-CN");
  } catch {
    return d;
  }
}
</script>

<style scoped>
.source-badge {
  font-size: var(--ac-font-size-xs);
  padding: 1px 6px;
  border-radius: 4px;
  background: var(--ac-color-bg-secondary);
}
.scope-char-name {
  font-size: 11px;
  color: var(--ac-color-text-muted);
  margin-left: 4px;
}
.scope-toggle-btn {
  margin-left: 4px !important;
  text-decoration: underline !important;
}
.retention-cell {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px;
  font-size: 11px;
}
.activity-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 11px;
  color: var(--ac-color-text-secondary);
}
.activity-time {
  color: var(--ac-color-text-muted);
}
.permission-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
</style>
