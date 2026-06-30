<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-tabs v-model="activeTabModel">
    <el-tab-pane label="编辑角色" name="edit">
      <el-form label-position="top" class="char-form">
        <el-row :gutter="12">
          <el-col :span="16">
            <el-form-item label="名称">
              <el-input v-model="nameModel" placeholder="角色名称" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="头像链接">
              <el-input v-model="avatarModel" placeholder="URL（可选）" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item label="身份">
              <el-input v-model="identityModel" placeholder="例如: AI 虚拟陪伴角色" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="性格">
              <el-input v-model="personalityModel" placeholder="例如: 温和、体贴、有耐心" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item label="说话风格">
              <el-input v-model="speakingStyleModel" placeholder="例如: 简短自然、轻声细语" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="关系氛围">
              <el-input v-model="relationshipStyleModel" placeholder="例如: 亲近但保持边界" />
            </el-form-item>
          </el-col>
        </el-row>

        <PersonalitySliders v-model="personalityConfigModel" style="margin-bottom:16px" />

        <el-form-item label="系统提示词 (System Prompt)">
          <div class="textarea-toolbar">
            <el-button text size="small" :icon="FullScreen" @click="emit('showFullPrompt')">全屏编辑</el-button>
            <el-button text size="small" @click="emit('resetPrompt')">恢复默认</el-button>
          </div>
          <el-input v-model="systemPromptModel" type="textarea" :rows="8" placeholder="编写角色的 System Prompt..." />
        </el-form-item>

        <el-form-item label="安全边界规则">
          <div class="textarea-toolbar">
            <el-button text size="small" :icon="FullScreen" @click="emit('showFullBounds')">全屏编辑</el-button>
            <el-button text size="small" @click="emit('resetBounds')">恢复默认</el-button>
          </div>
          <el-input v-model="boundaryRulesModel" type="textarea" :rows="5" placeholder="每行一条规则..." />
        </el-form-item>

        <div class="form-actions">
          <el-checkbox v-model="isActiveModel" :disabled="isActive && !hasOtherActive">
            设为当前启用角色
          </el-checkbox>
          <el-button type="primary" :loading="saving" @click="emit('save')">
            {{ selectedId ? "保存修改" : "创建角色" }}
          </el-button>
        </div>
      </el-form>
    </el-tab-pane>

    <el-tab-pane label="实时测试" name="test">
      <slot name="test" />
    </el-tab-pane>
  </el-tabs>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { FullScreen } from "@element-plus/icons-vue"
import PersonalitySliders, { type PersonalityConfig } from "../../../components/PersonalitySliders.vue"

const props = defineProps<{
  activeTab: string
  name: string; avatar: string; identity: string; personality: string
  speakingStyle: string; relationshipStyle: string
  systemPrompt: string; boundaryRules: string
  personalityConfig: PersonalityConfig
  isActive: boolean; hasOtherActive: boolean
  saving: boolean; selectedId: string
}>()

const emit = defineEmits<{
  (e: "update:activeTab", v: string): void
  (e: "update:name", v: string): void
  (e: "update:avatar", v: string): void
  (e: "update:identity", v: string): void
  (e: "update:personality", v: string): void
  (e: "update:speakingStyle", v: string): void
  (e: "update:relationshipStyle", v: string): void
  (e: "update:systemPrompt", v: string): void
  (e: "update:boundaryRules", v: string): void
  (e: "update:personalityConfig", v: PersonalityConfig): void
  (e: "update:isActive", v: boolean): void
  (e: "showFullPrompt"): void
  (e: "showFullBounds"): void
  (e: "resetPrompt"): void
  (e: "resetBounds"): void
  (e: "save"): void
}>()

const activeTabModel = computed({ get: () => props.activeTab, set: (v) => emit("update:activeTab", v) })
const nameModel = computed({ get: () => props.name, set: (v) => emit("update:name", v) })
const avatarModel = computed({ get: () => props.avatar, set: (v) => emit("update:avatar", v) })
const identityModel = computed({ get: () => props.identity, set: (v) => emit("update:identity", v) })
const personalityModel = computed({ get: () => props.personality, set: (v) => emit("update:personality", v) })
const speakingStyleModel = computed({ get: () => props.speakingStyle, set: (v) => emit("update:speakingStyle", v) })
const relationshipStyleModel = computed({ get: () => props.relationshipStyle, set: (v) => emit("update:relationshipStyle", v) })
const systemPromptModel = computed({ get: () => props.systemPrompt, set: (v) => emit("update:systemPrompt", v) })
const boundaryRulesModel = computed({ get: () => props.boundaryRules, set: (v) => emit("update:boundaryRules", v) })
const personalityConfigModel = computed({ get: () => props.personalityConfig, set: (v) => emit("update:personalityConfig", v) })
const isActiveModel = computed({ get: () => props.isActive, set: (v) => emit("update:isActive", v) })
</script>


