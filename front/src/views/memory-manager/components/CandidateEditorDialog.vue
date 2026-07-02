<!-- SPDX-FileCopyrightText: 2026 Peng Xu -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<template>
    <el-dialog v-model="visible" title="编辑候选记忆" width="480px" destroy-on-close>
      <el-form label-position="top">
        <el-form-item label="关键词"><el-input v-model="editForm.key" /></el-form-item>
        <el-form-item label="内容"><el-input v-model="editForm.content" type="textarea" :rows="3" /></el-form-item>
        <el-form-item label="类型">
          <el-select v-model="editForm.memoryType" style="width:100%">
            <el-option v-for="t in TYPES" :key="t.value" :label="t.label" :value="t.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="重要度">
          <el-slider v-model="editForm.importance" :max="10" show-input />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="visible = false">取消</el-button>
        <el-button type="primary" @click="saveEditCandidate" :loading="saving">保存</el-button>
      </template>
    </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, computed } from "vue"
import { ElMessage } from "element-plus"
import { useApi } from "../../../composables/useApi"
const { put } = useApi()
const emit = defineEmits<{ "update:modelValue": [value: boolean]; "candidate-updated": [] }>()
const props = defineProps<{ modelValue: boolean }>()
const visible = computed({ get: () => props.modelValue, set: (v) => emit("update:modelValue", v) })
const TYPES = [{ value: "custom", label: "自定义" }, { value: "fact", label: "事实" }, { value: "preference", label: "偏好" }, { value: "experience", label: "经历" }, { value: "rule", label: "规则" }, { value: "belief", label: "信念" }, { value: "emotion", label: "情感" }, { value: "skill", label: "技能" }]
const editForm = reactive({ key: "", value: "", content: "", memoryType: "custom", importance: 5, candidateId: "" })
const saving = ref(false)
async function saveEditCandidate() {
  if (!editForm.candidateId) return
  saving.value = true
  try {
    await put("/api/memory-candidates/" + editForm.candidateId, {
      key: editForm.key,
      value: editForm.content,
      memoryType: editForm.memoryType,
      importance: editForm.importance,
    })
    ElMessage.success("已更新")
    emit("candidate-updated")
    emit("update:modelValue", false)
  } catch (err: any) {
    ElMessage.error(err?.message || "更新失败")
  }
  saving.value = false
}
function setEditData(c: any) {
  editForm.key = c.key
  editForm.value = c.value
  editForm.content = c.value
  editForm.memoryType = c.memoryType
  editForm.importance = c.importance
  editForm.candidateId = c.id
}
defineExpose({ setEditData })
</script>

<style scoped>
</style>
