<!-- SPDX-FileCopyrightText: 2026 Peng Xu -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<template>
  <el-dialog
    v-model="visible"
    :title="editing ? '编辑记忆' : '新建记忆'"
    width="520px"
    destroy-on-close
  >
    <div v-if="editing && memory" class="retention-overview">
      <div class="retention-overview-main">
        <el-tag :type="retentionTagType">L{{ currentRetentionLevel }}</el-tag>
        <strong>{{ currentStrength }}%</strong>
        <el-tag v-if="memory.pinned" type="danger">固定</el-tag>
        <el-tag v-else-if="currentDecayState === 'archived'" type="info">已归档</el-tag>
        <el-tag v-else-if="currentDecayState === 'fading'" type="warning">正在淡化</el-tag>
        <el-tag v-else type="success">活跃</el-tag>
      </div>
      <div class="retention-overview-stats">
        <span>强化 {{ currentReinforceCount }} 次</span>
        <span>召回 {{ currentRetrievedCount }} 次</span>
        <span>注入 {{ currentInjectedCount }} 次</span>
        <span v-if="currentLastReinforcedAt">上次强化：{{ formatDate(currentLastReinforcedAt) }}</span>
      </div>
    </div>

    <el-form :model="form" label-position="top">
      <el-form-item label="关键词">
        <el-input v-model="form.key" placeholder="例如: 喜欢的音乐" />
      </el-form-item>
      <el-form-item label="内容">
        <el-input
          v-model="form.value"
          type="textarea"
          :rows="3"
          placeholder="例如: 喜欢星期六下午听轻音乐"
        />
      </el-form-item>
      <el-form-item label="类型">
        <el-select v-model="form.memoryType" style="width: 100%">
          <el-option
            v-for="t in TYPES"
            :key="t.value"
            :label="t.label"
            :value="t.value"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="记忆子类">
        <el-input
          v-model="form.memorySubtype"
          placeholder="可选，例如 TASTES / PROJECTS / BASIC_PROFILE"
        />
      </el-form-item>
      <el-form-item label="记忆保持">
        <el-select v-model="form.retentionLevel" style="width: 100%">
          <el-option v-if="!editing" :value="0" label="自动（按重要度与子类）" />
          <el-option :value="1" label="L1 核心 / 基本不遗忘" />
          <el-option :value="2" label="L2 稳定长期" />
          <el-option :value="3" label="L3 普通长期" />
          <el-option :value="4" label="L4 弱记忆" />
          <el-option :value="5" label="L5 短暂记忆" />
        </el-select>
        <el-checkbox v-model="form.pinned" style="margin-top: 8px">
          固定记忆（固定后按 L1 处理）
        </el-checkbox>
      </el-form-item>
      <el-form-item label="重要度">
        <el-slider
          v-model="form.importance"
          :max="10"
          show-input
          :marks="{ 1: '低', 5: '中', 10: '高' }"
        />
      </el-form-item>
      <el-form-item label="范围">
        <el-select v-model="form.scopeType" style="width: 100%">
          <el-option
            v-for="s in SCOPE_TYPES"
            :key="s.value"
            :label="s.label"
            :value="s.value"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="敏感等级">
        <el-select v-model="form.sensitivity" style="width: 100%">
          <el-option
            v-for="s in SENSITIVITY_OPTIONS"
            :key="s.value"
            :label="s.label"
            :value="s.value"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="使用权限">
        <div class="permission-switches">
          <el-checkbox v-model="form.allowContextUse">允许用于上下文理解</el-checkbox>
          <el-checkbox v-model="form.allowProactiveMention">允许主动提及</el-checkbox>
          <el-checkbox v-model="form.requiresConfirmation">使用前需要确认</el-checkbox>
        </div>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button
        v-if="editing && currentDecayState === 'archived'"
        type="success"
        plain
        @click="restoreMemory"
        :loading="restoring"
      >
        恢复归档记忆
      </el-button>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" @click="saveMem" :loading="saving">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { useApi } from "../../../composables/useApi";

const { post, put } = useApi();
const emit = defineEmits<{
  "update:modelValue": [value: boolean];
  "memory-saved": [];
}>();
const props = defineProps<{
  modelValue: boolean;
  editing: boolean;
  editingId: string;
  characterId: string;
  memory?: any | null;
}>();
const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit("update:modelValue", v),
});
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
const SCOPE_TYPES = [
  { value: "user_character", label: "角色记忆" },
  { value: "user_global", label: "全局记忆" },
  { value: "world", label: "世界规则" },
  { value: "character_self", label: "角色自识" },
];
const SENSITIVITY_OPTIONS = [
  { value: "public", label: "公开" },
  { value: "internal", label: "内部" },
  { value: "private", label: "私密" },
  { value: "secret", label: "高度敏感" },
];
const form = reactive({
  key: "",
  value: "",
  memoryType: "custom",
  memorySubtype: "",
  retentionLevel: 0,
  pinned: false,
  importance: 5,
  characterId: "",
  source: "manual",
  scope: "character",
  scopeType: "user_character",
  sensitivity: "internal",
  allowContextUse: true,
  allowProactiveMention: false,
  requiresConfirmation: false,
});
const saving = ref(false);
const restoring = ref(false);

