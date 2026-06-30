<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="import-view">
    <div class="page-header"><h2>导入聊天记录</h2></div>
    <el-card>
      <el-form label-width="100px">
        <el-form-item label="导入类型">
          <el-radio-group v-model="importType">
            <el-radio value="plaintext">纯文本</el-radio>
            <el-radio value="wechat">微信聊天记录</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="目标角色">
          <el-select v-model="characterId" placeholder="选择角色" style="width: 300px">
            <el-option v-for="c in characters" :key="c.id" :label="c.name" :value="c.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="聊天文本">
          <el-input v-model="rawText" type="textarea" :rows="12" :placeholder="importType === 'wechat' ? '粘贴微信聊天记录,每行一条消息...' : '粘贴纯文本对话,每行一条消息...'" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="doImport" :loading="importing">开始导入</el-button>
        </el-form-item>
      </el-form>
    </el-card>
    <div v-if="result" class="import-result">
      <el-alert :title="result" type="success" show-icon :closable="false" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue"
import axios from "axios"
import type { Character, ApiResponse, ImportResult } from "@/types"
import { ElMessage } from "element-plus"

const API = "http://127.0.0.1:8899"
const characters = ref<Character[]>([])
const importType = ref("plaintext")
const characterId = ref("")
const rawText = ref("")
const importing = ref(false)
const result = ref("")

onMounted(async () => {
  const { data } = await axios.get<ApiResponse<Character[]>>(API + "/api/characters")
  if (data.code === 200 && data.data) characters.value = data.data
})

async function doImport() {
  if (!characterId.value) { ElMessage.warning("请选择目标角色"); return }
  if (!rawText.value.trim()) { ElMessage.warning("请输入聊天文本"); return }
  importing.value = true
  try {
    const { data } = await axios.post<ApiResponse<ImportResult>>(API + "/api/import", {
      source: importType.value,
      characterId: characterId.value,
      raw: rawText.value,
    })
    if (data.code === 200) {
      result.value = data.message || "成功导入 " + (data.data?.messageCount || 0) + " 条消息"
      rawText.value = ""
    } else {
      ElMessage.error(data.message || "导入失败")
    }
  } catch (err: any) {
    ElMessage.error("导入失败: " + err.message)
  } finally {
    importing.value = false
  }
}
</script>

<style scoped>
.import-view { padding: 20px; }
.page-header { margin-bottom: 16px; }
.page-header h2 { font-size: 18px; font-weight: 600; }
.import-result { margin-top: 16px; }
</style>
