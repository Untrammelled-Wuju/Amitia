<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="step-panel">
    <h2>创建 AI 陪伴角色</h2>
    <p class="step-desc">给你的 AI 陪伴角色一个名字和性格，让它更有人情味。</p>
    <el-form label-position="top" size="default">
      <el-form-item label="角色头像">
        <div style="display:flex;align-items:center;gap:10px">
          <div @click="triggerUpload" style="width:64px;height:64px;border-radius:50%;border:2px dashed var(--ac-color-border);display:flex;align-items:center;justify-content:center;cursor:pointer;overflow:hidden;flex-shrink:0;background:var(--ac-color-surface)">
            <img v-if="charAvatarModel" :src="charAvatarModel" style="width:100%;height:100%;object-fit:cover" />
            <span v-else style="font-size:24px;color:var(--ac-color-text-muted)">+</span>
          </div>
          <input ref="avatarInput" type="file" accept="image/*" hidden @change="onAvatarFile" />
          <el-input v-model="charAvatarModel" placeholder="或输入头像URL" size="small" style="flex:1" />
        </div>
      </el-form-item>
      <el-form-item label="角色名称">
        <el-input v-model="charNameModel" placeholder="例如：阿米提亚" />
      </el-form-item>
      <el-form-item label="角色身份">
        <el-input v-model="charIdentityModel" placeholder="AI 虚拟角色" />
      </el-form-item>
      <el-form-item label="性格描述">
        <el-input v-model="charPersonalityModel" type="textarea" :rows="2" placeholder="温和、体贴、有耐心" />
      </el-form-item>
      <el-form-item label="角色提示词">
        <el-input v-model="charPromptModel" type="textarea" :rows="4" placeholder="设定角色的行为规则、语气、回复风格等..." />
      </el-form-item>
    </el-form>
  </div>
</template>
<script setup lang="ts">
import { computed, ref } from "vue"
const props = defineProps<{ charName: string; charAvatar: string; charIdentity: string; charPersonality: string; charPrompt: string }>()
const emit = defineEmits<{
  (e: "update:charAvatar", v: string): void
  (e: "update:charName", v: string): void
  (e: "update:charIdentity", v: string): void
  (e: "update:charPersonality", v: string): void
  (e: "update:charPrompt", v: string): void
}>()
const charNameModel = computed({ get: () => props.charName, set: (v) => emit("update:charName", v) })
const charAvatarModel = computed({ get: () => props.charAvatar, set: (v) => emit("update:charAvatar", v) })
const charIdentityModel = computed({ get: () => props.charIdentity, set: (v) => emit("update:charIdentity", v) })
const charPersonalityModel = computed({ get: () => props.charPersonality, set: (v) => emit("update:charPersonality", v) })
const charPromptModel = computed({ get: () => props.charPrompt, set: (v) => emit("update:charPrompt", v) })

const avatarInput = ref<HTMLInputElement>()
function triggerUpload() { avatarInput.value?.click() }
function onAvatarFile(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file || !file.type.startsWith("image/")) return
  const reader = new FileReader()
  reader.onload = () => { charAvatarModel.value = reader.result as string }
  reader.readAsDataURL(file)
  input.value = ""
}
</script>
