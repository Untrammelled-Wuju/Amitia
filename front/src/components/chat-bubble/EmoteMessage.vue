<template>
  <figure class="emote-message" :class="`is-${message.status || 'sent'}`">
    <img
      v-if="!failed"
      :src="assetUrl"
      :alt="displayText"
      :width="message.width || undefined"
      :height="message.height || undefined"
      loading="lazy"
      decoding="async"
      @error="handleError"
    />
    <figcaption v-else>{{ displayText }}</figcaption>
    <span v-if="message.status === 'sending'" class="emote-state">发送中</span>
    <span v-else-if="message.status === 'failed'" class="emote-state is-failed">发送失败</span>
  </figure>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue"
import { useAssetUrl } from "@/composables/useAssetUrl"

const props = defineProps<{ message: any }>()
const failed = ref(false)
const { assetUrl: resolveAssetUrl } = useAssetUrl()
const originalReference = computed(() => props.message.originalAssetReference || props.message.original_asset_reference || props.message.imageUrl || props.message.image_url || "")
const fallbackReference = computed(() => props.message.fallbackAssetReference || props.message.fallback_asset_reference || "")
const usingFallback = ref(!originalReference.value && !!fallbackReference.value)
const assetUrl = computed(() => resolveAssetUrl(usingFallback.value ? fallbackReference.value : originalReference.value))
const displayText = computed(() => props.message.altText || props.message.alt_text || props.message.content || "表情加载失败")

watch([originalReference, fallbackReference], () => {
  failed.value = false
  usingFallback.value = !originalReference.value && !!fallbackReference.value
})

function handleError() {
  if (!usingFallback.value && fallbackReference.value && fallbackReference.value !== originalReference.value) {
    usingFallback.value = true
    return
  }
  failed.value = true
}
</script>

<style scoped>
.emote-message { position: relative; display: inline-flex; flex-direction: column; align-items: flex-start; margin: 2px 0; max-width: min(240px, 58vw); }
.emote-message img { display: block; width: auto; height: auto; max-width: 100%; max-height: 220px; object-fit: contain; border-radius: 10px; }
.emote-message figcaption { padding: 8px 10px; border: 1px solid var(--ac-color-border-light); border-radius: var(--ac-radius-md); color: var(--ac-color-text-muted); font-size: 12px; }
.emote-state { margin-top: 4px; color: var(--ac-color-text-muted); font-size: 11px; }
.emote-state.is-failed { color: var(--el-color-danger); }
@media (prefers-reduced-motion: reduce) { .emote-message img { animation: none; } }
</style>
