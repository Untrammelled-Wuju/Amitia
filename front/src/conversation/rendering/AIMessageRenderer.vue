<template>
  <div class="amrp-root" :class="{ 'amrp-dark': resolvedMode === 'dark' }">
    <div v-if="message.role === 'system'" class="amrp-system-notice">
      <slot name="badges" :message="message" />
      {{ message.markdown }}
    </div>

    <div v-else-if="message.role === 'user'" class="amrp-user-row">
      <div class="amrp-user-stack">
        <div v-if="hasReply" class="amrp-quote" @click="emit('scroll-to-message', replyId)">
          <div class="amrp-quote-bar"></div>
          <div>
            <div class="amrp-quote-name">{{ replySender }}</div>
            <div class="amrp-quote-text">{{ replyText }}</div>
          </div>
        </div>
        <div class="amrp-user-bubble">{{ message.markdown }}</div>
        <div class="amrp-user-meta">
          <slot name="badges" :message="message" />
          <span>{{ formatTime(message.createdAt) }}</span>
        </div>
        <slot name="extension-content" :message="message" />
        <slot name="actions" :message="message" />
      </div>
      <div v-if="showAvatar" class="amrp-avatar user">
        <img v-if="userAvatar" :src="userAvatar" alt="" />
        <span v-else>{{ userInitial }}</span>
      </div>
    </div>

    <article v-else class="amrp-message">
      <div v-if="showAvatar" class="amrp-avatar">
        <img v-if="character.avatar" :src="character.avatar" alt="" />
        <span v-else>{{ characterInitial }}</span>
      </div>
      <div v-else class="amrp-avatar-spacer"></div>
      <div class="amrp-message-body">
        <header class="amrp-head">
          <span class="amrp-name">{{ character.name || "Amitia" }}</span>
          <span class="amrp-role">{{ roleLabel }}</span>
          <span class="amrp-time">{{ formatTime(message.createdAt) }}</span>
          <slot name="badges" :message="message" />
        </header>

        <AmitiaThinkingBlock
          v-if="message.thinking"
          :content="message.thinking.content"
          :state="message.thinking.state"
          :duration="message.thinking.duration"
        />

        <div v-if="stateNotice" class="amrp-message-state" :class="message.state">
          <b>{{ stateNotice.title }}</b>
          <span>{{ stateNotice.detail }}</span>
        </div>

        <RendererErrorBoundary label="Markdown Renderer">
          <MarkdownContent
            v-if="message.markdown"
            :source="message.markdown"
            :streaming="message.state === 'streaming'"
            @citation="activeCitationId = $event"
          />
        </RendererErrorBoundary>

        <template v-for="item in renderedBlocks" :key="item.key">
          <RendererErrorBoundary :label="blockLabel(item)">
            <AmitiaImageBlock v-if="item.kind === 'images'" :images="item.images" />
            <RichBlockRenderer v-else :block="item.block" />
          </RendererErrorBoundary>
        </template>

        <slot name="extension-content" :message="message" />

        <AmitiaCitationList
          :sources="message.sources"
          :highlight-id="activeCitationId"
          @highlight-consumed="activeCitationId = ''"
        />

        <footer v-if="!readOnly" class="amrp-actions">
          <button type="button" @click="copyPlainText">复制</button>
          <button type="button" @click="copyMarkdown">复制 Markdown</button>
          <button v-if="!streaming" type="button" @click="emit('reply', message)">回复</button>
          <button v-if="!streaming" type="button" @click="emit('retry', message)">重新生成</button>
          <slot name="actions" :message="message" />
        </footer>
      </div>
    </article>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { ElMessage } from "element-plus";
import { useTheme } from "@/composables/useTheme";
import type { AIMessageData, RichBlock } from "./types";
import { aimMessagePlainText, normalizeAIMessage } from "./amrp";
import { copyText } from "./utils";
import MarkdownContent from "./markdown/MarkdownContent.vue";
import AmitiaThinkingBlock from "./blocks/AmitiaThinkingBlock.vue";
import AmitiaImageBlock from "./blocks/AmitiaImageBlock.vue";
import RichBlockRenderer from "./blocks/RichBlockRenderer.vue";
import AmitiaCitationList from "./blocks/AmitiaCitationList.vue";
import RendererErrorBoundary from "./blocks/RendererErrorBoundary.vue";
import MinecraftServerBlock from "./blocks/MinecraftServerBlock.vue";
import { registerExtensionRenderer } from "./rendererRegistry";

registerExtensionRenderer({
  rendererId: "minecraft.server",
  component: MinecraftServerBlock,
  capability: "conversation.renderer",
});