const currentRetentionLevel = computed(() => clampRetention(props.memory?.retentionLevel ?? props.memory?.retention_level ?? 3));
const currentStrength = computed(() => Math.round(clamp01(Number(props.memory?.memoryStrength ?? props.memory?.memory_strength ?? 0.68)) * 100));
const currentDecayState = computed(() => String(props.memory?.decayState ?? props.memory?.decay_state ?? "active"));
const currentReinforceCount = computed(() => Number(props.memory?.reinforceCount ?? props.memory?.reinforce_count ?? 0));
const currentRetrievedCount = computed(() => Number(props.memory?.retrievedCount ?? props.memory?.retrieved_count ?? 0));
const currentInjectedCount = computed(() => Number(props.memory?.injectedCount ?? props.memory?.injected_count ?? 0));
const currentLastReinforcedAt = computed(() => String(props.memory?.lastReinforcedAt ?? props.memory?.last_reinforced_at ?? ""));
const retentionTagType = computed(() => currentRetentionLevel.value <= 2 ? "success" : currentRetentionLevel.value === 3 ? "primary" : currentRetentionLevel.value === 4 ? "warning" : "info");

function clampRetention(value: unknown) {
  const level = Number(value);
  if (!Number.isFinite(level)) return 3;
  return Math.min(5, Math.max(1, Math.round(level)));
}
function clamp01(value: number) {
  if (!Number.isFinite(value)) return 0.68;
  return Math.min(1, Math.max(0, value));
}
function scopeTypeToScope(scopeType: string) {
  return scopeType === "user_global"
    ? "user"
    : scopeType === "world"
      ? "world"
      : "character";
}
function initCreate() {
  Object.assign(form, {
    key: "",
    value: "",
    memoryType: "custom",
    memorySubtype: "",
    retentionLevel: 0,
    pinned: false,
    importance: 5,
    characterId: props.characterId,
    source: "manual",
    scope: "character",
    scopeType: "user_character",
    sensitivity: "internal",
    allowContextUse: true,
    allowProactiveMention: false,
    requiresConfirmation: false,
  });
}
function initEdit(row: any) {
  if (!row) {
    initCreate();
    return;
  }
  Object.assign(form, {
    key: row.key || "",
    value: row.value || "",
    memoryType: row.memoryType || "custom",
    memorySubtype: row.memorySubtype || row.memory_subtype || "",
    retentionLevel: clampRetention(row.retentionLevel ?? row.retention_level ?? 3),
    pinned: readFlag(row, ["pinned"], false),
    importance: Number(row.importance ?? 5),
    characterId: row.characterId || props.characterId || "",
    scope: row.scope || "character",
    scopeType:
      row.scopeType ||
      row.scope_type ||
      (row.scope === "user" ? "user_global" : row.scope === "world" ? "world" : "user_character"),
    source: row.source || "manual",
    sensitivity:
      row.sensitivity || row.sensitivityLevel || row.sensitivity_level || "internal",
    allowContextUse: readFlag(row, ["allowContextUse", "allow_context_use"], true),
    allowProactiveMention: readFlag(row, ["allowProactiveMention", "allow_proactive_mention"], false),
    requiresConfirmation: readFlag(row, ["requiresConfirmation", "requires_confirmation"], false),
  });
}
function readFlag(row: any, keys: string[], defaultVal: boolean): boolean {
  for (const key of keys) {
    const value = row?.[key];
    if (typeof value === "boolean") return value;
    if (typeof value === "number") return value !== 0;
    if (typeof value === "string") {
      const normalized = value.trim().toLowerCase();
      if (["true", "1"].includes(normalized)) return true;
      if (["false", "0"].includes(normalized)) return false;
    }
  }
  return defaultVal;
}
function formatDate(value: string) {
  if (!value) return "";
  try {
    return new Date(value).toLocaleString("zh-CN");
  } catch {
    return value;
  }
}
async function saveMem() {
  if (!form.key.trim() || !form.value.trim()) {
    ElMessage.warning("关键词和内容不能为空");
    return;
  }
  saving.value = true;
  try {
    const payload: Record<string, any> = {
      ...form,
      source: form.source || "manual",
      scope: scopeTypeToScope(form.scopeType),
      sensitivityLevel: form.sensitivity,
      allowContextUse: !!form.allowContextUse,
      allowProactiveMention: !!form.allowProactiveMention,
      requiresConfirmation: !!form.requiresConfirmation,
    };
    if (!props.editingId && payload.retentionLevel === 0) delete payload.retentionLevel;
    if (props.editingId) await put(`/api/memories/${props.editingId}`, payload);
    else await post("/api/memories", payload);
    emit("update:modelValue", false);
    emit("memory-saved");
    ElMessage.success(props.editingId ? "保存成功" : "新建成功");
  } catch (err: any) {
    ElMessage.error(err?.message || "保存失败");
  } finally {
    saving.value = false;
  }
}
async function restoreMemory() {
  if (!props.editingId) return;
  restoring.value = true;
  try {
    await post(`/api/memories/${props.editingId}/restore`);
    emit("update:modelValue", false);
    emit("memory-saved");
    ElMessage.success("记忆已恢复");
  } catch (err: any) {
    ElMessage.error(err?.message || "恢复失败");
  } finally {
    restoring.value = false;
  }
}

watch(
  () => props.modelValue,
  (open) => {
    if (!open) return;
    if (props.editing) initEdit(props.memory);
    else initCreate();
  },
  { immediate: true },
);
watch(
  () => props.memory,
  (memory) => {
    if (props.modelValue && props.editing) initEdit(memory);
  },
);
</script>

<style scoped>
.retention-overview {
  padding: 12px;
  margin-bottom: 14px;
  border: 1px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-md);
  background: var(--ac-color-bg-secondary);
}
.retention-overview-main {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.retention-overview-stats {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  margin-top: 8px;
  font-size: 12px;
  color: var(--ac-color-text-secondary);
}
.permission-switches {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}
</style>
