<template>
  <div class="background-media" :style="{ background: 'var(--tp-page)' }">
    <div v-if="source && !failed" class="media-layer" :style="{ opacity: settings.opacity, filter: settings.blurEnabled ? `blur(${settings.blurRadius}px)` : undefined, inset: settings.blurEnabled ? `-${settings.blurRadius * 2}px` : '0' }">
      <video v-if="settings.kind === 'video'" :key="source" ref="video" :src="source" muted loop playsinline preload="auto" @loadeddata="syncPlayback" @error="fail" />
      <img v-else :key="source" :src="source" alt="" @error="fail" />
    </div>
    <span v-if="failed && preview" role="alert">背景无法显示，请更换文件或格式</span>
    <slot />
  </div>
</template>
<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch, nextTick } from "vue";
import type { BackgroundSettings } from "../composables/backgroundStorage";
const props = withDefaults(defineProps<{ settings: BackgroundSettings; source: string; animate?: boolean; preview?: boolean }>(), { animate: true, preview: false });
const emit = defineEmits<{ error: [] }>();
const video = ref<HTMLVideoElement>();
const failed = ref(false);
function fail() { failed.value = true; emit("error"); }
function syncPlayback() {
  if (!video.value) return;
  video.value.muted = true;
  if (!props.animate || props.settings.opacity === 0 || document.hidden) video.value.pause();
  else void video.value.play().catch(() => { video.value?.pause(); });
}
watch(() => props.source, async () => { failed.value = false; await nextTick(); syncPlayback(); });
watch(() => props.animate, syncPlayback);
watch(() => props.settings.opacity, syncPlayback);
onMounted(() => { document.addEventListener("visibilitychange", syncPlayback); syncPlayback(); });
onUnmounted(() => { document.removeEventListener("visibilitychange", syncPlayback); video.value?.pause(); });
</script>
<style scoped>
.background-media { position: relative; overflow: hidden; isolation: isolate; }
.media-layer { position: absolute; pointer-events: none; z-index: -1; }
video, img { width: 100%; height: 100%; object-fit: cover; }
.background-media > span { display: block; padding: 16px; color: var(--ac-color-text); }
</style>
