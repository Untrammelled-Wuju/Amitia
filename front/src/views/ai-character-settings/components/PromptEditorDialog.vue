<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-dialog v-model="showModel" title="修改角色提示词" width="700px" destroy-on-close>
    <div class="preview-content">
      <el-input v-model="editingPromptModel" type="textarea" :rows="20" class="preview-textarea" />
    </div>
    <template #footer>
      <el-button @click="showModel = false">关闭</el-button>
      <el-button @click="copyPrompt">复制提示词</el-button>
      <el-button type="primary" @click="saveCharPrompt" :loading="promptSaving">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed } from "vue"
import { ElMessage } from "element-plus"
import { useApi } from "../../../composables/useApi"

const props = defineProps<{
  modelValue: boolean
  editingPrompt: string
  charId: string
  charName: string
}>()

const emit = defineEmits<{
  (e: "update:modelValue", v: boolean): void
  (e: "update:editingPrompt", v: string): void
  (e: "saved"): void
}>()

const { post } = useApi()
const promptSaving = ref(false)

const showModel = computed({
  get: () => props.modelValue,
  set: (v) => emit("update:modelValue", v)
})

const editingPromptModel = computed({
  get: () => props.editingPrompt,
  set: (v) => emit("update:editingPrompt", v)
})

async function saveCharPrompt() {
  if (!props.charId) {
    ElMessage.warning("请先保存角色后再编辑提示词")
    return
  }
  promptSaving.value = true
  try {
    await post<any>("/api/ai/character/save", {
      id: props.charId,
      name: props.charName.trim(),
      basePrompt: props.editingPrompt,
    })
    ElMessage.success("提示词已保存")
    emit("update:modelValue", false)
    emit("saved")
  } catch {
    ElMessage.error("保存失败")
  } finally {
    promptSaving.value = false
  }
}

async function copyPrompt() {
  try {
    await navigator.clipboard.writeText(props.editingPrompt)
    ElMessage.success("已复制到剪贴板")
  } catch {
    ElMessage.warning("复制失败，请手动选择复制")
  }
}
</script>
