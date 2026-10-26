<template>
  <div class="channel-message" :class="message.role">
    <div class="message-meta">
      <span class="message-sender">{{ message.senderLabel }}</span>
      <span v-if="message.peerKind === 'group'" class="message-chip">群聊</span>
      <span v-for="name in message.mentions" :key="`mention-${name}`" class="message-chip message-chip--mention">@{{ name }}</span>
      <span class="message-time">{{ formatTime(String(message.createdAt || "")) }}</span>
    </div>
    <blockquote v-if="message.replyExcerpt" class="message-quote">
      <span class="message-quote-sender">{{ message.replySender || "引用" }}</span>
      <span class="message-quote-text">{{ message.replyExcerpt }}</span>
    </blockquote>
    <div v-if="mediaTags.length" class="message-tags">
      <span v-for="tag in mediaTags" :key="tag" class="message-chip message-chip--media">{{ tag }}</span>
    </div>
    <div v-if="images.length" class="media-grid">
      <div v-for="item in images" :key="item.url" class="media-block">
        <img
          v-if="!failed.has(item.url)"
          class="message-image"
          :src="item.url"
          :alt="item.name || '渠道图片'"
          loading="lazy"
          @error="markFailed(item.url)"
        />
        <a v-else class="media-fallback" :href="item.url" target="_blank" rel="noreferrer">图片加载失败，打开原图</a>
      </div>
    </div>
    <video
      v-for="item in videos"
      :key="item.url"
      class="message-video"
      :src="item.url"
      controls
      preload="metadata"
    ></video>
    <audio
      v-for="item in audios"
      :key="item.url"
      class="message-audio"
      :src="item.url"
      controls
      preload="metadata"
    ></audio>
    <a
      v-for="item in files"
      :key="item.url"
      class="message-file"
      :href="item.url"
      :download="item.name || undefined"
      target="_blank"
      rel="noreferrer"
    >
      <span class="file-icon">▤</span>
      <span class="file-name">{{ item.name || "附件" }}</span>
      <small>{{ item.mime || "文件" }}</small>
    </a>
    <div v-if="message.text" class="message-content">{{ message.text }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive } from "vue";
import type { ChannelMediaItem, NormalizedChannelMessage } from "./channelMessageMedia";

const props = defineProps<{
  message: NormalizedChannelMessage;
}>();

const failed = reactive(new Set<string>());
const images = computed(() => media("image"));
const videos = computed(() => media("video"));
const audios = computed(() => media("audio"));
const files = computed(() => media("file"));

function media(kind: ChannelMediaItem["kind"]) {
  return props.message.media.filter((item) => item.kind === kind);
}

const mediaTags = computed(() => {
  const tags: string[] = [];
  const counts: Array<[ChannelMediaItem["kind"], string]> = [["image", "图片"], ["video", "视频"], ["audio", "语音"]];
  for (const [kind, label] of counts) {
    const total = media(kind).length;
    if (total > 0) tags.push(total > 1 ? `${label} ×${total}` : label);
  }
  for (const item of media("file")) tags.push(item.name ? `文件 ${item.name}` : "文件");
  return tags;
});

function markFailed(url: string) {
  failed.add(url);
}

function formatTime(value: string) {
  const date = new Date(value);
  return value && !Number.isNaN(date.getTime()) ? date.toLocaleString() : value;
}
</script>

<style scoped>
.channel-message { max-width: min(680px, 82%); padding: 9px 12px; border-radius: 10px; background: var(--ac-color-surface); }
.channel-message.user { align-self: flex-end; background: var(--ac-color-primary-bg); }
.message-meta { display: flex; flex-wrap: wrap; align-items: center; gap: 5px; color: var(--text-muted); font-size: 10px; }
.message-sender { color: var(--text-secondary); font-weight: 600; }
.message-time { margin-left: auto; }
.message-chip { padding: 1px 6px; border-radius: 999px; background: var(--ac-color-bg-secondary, rgba(127, 127, 127, 0.12)); color: var(--text-secondary); font-size: 10px; line-height: 1.5; }
.message-chip--mention { background: color-mix(in srgb, var(--ac-color-primary) 16%, transparent); color: var(--ac-color-primary); }
.message-chip--media { background: color-mix(in srgb, var(--ac-color-primary) 10%, transparent); color: var(--text-secondary); }
.message-quote { margin: 7px 0 0; padding: 6px 9px; border-left: 3px solid var(--surface-border); border-radius: 0 6px 6px 0; background: color-mix(in srgb, var(--text-muted) 8%, transparent); font-size: 11px; line-height: 1.5; }
.message-quote-sender { display: block; margin-bottom: 2px; color: var(--text-secondary); font-weight: 600; }
.message-quote-text { color: var(--text-muted); word-break: break-word; }
.message-tags { display: flex; flex-wrap: wrap; gap: 5px; margin-top: 6px; }
.media-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 6px; margin-top: 7px; }
.media-block { min-width: 0; }
.message-image, .message-video { display: block; max-width: 100%; max-height: 420px; border-radius: 8px; }
.message-image { width: 100%; object-fit: contain; background: var(--ac-color-bg-secondary, transparent); }
.message-video { width: 100%; margin-top: 7px; }
.message-audio { width: 100%; margin-top: 7px; }
.message-file { display: flex; align-items: center; gap: 8px; margin-top: 7px; padding: 8px 10px; border: 1px solid var(--surface-border); border-radius: 8px; color: var(--text-primary); text-decoration: none; word-break: break-all; }
.message-file:hover { border-color: var(--ac-color-primary); }
.file-icon { flex-shrink: 0; color: var(--text-muted); }
.file-name { flex: 1; min-width: 0; }
.message-file small { flex-shrink: 0; color: var(--text-muted); font-size: 10px; }
.media-fallback { display: inline-block; margin-top: 6px; color: var(--ac-color-primary); font-size: 12px; }
.message-content { margin-top: 5px; white-space: pre-wrap; word-break: break-word; color: var(--text-primary); font-size: 13px; line-height: 1.55; }
</style>
