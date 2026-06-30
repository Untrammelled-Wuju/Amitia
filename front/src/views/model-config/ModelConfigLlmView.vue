<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div>
    <el-alert type="info" :closable="false" show-icon style="margin-bottom:14px">
      <template #title>
        使用云端模型时，聊天内容会发送到模型服务商。请勿在聊天中发送密码、验证码、银行卡等敏感信息。
      </template>
    </el-alert>

    <div class="toolbar">
      <el-button type="primary" :icon="Plus" @click="showDialog(null)">新增配置</el-button>
    </div>

    <ConfigCardList
      :configs="configs"
      :testing-id="testingId"
      :providers="providers"
      @test="testConfig"
      @edit="showDialog"
      @set-active="setActive"
      @delete="delConfig"
      @add="showDialog(null)"
    />

    <ScenarioAssignment
      v-if="configs.length > 0"
      :configs="configs"
      :scenario-routes="scenarioRoutes"
      :route-assignments="routeAssignments"
      @assign="assignRoute"
    />

    <ConfigEditDialog
      ref="editDialogRef"
      v-model="dialogVisible"
      :editing-id="editingId"
      :form="form"
      :rules="rules"
      :providers="providers"
      :current-provider-schema="currentProviderSchema"
      :detecting-models="detectingModels"
      :detected-models="detectedModels"
      :detect-error="detectError"
      :saving="saving"
      @save="saveConfig"
      @on-provider-change="onProviderChange"
      @detect-models="detectModels"
    />

    <TestResultDialog
      v-model="testResultVisible"
      :test-result="testResult"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, watch, nextTick } from "vue"
import { Plus } from "@element-plus/icons-vue"
import { useModelConfig } from "./composables/useModelConfig"
import ConfigCardList from "./components/ConfigCardList.vue"
import ScenarioAssignment from "./components/ScenarioAssignment.vue"
import ConfigEditDialog from "./components/ConfigEditDialog.vue"
import TestResultDialog from "./components/TestResultDialog.vue"

const modelConfig = useModelConfig()

const {
  configs, providers, currentProviderSchema,
  dialogVisible, detectingModels, detectedModels, detectError,
  editingId, saving,
  testingId, testResultVisible, testResult,
  scenarioRoutes, routeAssignments,
  form, rules,
  showDialog, saveConfig, testConfig, setActive, delConfig,
  onProviderChange, detectModels,
  assignRoute,
} = modelConfig

const editDialogRef = ref<InstanceType<typeof ConfigEditDialog>>()

watch(dialogVisible, async (v) => {
  if (v) {
    await nextTick()
    modelConfig.dialogFormRef.value = editDialogRef.value?.formRef ?? null
  }
})


</script>

<style scoped>
.toolbar {
  display: flex;
  gap: 8px;
  margin-bottom: 14px;
}
</style>

