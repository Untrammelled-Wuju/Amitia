<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="section-card">
    <div class="section-title">
      作息倾向
      <el-button size="small" @click="saveLifestyle" :loading="lifestyleSaving" style="margin-left:12px">保存</el-button>
      <el-button size="small" @click="resetLifestyle" :loading="lifestyleResetting" type="warning" plain>恢复默认</el-button>
    </div>
    <template v-for="group in lifestyleGroups" :key="group.name">
      <div class="slider-hint">{{ group.name }}</div>
      <div class="slider-grid">
        <SliderRow v-for="item in group.sliders" :key="item.key" v-model="lifestyleForm[item.key]" :label="item.label" :left="item.left" :right="item.right" :min="0" :max="100" />
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from "vue"
import { ElMessage } from "element-plus"
import SliderRow from "../../../components/SliderRow.vue"
import { useLifestyleTendency, LIFESTYLE_SLIDERS, LIFESTYLE_GROUPS } from "../../../composables/useLifestyleTendency"

const props = defineProps<{
  characterId: string
}>()

const { getLifestyleTendency, updateLifestyleTendency, resetLifestyleTendency } = useLifestyleTendency()

const lifestyleForm = reactive({
  punctualityTendency: 50,
  earlyPrepareTendency: 50,
  selfDisciplineTendency: 50,
  sleepinessTendency: 50,
  randomnessTendency: 50,
  activityEnergy: 50,
  socialEnergy: 50,
  careTendency: 50,
  dailyShareTendency: 50,
})
const lifestyleSaving = ref(false)
const lifestyleResetting = ref(false)
const lifestyleConfigured = ref(false)

const lifestyleGroups = computed(() => LIFESTYLE_GROUPS.map(g => ({
  name: g.name,
  sliders: LIFESTYLE_SLIDERS.filter(s => g.keys.includes(s.key)),
})))

onMounted(async () => {
  try {
    const lt = await getLifestyleTendency(props.characterId || undefined)
    if (lt) {
      lifestyleForm.punctualityTendency = lt.punctualityTendency ?? 50
      lifestyleForm.earlyPrepareTendency = lt.earlyPrepareTendency ?? 50
      lifestyleForm.selfDisciplineTendency = lt.selfDisciplineTendency ?? 50
      lifestyleForm.sleepinessTendency = lt.sleepinessTendency ?? 50
      lifestyleForm.randomnessTendency = lt.randomnessTendency ?? 50
      lifestyleForm.activityEnergy = lt.activityEnergy ?? 50
      lifestyleForm.socialEnergy = lt.socialEnergy ?? 50
      lifestyleForm.careTendency = lt.careTendency ?? 50
      lifestyleForm.dailyShareTendency = lt.dailyShareTendency ?? 50
      lifestyleConfigured.value = (lt as any).manuallyConfigured || false
    }
  } catch { }
})

async function saveLifestyle() {
  lifestyleSaving.value = true
  try {
    await updateLifestyleTendency({
      punctualityTendency: lifestyleForm.punctualityTendency,
      earlyPrepareTendency: lifestyleForm.earlyPrepareTendency,
      selfDisciplineTendency: lifestyleForm.selfDisciplineTendency,
      sleepinessTendency: lifestyleForm.sleepinessTendency,
      randomnessTendency: lifestyleForm.randomnessTendency,
      activityEnergy: lifestyleForm.activityEnergy,
      socialEnergy: lifestyleForm.socialEnergy,
      careTendency: lifestyleForm.careTendency,
      dailyShareTendency: lifestyleForm.dailyShareTendency,
    }, props.characterId || undefined)
    lifestyleConfigured.value = true
    ElMessage.success("作息倾向已保存")
  } catch {
    ElMessage.error("保存失败")
  } finally {
    lifestyleSaving.value = false
  }
}

async function resetLifestyle() {
  lifestyleResetting.value = true
  try {
    const data = await resetLifestyleTendency(props.characterId || undefined)
    lifestyleForm.punctualityTendency = data.punctualityTendency ?? 50
    lifestyleForm.earlyPrepareTendency = data.earlyPrepareTendency ?? 50
    lifestyleForm.selfDisciplineTendency = data.selfDisciplineTendency ?? 50
    lifestyleForm.sleepinessTendency = data.sleepinessTendency ?? 50
    lifestyleForm.randomnessTendency = data.randomnessTendency ?? 50
    lifestyleForm.activityEnergy = data.activityEnergy ?? 50
    lifestyleForm.socialEnergy = data.socialEnergy ?? 50
    lifestyleForm.careTendency = data.careTendency ?? 50
    lifestyleForm.dailyShareTendency = data.dailyShareTendency ?? 50
    lifestyleConfigured.value = false
    ElMessage.success("已恢复默认作息倾向")
  } catch {
    ElMessage.error("恢复失败")
  } finally {
    lifestyleResetting.value = false
  }
}
</script>