type RenderItem =
  | { key: string; kind: "images"; images: Extract<RichBlock, { kind: "image" }>[] }
  | { key: string; kind: "block"; block: RichBlock };

const props = withDefaults(
  defineProps<{
    message: Record<string, any>;
    charName?: string;
    charAvatar?: string;
    characterId?: string;
    userAvatar?: string;
    userName?: string;
    showAvatar?: boolean;
    readOnly?: boolean;
  }>(),
  {
    charName: "Amitia",
    charAvatar: "",
    characterId: "",
    userAvatar: "",
    userName: "我",
    showAvatar: true,
    readOnly: false,
  },
);

const emit = defineEmits<{
  retry: [message: Record<string, any>];
  reply: [message: Record<string, any>];
  "scroll-to-message": [id: string];
}>();

const { resolvedMode } = useTheme();
const activeCitationId = ref("");
const message = computed<AIMessageData>(() =>
  normalizeAIMessage(props.message, {
    id: props.characterId,
    name: props.charName,
    avatar: props.charAvatar,
  }),
);
const character = computed(() => message.value.character ?? { id: "", name: props.charName });
const characterInitial = computed(() => (character.value.name || "A").trim().slice(0, 1));
const userInitial = computed(() => (props.userName || "我").trim().slice(0, 1));
const streaming = computed(() => message.value.state === "streaming" || message.value.state === "queued");
const roleLabel = computed(() => String(props.message.roleLabel ?? props.message.characterRole ?? "默认角色"));
const hasReply = computed(() => Boolean(props.message.replyToMessageId));
const replyId = computed(() => String(props.message.replyToMessageId ?? ""));
const replySender = computed(() => String(props.message.replyToRole === "user" ? "你" : props.charName));
const replyText = computed(() => String(props.message.replyToExcerpt ?? "").slice(0, 120));

const stateNotice = computed(() => {
  switch (message.value.state) {
    case "interrupted":
      return { title: "已中断", detail: "保留已生成内容 · 可继续生成" };
    case "failed":
      return { title: "生成失败", detail: "网络错误 · 可重试" };
    case "cancelled":
      return { title: "已取消", detail: "用户主动停止生成" };
    case "queued":
      return { title: "等待开始生成", detail: "任务已进入队列" };
    default:
      return null;
  }
});

const renderedBlocks = computed<RenderItem[]>(() => {
  const items: RenderItem[] = [];
  let imageGroup: Extract<RichBlock, { kind: "image" }>[] = [];
  const flushImages = () => {
    if (!imageGroup.length) return;
    items.push({
      key: `images:${imageGroup.map((image) => image.id).join(":")}`,
      kind: "images",
      images: imageGroup,
    });
    imageGroup = [];
  };
  for (const block of message.value.blocks) {
    if (block.kind === "image" && block.url) {
      imageGroup.push(block);
      continue;
    }
    flushImages();
    items.push({ key: `${block.kind}:${block.id}`, kind: "block", block });
  }
  flushImages();
  return items;
});

function formatTime(value: number): string {
  const date = new Date(value);
  return `${String(date.getHours()).padStart(2, "0")}:${String(date.getMinutes()).padStart(2, "0")}`;
}

function blockLabel(item: RenderItem): string {
  if (item.kind === "images") return "Image Renderer";
  return `${item.block.kind} Renderer`;
}

async function copyPlainText() {
  const copied = await copyText(aimMessagePlainText(message.value));
  copied ? ElMessage.success("已复制纯文本") : ElMessage.warning("复制失败");
}

async function copyMarkdown() {
  const copied = await copyText(message.value.markdown);
  copied ? ElMessage.success("已复制 Markdown") : ElMessage.warning("复制失败");
}

</script>

<style scoped>
.amrp-root {
  --amrp-bg: #f7f7f8;
  --amrp-text: #19191c;
  --amrp-muted: #8c8e95;
  --amrp-line: #e8e8eb;
  --amrp-user: #ece9ff;
  --amrp-code-bg: #1b1c20;
  --amrp-code-head: #232429;
  --amrp-soft: #f1f1f3;
  --amrp-control: #e9e9ed;
  --amrp-surface: #ffffff;
  --amrp-inline-code: #ededf0;
  --amrp-tool-text: #5f6168;
  --amrp-accent: #7060e8;
  --amrp-accent-soft: #efedff;
  --amrp-good: #3e8d5d;
  --amrp-danger: #c85353;
  width: 100%;
}

