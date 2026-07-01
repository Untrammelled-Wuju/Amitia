<!-- SPDX-FileCopyrightText: 2026 Peng Xu -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<template>
    <el-dialog v-model="visible" title="\u751f\u6210\u5019\u9009" width="500px" destroy-on-close>
      <el-form label-position="top">
        <el-form-item label="\u9009\u62e9\u4f1a\u8bdd">
          <el-select v-model="generateConvId" placeholder="\u9009\u62e9\u4f1a\u8bdd" filterable style="width:100%">
            <el-option v-for="c in conversationList" :key="c.id" :label="c.title" :value="c.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="visible = false">\u53d6\u6d88</el-button>
        <el-button type="primary" @click="generateCandidates" :loading="generating">\u751f\u6210</el-button>
      </template>
    </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed } from "vue"
import { ElMessage } from "element-plus"
import { useApi } from "../../../composables/useApi"
const { post } = useApi()
const props = defineProps<{ modelValue: boolean; conversationList: any[]; candidates: any[] }>()
const emit = defineEmits<{ "update:modelValue": [value: boolean]; "update:candidates": [value: any[]]; "show-candidates": [] }>()
const visible = computed({ get: () => props.modelValue, set: (v) => emit("update:modelValue", v) })
const generateConvId = ref("")
const generating = ref(false)
async function generateCandidates() {
  if (!generateConvId.value) { ElMessage.warning("\u8bf7\u9009\u62e9\u4f1a\u8bdd"); return }
  generating.value = true
  try {
    const res: any = await post("/api/memory-candidates/generate", { conversationId: generateConvId.value })
    const items = res?.candidates || []
    emit("update:candidates", items)
    if (items.length > 0) {
      visible.value = false
      emit("show-candidates")
      ElMessage.success("\u5df2\u83b7\u53d6 " + items.length + " \u6761\u5019\u9009\u8bb0\u5fc6")
    } else {
      ElMessage.info("\u672a\u83b7\u53d6\u5230\u5019\u9009\u8bb0\u5fc6")
    }
  } catch (err: any) {
    ElMessage.error(err?.message || "\u83b7\u53d6\u5931\u8d25")
  }
  generating.value = false
}
</script>

<style scoped>
</style>