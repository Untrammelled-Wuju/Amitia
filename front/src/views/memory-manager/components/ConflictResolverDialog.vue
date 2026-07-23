<!-- SPDX-FileCopyrightText: 2026 Peng Xu -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<template>
  <el-dialog
    v-model="visible"
    title="记忆冲突检测"
    width="550px"
    destroy-on-close
    :close-on-click-modal="false"
  >
    <div v-if="conflictList.length === 0">
      <el-form label-position="top">
        <el-form-item label="新记忆类型">
          <el-select v-model="conflictNewType" style="width: 100%">
            <el-option
              v-for="t in TYPES"
              :key="t.value"
              :label="t.label"
              :value="t.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="新记忆内容"
          ><el-input v-model="conflictNewContent" type="textarea" :rows="3"
        /></el-form-item>
        <el-form-item>
          <el-button type="primary" @click="checkConflict" :loading="checking"
            >检查冲突</el-button
          >
        </el-form-item>
      </el-form>
    </div>
    <div v-else>
      <h4 style="margin-bottom: 8px">
        发现 {{ conflictList.length }} 条潜在冲突
      </h4>
      <div v-for="c in conflictList" :key="c.id" class="conflict-item">
        <div class="ci-header">
          <el-tag size="small">{{ typeLabel(c.memoryType) }}</el-tag> 重要度:
          {{ c.importance }}/10
        </div>
        <div class="ci-key">{{ c.key }}</div>
        <div class="ci-value">{{ c.value }}</div>
        <div class="ci-reason">
          <el-tag size="small" type="danger">冲突原因</el-tag> {{ c.reason }}
        </div>
      </div>
      <div style="margin-top: 12px">
        <el-radio-group v-model="resolveAction">
          <el-radio value="keep_new">保留新记忆</el-radio>
          <el-radio value="keep_old">保留旧记忆</el-radio>
          <el-radio value="merge">合并</el-radio>
        </el-radio-group>
      </div>
      <el-button
        type="primary"
        @click="resolveConflict"
        :loading="resolving"
        style="margin-top: 12px"
        >解决冲突</el-button
      >
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed } from "vue";
import { ElMessage } from "element-plus";
import { useApi } from "../../../composables/useApi";
const { post, get } = useApi();
const emit = defineEmits<{
  "update:modelValue": [value: boolean];
  "conflict-resolved": [];
}>();
const props = defineProps<{ modelValue: boolean }>();
const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit("update:modelValue", v),
});
const TYPES = [
  { value: "custom", label: "自定义" },
  { value: "fact", label: "事实" },
  { value: "preference", label: "偏好" },
  { value: "experience", label: "经历" },
  { value: "rule", label: "规则" },
  { value: "belief", label: "信念" },
  { value: "emotion", label: "情感" },
  { value: "skill", label: "技能" },
];
function typeLabel(t: string) {
  return TYPES.find((x) => x.value === t)?.label || t;
}
const conflictNewType = ref("");
const conflictNewContent = ref("");
const conflictList = ref<any[]>([]);
const resolveAction = ref("keep_new");
const checking = ref(false);
const resolving = ref(false);

async function checkConflict() {
  if (!conflictNewContent.value.trim()) return;
  checking.value = true;
  try {
    const result = await post<any>("/api/memories/check-conflict", {
      memoryType: conflictNewType.value || "custom",
      value: conflictNewContent.value.trim(),
    });
    conflictList.value = result?.conflicts || result?.items || [];
    if (conflictList.value.length === 0) ElMessage.success("未发现冲突");
  } catch (err: any) {
    ElMessage.error(err?.message || "冲突检测失败");
  } finally {
    checking.value = false;
  }
}

async function resolveConflict() {
  resolving.value = true;
  try {
    await post("/api/memories/resolve-conflict", {
      action: resolveAction.value,
      conflicts: conflictList.value,
      memoryType: conflictNewType.value || "custom",
      value: conflictNewContent.value.trim(),
    });
    ElMessage.success("冲突已处理");
    conflictList.value = [];
    emit("conflict-resolved");
    emit("update:modelValue", false);
  } catch (err: any) {
    ElMessage.error(err?.message || "冲突处理失败");
  } finally {
    resolving.value = false;
  }
}
</script>

<style scoped>
.conflict-item {
  padding: 10px;
  border: 1px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-sm);
  margin-bottom: 8px;
}
.ci-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}
.ci-key {
  font-weight: 600;
  font-size: var(--ac-font-size-sm);
}
.ci-value {
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-text-secondary);
}
.ci-reason {
  margin-top: 6px;
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-danger);
}
</style>
