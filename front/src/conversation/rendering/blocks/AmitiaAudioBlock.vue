<template>
  <div class="amrp-audio">
    <div class="amrp-media-head">
      <span>{{ block.title || "语音" }}</span>
      <span>{{ formatDuration(block.duration) }}</span>
    </div>
    <div v-if="status === 'failed'" class="amrp-media-error">
      <span>{{ errorMessage || "音频加载失败" }}</span>
      <button type="button" @click="retry">重试</button>
    </div>
    <div v-else class="amrp-audio-line">
      <audio
        ref="audioEl"
        :src="block.url"
        controls
        preload="metadata"
        @loadedmetadata="status = 'ready'"
        @error="onError"
      ></audio>
      <button type="button" @click="cycleSpeed">{{ speed }}×</button>
      <a :href="block.url" download>下载</a>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import type { AudioBlock } from "../types";
import { formatDuration } from "../utils";

const props = defineProps<{
  block: AudioBlock;
}>();

const audioEl = ref<HTMLAudioElement>();
const errorMessage = ref("");
const status = ref(props.block.status ?? "loading");
const speeds = [1, 1.25, 1.5, 2];
const speed = ref(1);

watch(
  () => props.block.status,
  (value) => {
    if (value) status.value = value;
  },
);

function cycleSpeed() {
  const index = speeds.indexOf(speed.value);
  speed.value = speeds[(index + 1) % speeds.length];
  if (audioEl.value) audioEl.value.playbackRate = speed.value;
}

function onError() {
  status.value = "failed";
  errorMessage.value = "音频无法播放";
}

function retry() {
  status.value = "loading";
  errorMessage.value = "";
  if (audioEl.value) audioEl.value.load();
}
</script>

<style scoped>
.amrp-audio {
  max-width: 460px;
  margin: 10px 0;
  border: 1px solid var(--amrp-line);
  border-radius: 10px;
  padding: 9px 11px;
  background: var(--amrp-surface);
}

.amrp-media-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 7px;
  color: var(--amrp-muted);
  font-size: 10.5px;
}

.amrp-audio-line {
  display: flex;
  align-items: center;
  gap: 7px;
}

audio {
  min-width: 0;
  flex: 1;
  height: 36px;
}

button,
a {
  border: 0;
  border-radius: 6px;
  padding: 5px 7px;
  background: var(--amrp-soft);
  color: var(--amrp-text);
  font: inherit;
  font-size: 10px;
  text-decoration: none;
  cursor: pointer;
}

.amrp-media-error {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: var(--amrp-danger);
  font-size: 11px;
}
</style>
