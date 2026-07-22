<template>
  <div class="core-zone" ref="coreZoneRef">
    <div class="pulse" ref="pulseRef"></div>
    <div class="core-symbol" aria-hidden="true">
      <span class="sigil-loop loop-a"></span>
      <span class="sigil-loop loop-b"></span>
      <span class="sigil-loop loop-c"></span>
      <span class="sigil-center"></span>
    </div>
    <div class="core-caption">{{ caption }}</div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue"

const props = defineProps<{
  caption?: string
}>()

const coreZoneRef = ref<HTMLElement | null>(null)
const pulseRef = ref<HTMLElement | null>(null)

function ping() {
  const p = pulseRef.value
  if (!p) return
  p.classList.remove("active")
  void (p as HTMLElement).offsetWidth
  p.classList.add("active")
}

function getEl() {
  return coreZoneRef.value
}

defineExpose({ ping, getEl })
</script>
