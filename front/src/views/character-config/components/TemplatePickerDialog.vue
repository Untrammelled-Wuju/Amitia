<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-dialog v-model="show" title="从模板创建角色" width="720px" top="5vh">
    <div v-if="loading" style="text-align:center;padding:40px">
      <el-icon class="is-loading" :size="32"><Loading /></el-icon>
      <p style="margin-top:12px;color:var(--ac-color-text-muted)">加载模板中...</p>
    </div>
    <div v-else-if="templates.length === 0" style="text-align:center;padding:40px">
      <el-empty description="暂无可用模板" :image-size="60" />
    </div>
    <div v-else class="template-grid">
      <div
        v-for="tpl in templates"
        :key="tpl.id"
        class="template-card"
        @click="emit('select', tpl)"
      >
        <div class="tpl-card-header">
          <span class="tpl-card-name">{{ tpl.name }}</span>
          <el-tag v-if="tpl.hasSafeBoundaries" type="success" size="small" effect="plain">已审查</el-tag>
        </div>
        <div class="tpl-card-scenario">{{ tpl.scenario }}</div>
        <div class="tpl-card-details">
          <div class="tpl-detail-row">
            <span class="tpl-detail-label">说话风格</span>
            <span class="tpl-detail-value">{{ tpl.speakingStyle }}</span>
          </div>
          <div class="tpl-detail-row">
            <span class="tpl-detail-label">关系氛围</span>
            <span class="tpl-detail-value">{{ tpl.relationshipStyle }}</span>
          </div>
        </div>
      </div>
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { Loading } from "@element-plus/icons-vue"
import type { TemplateItem } from "../composables/useCharacterConfig"

const props = defineProps<{ modelValue: boolean; templates: TemplateItem[]; loading: boolean }>()
const emit = defineEmits<{
  (e: "update:modelValue", v: boolean): void
  (e: "select", tpl: TemplateItem): void
}>()
const show = computed({ get: () => props.modelValue, set: (v) => emit("update:modelValue", v) })
</script>
