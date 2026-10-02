<template>
  <section class="generation-chat">
    <p>描述你想创建的角色，也可以继续对话调整当前角色。生成内容先作为草稿，保存前均可编辑。</p>
    <div class="generation-messages" role="log" aria-live="polite">
      <div v-for="(message, index) in messages" :key="index" :class="['generation-message', message.role]">{{ message.content }}</div>
      <div v-if="busy" role="status">正在整理角色草稿…</div>
    </div>
    <el-input v-model="input" type="textarea" :rows="3" :maxlength="4000" :disabled="busy" placeholder="例如：设计一位喜欢天文、说话简洁的图书管理员" />
    <div class="generation-actions">
      <el-button type="primary" :loading="busy" :disabled="!input.trim()" @click="send">发送</el-button>
      <el-button :disabled="busy || !proposal" @click="apply">同步到编辑角色</el-button>
    </div>
    <el-alert v-if="error" :title="error" type="error" :closable="false" />
    <p v-if="proposal">草稿已更新，可继续补充需求，或同步后进入编辑角色调整。</p>
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref } from "vue";
import { useApi } from "@/composables/useApi";
const props = defineProps<{ draft: Record<string, any> }>();
const emit = defineEmits<{ apply: [draft: Record<string, any>] }>();
const { post } = useApi();
const input = ref("");
const messages = ref<{ role: string; content: string }[]>([]);
const busy = ref(false);
const error = ref("");
const proposal = ref<Record<string, any> | null>(null);
let active = true;
onBeforeUnmount(() => { active = false; });
function apply() {
  if (!proposal.value || busy.value) return;
  emit('apply', proposal.value);
  proposal.value = null;
}
async function send() {
  if (busy.value || !input.value.trim()) return;
  const content = input.value.trim();
  busy.value = true;
  error.value = "";
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
  } catch {
    if (active) error.value = "生成失败，请检查默认文本模型后重试；现有编辑内容未修改。";
  } finally {
    if (active) busy.value = false;
  }
}
</script>

<style scoped>
.generation-chat { display: grid; gap: 14px; }
.generation-chat p { color: var(--ac-color-text-secondary); line-height: 1.6; margin: 0; }
.generation-messages { display: flex; flex-direction: column; gap: 12px; max-height: 48vh; overflow-y: auto; }
.generation-message { padding: 12px 14px; border-radius: 14px; background: var(--ac-color-surface-alt); white-space: pre-wrap; overflow-wrap: anywhere; }
.generation-message.user { align-self: flex-end; background: var(--ac-color-primary-bg); }
.generation-actions { display: flex; gap: 10px; flex-wrap: wrap; }
</style>
