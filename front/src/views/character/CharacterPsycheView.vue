<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div>
    <div v-if="psycheLoading" style="text-align:center;padding:16px"><el-icon class="is-loading" size="20"><Loading /></el-icon></div>
    <div v-else-if="psycheError" class="form-tip" style="color:var(--el-color-danger)">{{ psycheError }}</div>
    <div v-else>
      <el-descriptions :column="3" border size="small">
        <el-descriptions-item label="情绪正">{{ psycheState.emotion.positive.toFixed(3) }}</el-descriptions-item>
        <el-descriptions-item label="情绪负">{{ psycheState.emotion.negative.toFixed(3) }}</el-descriptions-item>
        <el-descriptions-item label="唤醒度">{{ psycheState.emotion.arousal.toFixed(3) }}</el-descriptions-item>
        <el-descriptions-item label="支配度">{{ psycheState.emotion.dominance.toFixed(3) }}</el-descriptions-item>
        <el-descriptions-item label="心境效价">{{ psycheState.mood.valence.toFixed(3) }}</el-descriptions-item>
        <el-descriptions-item label="心境张力">{{ psycheState.mood.tension.toFixed(3) }}</el-descriptions-item>
        <el-descriptions-item label="压力">{{ psycheState.stress.toFixed(3) }}</el-descriptions-item>
        <el-descriptions-item label="PAD标签">{{ psycheState.mood.pad || '—' }}</el-descriptions-item>
        <el-descriptions-item label="情绪标签">{{ psycheState.affectLabel || '—' }}</el-descriptions-item>
      </el-descriptions>
      <div v-if="psycheState.needs && Object.keys(psycheState.needs).length" style="margin-top:8px">
        <div class="form-tip" style="font-weight:600;margin-bottom:4px">需求水平：</div>
        <div style="display:flex;gap:6px;flex-wrap:wrap">
          <el-tag v-for="(v,k) in psycheState.needs" :key="k" size="small">{{ k }}: {{ v.toFixed(2) }}</el-tag>
        </div>
      </div>
      <div v-if="psycheState.relationship" style="margin-top:8px">
        <div class="form-tip" style="font-weight:600;margin-bottom:4px">关系状态：</div>
        <div style="display:flex;gap:6px;flex-wrap:wrap">
          <el-tag size="small" type="primary">信任 {{ psycheState.relationship.trust.toFixed(2) }}</el-tag>
          <el-tag size="small" type="success">熟悉 {{ psycheState.relationship.familiarity.toFixed(2) }}</el-tag>
          <el-tag size="small" type="warning">张力 {{ psycheState.relationship.tension.toFixed(2) }}</el-tag>
          <el-tag size="small" type="info">安全感 {{ psycheState.relationship.security.toFixed(2) }}</el-tag>
        </div>
      </div>
      <div v-if="psycheState.beliefs?.length" style="margin-top:8px">
        <div class="form-tip" style="font-weight:600;margin-bottom:4px">活跃信念：</div>
        <div style="display:flex;gap:4px;flex-wrap:wrap">
          <el-tag v-for="b in psycheState.beliefs.slice(0,5)" :key="b.key" :type="b.conflicted?'danger':'success'" size="small">{{ b.key }}={{ b.value }}[{{ (b.confidence*100).toFixed(0) }}%]</el-tag>
        </div>
        <div v-if="psycheState.beliefs.length > 5" class="form-tip">+{{ psycheState.beliefs.length-5 }} 更多</div>
      </div>
      <div class="form-tip" style="margin-top:8px">采样时间: {{ psycheState.collectedAt || '—' }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, inject, watch, onMounted, type Ref } from "vue"
import { Loading } from '@element-plus/icons-vue'
import { apiClient } from "@/composables/useApi"
import type { PsycheStateSnapshot } from "../../types"

const currentCharacterId = inject<Ref<string | null>>('currentCharacterId')

const psycheLoading = ref(false)
const psycheError = ref("")
const psycheState = ref<PsycheStateSnapshot>({
  emotion: { positive: 0, negative: 0, arousal: 0, dominance: 0 },
  mood: { valence: 0, tension: 0, pad: "" },
  stress: 0,
  needs: {},
  beliefs: [],
  relationship: { trust: 0.5, familiarity: 0.5, tension: 0, security: 0.5 },
  affectLabel: "",
  collectedAt: "",
})

async function loadPsycheState() {
  if (!currentCharacterId?.value) return
  psycheLoading.value = true
  psycheError.value = ""
  try {
    const { data } = await apiClient.get("/api/psyche/snapshot", {
      params: { characterId: currentCharacterId.value }
    })
    if ((data as any)?.data) {
      psycheState.value = (data as any).data
    } else if (data) {
      psycheState.value = data as PsycheStateSnapshot
    }
  } catch (e: any) {
    psycheError.value = "心理状态不可用: " + (e?.message || "网络错误")
  } finally {
    psycheLoading.value = false
  }
}

onMounted(() => {
  loadPsycheState()
})

watch(currentCharacterId as Ref<string | null>, () => {
  loadPsycheState()
})
</script>

<style scoped>
.form-tip { font-size: 12px; color: var(--el-text-color-secondary); margin-top: 4px; }
</style>
