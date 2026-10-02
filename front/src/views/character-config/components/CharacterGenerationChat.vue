<template>
  <section class="generation-chat">
    <div ref="messageList" class="generation-messages" role="log" aria-live="polite">
      <div class="generation-message assistant"><span class="message-author">角色设计助手</span><div class="message-bubble">你想创建怎样的角色？告诉我角色的身份、性格或故事，我们可以一起完善。也可以直接点击下一步，从空白角色卡开始编辑。</div></div>
      <div v-for="(message, index) in messages" :key="index" :class="['generation-message', message.role]"><span class="message-author">{{ message.role === 'user' ? '你' : '角色设计助手' }}</span><div class="message-bubble">{{ message.content }}</div></div>
      <div v-if="pendingMessage" class="generation-message user"><span class="message-author">你</span><div class="message-bubble">{{ pendingMessage }}</div></div>
      <div v-if="busy" class="generation-message assistant" role="status"><div class="message-bubble">正在整理角色草稿…</div></div>
      <el-alert v-if="error" :title="error" type="error" :closable="false" />
      <p v-if="proposal">角色草稿已更新，下一步可手动调整。</p>
    </div>
    <div class="generation-composer">
      <el-input v-model="input" type="textarea" :autosize="{ minRows: 1, maxRows: 4 }" :maxlength="4000" :disabled="busy" placeholder="描述你想创建的角色…" aria-label="角色需求" />
      <div class="generation-actions">
        <el-button type="primary" :loading="busy" :disabled="busy || !input.trim()" @click="send">发送</el-button>
        <el-button @click="apply">下一步：编辑角色卡</el-button>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref } from "vue";
import { useApi } from "@/composables/useApi";
const props = defineProps<{ draft: Record<string, any> }>();
const emit = defineEmits<{ apply: [draft: Record<string, any>] }>();
const { post } = useApi();
const input = ref("");
const messages = ref<{ role: string; content: string }[]>([]);
const busy = ref(false);
const error = ref("");
const proposal = ref<Record<string, any> | null>(null);
const pendingMessage = ref('');
const messageList = ref<HTMLElement | null>(null);
let active = true;
onBeforeUnmount(() => { active = false; });
function apply() {
  emit('apply', proposal.value ?? props.draft);
  proposal.value = null;
}
async function send() {
  if (busy.value || !input.value.trim()) return;
  const content = input.value.trim();
  busy.value = true;
  error.value = "";
  pendingMessage.value = content;
  await scrollToLatest();
  try {
    const history = [...messages.value.slice(-30), { role: "user", content }];
    const result = await post<{ reply: string; draft: Record<string, any> }>("/api/characters/generate-card", {
      messages: history,
      draft: { ...props.draft, ...proposal.value, personalityConfig: { ...props.draft.personalityConfig, ...proposal.value?.personalityConfig } },
    });
    if (!active) return;
    if (!result?.reply || !result.draft) throw new Error("empty draft");
    proposal.value = { ...proposal.value, ...result.draft, personalityConfig: { ...proposal.value?.personalityConfig, ...result.draft.personalityConfig } };
    messages.value = [...history, { role: "assistant", content: result.reply }];
    input.value = "";
    pendingMessage.value = '';
  } catch {
    if (active) error.value = "生成失败，请检查默认文本模型后重试；现有编辑内容未修改。";
  } finally {
    if (active) busy.value = false;
    if (active) await scrollToLatest();
  }
}
async function scrollToLatest() {
  await nextTick();
  if (active && messageList.value) messageList.value.scrollTop = messageList.value.scrollHeight;
}
</script>

<style scoped>
.generation-chat { display: flex; flex-direction: column; height: clamp(360px, 65dvh, 780px); min-height: 0; }
.generation-chat p { color: var(--ac-color-text-secondary); line-height: 1.6; margin: 0; }
.generation-messages { display: flex; flex-direction: column; gap: 16px; flex: 1; min-height: 0; overflow-y: auto; padding: 8px 4px 16px; }
.generation-message { align-self: flex-start; max-width: 84%; white-space: pre-wrap; overflow-wrap: anywhere; flex-shrink: 0; }
.message-author { display: block; margin-bottom: 6px; font-size: 12px; color: var(--ac-color-text-secondary); }
.message-bubble { padding: 12px 14px; border-radius: 18px; line-height: 1.6; background: var(--ac-color-surface-alt); }
.generation-message.user { align-self: flex-end; }
.generation-message.user .message-author { text-align: right; }
.generation-message.user .message-bubble { background: var(--ac-color-primary-bg); }
.generation-composer { display: grid; gap: 12px; padding-top: 12px; }
.generation-actions { display: flex; gap: 10px; flex-wrap: wrap; }
</style>
