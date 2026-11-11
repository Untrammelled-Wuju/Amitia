<template>
  <BackgroundMedia v-if="active" class="application-background" :settings="settings" :source="source" :animate="!state.reduceAnimation && state.dynamicEffect && !reducedMotion" />
</template>
<script setup lang="ts">
import { computed, h, onMounted, onUnmounted, ref, watch } from "vue";
import { ElNotification } from "element-plus";
import BackgroundMedia from "./BackgroundMedia.vue";
import { useBackgroundPreference } from "../composables/useBackgroundPreference";
import { useTheme } from "../composables/useTheme";
import { originalErrorText } from "../runtime/original-error";
const props = defineProps<{ visible: boolean }>();
const { settings, source, init } = useBackgroundPreference();
const { state } = useTheme();
const mediaQuery = window.matchMedia("(prefers-reduced-motion: reduce)");
const reducedMotion = ref(mediaQuery.matches);
const active = computed(() => props.visible && settings.value.enabled && (!!source.value || settings.value.mode === "color" || settings.value.mode === "gradient"));
function motionChanged() { reducedMotion.value = mediaQuery.matches; }
watch(active, value => { document.documentElement.dataset.customBackground = String(value); }, { immediate: true });
onMounted(() => {
  void init().catch((error) => ElNotification({
    title: "读取背景设置失败",
    type: "error",
    duration: 0,
    message: h("pre", { style: "white-space: pre-wrap; overflow-wrap: anywhere; max-height: 240px; overflow: auto; user-select: text" }, originalErrorText(error)),
  }));
  mediaQuery.addEventListener("change", motionChanged);
});
onUnmounted(() => { mediaQuery.removeEventListener("change", motionChanged); delete document.documentElement.dataset.customBackground; });
</script>
<style>
.application-background { position: absolute !important; inset: 0; z-index: 0; pointer-events: none; }
html[data-custom-background="true"] .app-shell,
html[data-custom-background="true"] .app-body.is-desktop-shell,
html[data-custom-background="true"] .app-main,
html[data-custom-background="true"] .app-content,
html[data-custom-background="true"] .chat-surface { background: transparent !important; }
</style>
