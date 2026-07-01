<!-- SPDX-FileCopyrightText: 2026 Peng Xu -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<template>
    <el-dialog v-model="dialogVisible" :title="editing ? '编辑记忆' : '新建记忆'" width="480px" destroy-on-close>
      <el-form :model="form" label-position="top">
        <el-form-item label="关键词"><el-input v-model="form.key" placeholder="例如: 喜欢的音乐" /></el-form-item>
        <el-form-item label="内容"><el-input v-model="form.value" type="textarea" :rows="3" placeholder="例如: 喜欢星期六下午听轻音乐" /></el-form-item>
        <el-form-item label="类型">
          <el-select v-model="form.memoryType" style="width:100%"><el-option v-for="t in TYPES" :key="t.value" :label="t.label" :value="t.value" /></el-select>
        </el-form-item>
        <el-form-item label="重要度">
          <el-slider v-model="form.importance" :max="10" show-input :marks="{1:'低',5:'中',10:'高'}" />
        </el-form-item>
        <el-form-item label="范围">
          <el-select v-model="form.scopeType" style="width:100%">
            <el-option v-for="s in SCOPE_TYPES" :key="s.value" :label="s.label" :value="s.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="敏感等级">
          <el-select v-model="form.sensitivity" style="width:100%">
            <el-option v-for="s in SENSITIVITY_OPTIONS" :key="s.value" :label="s.label" :value="s.value" />
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
        <el-button @click="dialogVisible=false">取消</el-button>
        <el-button type="primary" @click="saveMem" :loading="saving">保存</el-button>
      </template>
    </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, computed } from "vue"
import { ElMessage } from "element-plus"
import { useApi } from "../../../composables/useApi"
const { post, put } = useApi()
const emit = defineEmits<{ "update:modelValue": [value: boolean]; "memory-saved": [] }>()
const props = defineProps<{ modelValue: boolean; editing: boolean; editingId: string; characterId: string }>()
const visible = computed({ get: () => props.modelValue, set: (v) => emit("update:modelValue", v) })
const TYPES = [{ value: "custom", label: "自定义" }, { value: "fact", label: "事实" }, { value: "preference", label: "偏好" }, { value: "experience", label: "经历" }, { value: "rule", label: "规则" }, { value: "belief", label: "信念" }, { value: "emotion", label: "情感" }, { value: "skill", label: "技能" }]
const SCOPE_TYPES = [{ value: "user_character", label: "角色记忆" }, { value: "user_global", label: "全局记忆" }, { value: "world", label: "世界规则" }, { value: "character_self", label: "角色自识" }]
const SENSITIVITY_OPTIONS = [{ value: "normal", label: "普通" }, { value: "sensitive", label: "较敏感" }, { value: "high", label: "高度敏感" }]
const form = reactive({ key: "", value: "", memoryType: "custom", importance: 5, characterId: "", source: "manual", scope: "character", scopeType: "user_character", sensitivity: "normal", allowContextUse: true, allowProactiveMention: false, requiresConfirmation: false })
const saving = ref(false)
function scopeTypeToScope(scopeType: string) { return scopeType === "user_global" ? "user" : scopeType === "world" ? "world" : "character" }
function initCreate() {
  form.key = ""; form.value = ""; form.memoryType = "custom"; form.importance = 5
  form.characterId = props.characterId; form.source = "manual"; form.scope = "character"
  form.scopeType = "user_character"; form.sensitivity = "normal"
  form.allowContextUse = true; form.allowProactiveMention = false; form.requiresConfirmation = false
}
function initEdit(row: any) {
  form.key = row.key; form.value = row.value; form.memoryType = row.memoryType; form.importance = row.importance
  form.characterId = row.characterId || ""; form.scope = row.scope || "character"
  form.scopeType = row.scopeType || row.scope_type || (row.scope === "user" ? "user_global" : "user_character")
  form.source = row.source || "manual"; form.sensitivity = row.sensitivity || row.sensitivityLevel || row.sensitivity_level || "normal"
  form.allowContextUse = readFlag(row, ["allowContextUse", "allow_context_use"], true)
  form.allowProactiveMention = readFlag(row, ["allowProactiveMention", "allow_proactive_mention"], false)
  form.requiresConfirmation = readFlag(row, ["requiresConfirmation", "requires_confirmation"], false)
}
function readFlag(row: any, keys: string[], defaultVal: boolean): boolean {
  for (const key of keys) {
    const value = row?.[key]
    if (typeof value === "boolean") return value
    if (typeof value === "number") return value !== 0
    if (typeof value === "string") { const n = value.trim().toLowerCase(); if (["true", "1"].includes(n)) return true; if (["false", "0"].includes(n)) return false }
  }
  return defaultVal
}
async function saveMem() {
  saving.value = true
  try {
    const payload = { ...form, source: form.source || "manual", scope: scopeTypeToScope(form.scopeType), allowContextUse: !!form.allowContextUse, allowProactiveMention: !!form.allowProactiveMention, requiresConfirmation: !!form.requiresConfirmation }
    if (props.editingId) await put("/api/memories/" + props.editingId, payload)
    else await post("/api/memories", payload)
    emit("update:modelValue", false)
    emit("memory-saved")
    ElMessage.success(props.editingId ? "保存成功" : "新建成功")
  } catch (err: any) { ElMessage.error(err?.message || "保存失败") }
  saving.value = false
}
defineExpose({ initCreate, initEdit })
</script>

<style scoped>
.permission-switches { display: flex; flex-direction: column; gap: 8px; width: 100%; }
</style>