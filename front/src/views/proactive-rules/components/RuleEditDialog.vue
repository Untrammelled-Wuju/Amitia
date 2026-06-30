<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-dialog
    v-model="visible"
    :title="isEditing ? '编辑规则' : '新建规则'"
    width="560px"
    destroy-on-close
    :close-on-click-modal="false"
  >
    <el-form :model="form" label-position="top" size="small">
      <el-form-item label="规则名称" required>
        <el-input v-model="form.name" placeholder="例如：早安问候" maxlength="50" />
      </el-form-item>

      <el-row :gutter="16">
        <el-col :span="12">
          <el-form-item label="规则类型" required>
            <el-select v-model="form.ruleType" style="width:100%">
              <el-option v-for="t in ruleTypes" :key="t.value" :label="t.label" :value="t.value" />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="发送渠道">
            <el-select v-model="form.channel" style="width:100%">
              <el-option label="全部平台" value="all" />
              <el-option label="Web 端" value="web" />
              <el-option label="微信" value="wechat" />
            </el-select>
          </el-form-item>
        </el-col>
      </el-row>

      <el-row :gutter="16">
        <el-col :span="12">
          <el-form-item label="目标会话">
            <el-select v-model="form.conversationId" placeholder="选择会话" clearable filterable style="width:100%">
              <el-option v-for="c in conversations" :key="c.id" :label="c.title" :value="c.id" />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="角色">
            <el-select v-model="form.characterId" placeholder="选择角色" clearable filterable style="width:100%">
              <el-option v-for="ch in characters" :key="ch.id" :label="ch.name" :value="ch.id" />
            </el-select>
          </el-form-item>
        </el-col>
      </el-row>

      <el-form-item label="Cron 表达式">
        <el-input v-model="form.scheduleCron" placeholder="0 9 * * * （每天9点）">
          <template #append>
            <el-tooltip content="分 时 日 月 周，例如 0 9 * * * 表示每天9:00" placement="top">
              <el-icon><QuestionFilled /></el-icon>
            </el-tooltip>
          </template>
        </el-input>
      </el-form-item>

      <el-row :gutter="16">
        <el-col :span="12">
          <el-form-item label="安静开始">
            <el-time-picker v-model="form.quietStart" format="HH:mm" value-format="HH:mm" style="width:100%" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="安静结束">
            <el-time-picker v-model="form.quietEnd" format="HH:mm" value-format="HH:mm" style="width:100%" />
          </el-form-item>
        </el-col>
      </el-row>

      <el-form-item label="每日最大发送">
        <el-input-number v-model="form.maxPerDay" :min="1" :max="24" />
      </el-form-item>

      <el-form-item label="提示词模板">
        <el-input v-model="form.promptTemplate" type="textarea" :rows="4" placeholder="AI 生成消息的提示词模板" />
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="emit('save')">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { QuestionFilled } from "@element-plus/icons-vue"

const props = defineProps<{
  modelValue: boolean
  isEditing: boolean
  form: any
  saving: boolean
  ruleTypes: { value: string; label: string }[]
  conversations: any[]
  characters: any[]
}>()

const emit = defineEmits<{
  (e: "update:modelValue", v: boolean): void
  (e: "save"): void
}>()

const visible = computed({ get: () => props.modelValue, set: (v) => emit("update:modelValue", v) })
</script>
