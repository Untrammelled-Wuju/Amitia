<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-dialog :model-value="modelValue" @update:model-value="emit('update:modelValue', $event)" title="导入角色包" width="560px" destroy-on-close>
    <template v-if="!preview">
      <el-form label-position="top">
        <el-form-item label="角色包名称">
          <el-input v-model="packNameModel" placeholder="输入 data/exports/character-packs/ 下的包名" />
          <div class="form-hint" style="margin-top:4px">角色包位于 data/exports/character-packs/ 目录</div>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="previewing" @click="emit('preview')" :disabled="!packName">
            预览
          </el-button>
        </el-form-item>
      </el-form>

      <div v-if="history.length > 0" style="margin-top:16px">
        <div class="section-label">已有角色包</div>
        <div v-for="p in history" :key="p.name" class="pack-history-item" @click="emit('update:packName', p.name); emit('preview')">
          <span class="phi-name">{{ p.name }}</span>
          <span class="phi-time">{{ p.createdAt?.slice(0,10) }}</span>
        </div>
      </div>
    </template>

    <template v-else>
      <el-alert
        v-if="preview?.risks?.length > 0"
        type="warning" title="风险提示" :closable="false" show-icon style="margin-bottom:12px"
      >
        <template #default>
          <ul style="margin:4px 0;padding-left:16px;font-size:13px">
            <li v-for="r in preview.risks" :key="r.category" :style="{color: r.level==='high'?'var(--el-color-danger)':'var(--el-color-warning)'}">
              [{{ r.level==='high'?'高':'中' }}] {{ r.message }}
            </li>
          </ul>
        </template>
      </el-alert>

      <div class="import-preview-info">
        <div class="ipi-row"><span class="ipi-label">名称</span><strong>{{ preview.name }}</strong></div>
        <div class="ipi-row"><span class="ipi-label">作者</span><span>{{ preview.author }}</span></div>
        <div class="ipi-row"><span class="ipi-label">身份</span><span>{{ preview.identity || '未设置' }}</span></div>
        <div class="ipi-row"><span class="ipi-label">性格</span><span>{{ preview.personality || '未设置' }}</span></div>
        <div class="ipi-row"><span class="ipi-label">说话风格</span><span>{{ preview.speakingStyle || '未设置' }}</span></div>
        <div class="ipi-row"><span class="ipi-label">关系氛围</span><span>{{ preview.relationshipStyle || '未设置' }}</span></div>
        <div class="ipi-row"><span class="ipi-label">边界规则</span><span class="ipi-value-wrap">{{ preview.boundaryRulesSummary }}</span></div>
        <div class="ipi-row"><span class="ipi-label">包含记忆</span><span>{{ preview.hasMemories ? preview.memoryCount + ' 条' : '无' }}</span></div>
        <div class="ipi-row"><span class="ipi-label">安全等级</span>
          <el-tag :type="preview.safetyLevel==='high'?'danger':preview.safetyLevel==='medium'?'warning':'success'" size="small">
            {{ preview.safetyLevel==='high'?'高风险':preview.safetyLevel==='medium'?'中风险':'正常' }}
          </el-tag>
        </div>
      </div>

      <el-divider />
      <div class="confirm-row" style="margin-bottom:8px">
        <span style="font-size:13px">输入 确认导入 以继续：</span>
        <el-input v-model="confirmTextModel" placeholder='输入"确认导入"' style="width:160px" size="small" />
      </div>
      <el-row :gutter="8">
        <el-col :span="12">
          <el-button @click="emit('cancelPreview')" style="width:100%">返回</el-button>
        </el-col>
        <el-col :span="12">
          <el-button type="primary" :disabled="confirmText !== '确认导入'" :loading="importing" @click="emit('confirm')" style="width:100%">
            确认导入
          </el-button>
        </el-col>
      </el-row>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed } from "vue"

const props = defineProps<{
  modelValue: boolean
  packName: string; preview: any; previewing: boolean
  confirmText: string; importing: boolean; history: any[]
}>()

const emit = defineEmits<{
  (e: "update:modelValue", v: boolean): void
  (e: "update:packName", v: string): void
  (e: "update:confirmText", v: string): void
  (e: "preview"): void
  (e: "cancelPreview"): void
  (e: "confirm"): void
}>()

const packNameModel = computed({ get: () => props.packName, set: (v) => emit("update:packName", v) })
const confirmTextModel = computed({ get: () => props.confirmText, set: (v) => emit("update:confirmText", v) })
</script>