.amrp-root.amrp-dark {
  --amrp-bg: #17181b;
  --amrp-text: #ececef;
  --amrp-muted: #9b9da5;
  --amrp-line: #2a2b30;
  --amrp-user: #302d4f;
  --amrp-code-bg: #111216;
  --amrp-code-head: #1c1d22;
  --amrp-soft: #24252a;
  --amrp-control: #292a30;
  --amrp-surface: #1d1e22;
  --amrp-inline-code: #24252a;
  --amrp-tool-text: #a7a9b0;
  --amrp-accent: #9a8cff;
  --amrp-accent-soft: #2e294d;
  --amrp-good: #6fbd8b;
  --amrp-danger: #e07878;
}

.amrp-message {
  display: grid;
  grid-template-columns: 36px minmax(0, 1fr);
  gap: 11px;
  width: 100%;
  max-width: 820px;
  margin: 0 auto 38px;
}

.amrp-avatar,
.amrp-avatar-spacer {
  width: 34px;
  height: 34px;
}

.amrp-avatar {
  display: grid;
  place-items: center;
  overflow: hidden;
  border-radius: 11px;
  background: linear-gradient(145deg, #9185ed, #6858d4);
  color: white;
  font-size: 12px;
  font-weight: 800;
}

.amrp-avatar.user {
  background: var(--amrp-accent);
}

.amrp-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.amrp-message-body {
  min-width: 0;
  max-width: 820px;
}

.amrp-head {
  min-height: 34px;
  display: flex;
  align-items: center;
  gap: 7px;
  margin-bottom: 3px;
}

.amrp-name {
  color: var(--amrp-text);
  font-weight: 720;
}

.amrp-role,
.amrp-time {
  color: var(--amrp-muted);
  font-size: 11px;
}

.amrp-time {
  color: color-mix(in srgb, var(--amrp-muted) 75%, transparent);
}

.amrp-system-notice {
  max-width: 700px;
  margin: 13px auto;
  border-radius: 8px;
  padding: 7px 10px;
  background: var(--amrp-soft);
  color: var(--amrp-muted);
  font-size: 11px;
  text-align: center;
}

.amrp-user-row {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  width: 100%;
  max-width: 820px;
  margin: 0 auto 34px;
}

.amrp-user-stack {
  max-width: min(620px, 82%);
  display: flex;
  flex-direction: column;
  align-items: flex-end;
}

.amrp-user-bubble {
  border-radius: 18px 6px 18px 18px;
  padding: 11px 15px;
  background: var(--amrp-user);
  color: var(--amrp-text);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.amrp-user-meta {
  display: flex;
  align-items: center;
  gap: 7px;
  margin-top: 4px;
  color: var(--amrp-muted);
  font-size: 10px;
}

.amrp-quote {
  max-width: 320px;
  display: flex;
  margin-bottom: 5px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--amrp-soft) 78%, transparent);
  cursor: pointer;
}

.amrp-quote-bar {
  width: 3px;
  margin: 7px;
  border-radius: 2px;
  background: var(--amrp-accent);
}

.amrp-quote-name {
  margin-top: 6px;
  color: var(--amrp-muted);
  font-size: 10px;
}

.amrp-quote-text {
  max-width: 260px;
  margin: 2px 8px 6px 0;
  overflow: hidden;
  color: var(--amrp-muted);
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.amrp-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
  margin-top: 9px;
  margin-left: -6px;
}

.amrp-actions button {
  border: 0;
  border-radius: 6px;
  padding: 5px 7px;
  background: transparent;
  color: var(--amrp-muted);
  font: inherit;
  font-size: 11px;
  cursor: pointer;
}

.amrp-actions button:hover {
  background: var(--amrp-soft);
  color: var(--amrp-text);
}

.amrp-message-state {
  max-width: 700px;
  display: flex;
  gap: 8px;
  margin: 10px 0;
  border: 1px solid var(--amrp-line);
  border-radius: 9px;
  padding: 8px 10px;
  background: var(--amrp-surface);
  font-size: 11px;
}

.amrp-message-state b {
  color: var(--amrp-text);
}

.amrp-message-state span {
  color: var(--amrp-muted);
}

.amrp-message-state.failed {
  border-color: color-mix(in srgb, var(--amrp-danger) 45%, var(--amrp-line));
}

@media (max-width: 700px) {
  .amrp-message {
    grid-template-columns: 32px minmax(0, 1fr);
    gap: 9px;
    margin-bottom: 32px;
  }

  .amrp-avatar,
  .amrp-avatar-spacer {
    width: 30px;
    height: 30px;
  }

  .amrp-avatar {
    border-radius: 10px;
  }

  .amrp-head {
    min-height: 30px;
  }

  .amrp-user-stack {
    max-width: 85%;
  }
}
</style>
