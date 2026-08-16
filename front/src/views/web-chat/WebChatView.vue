<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="webchat-page">
    <section class="chat-surface">
    <ChatBanners
      :model-missing="modelMissing"
      :is-offline="isOffline"
      :model-error="modelError"
      :import-context="importContext"
      :show-import-detail="showImportDetail"
      :conv-summary="convSummary"
      :show-summary="showSummary"
      @close-error="modelError = ''"
      @close-import="importContext = null"
      @toggle-summary="showSummary = !showSummary"
    />

    <div class="chat-header-region">
    <ChatHeaderBar
      :char-name="charName"
      :char-avatar="charAvatar"
      :char-identity="charIdentity"
      :conv-title="convTitle"
      :can-regenerate="canRegenerate"
      :messages-count="messages.length"
      :conv-id="convId"
      :show-profiles="showProfiles"
      :show-mem-inject="showMemInject"
      :call-active="callActive"
      @toggle-drawer="showDrawer = true"
      @regenerate="handleRegenerate"
      @clear="handleClear"
      @view-memories="handleViewMemories"
      @toggle-char-picker="showCharPicker = true"
      @toggle-profiles="toggleProfiles"
      @toggle-mem-inject="toggleMemInject"
      @toggle-call="handleToggleCall"
    ><template #extension-actions><ChatHeaderExtensionHost :context="chatExtensionContext" /></template></ChatHeaderBar>
    </div>
    <div class="chat-body-wrapper">
      <ProfileSummaryPanel
        :visible="showProfiles"
        @close="showProfiles = false"
      />
      <MemoryInjectPanel
        :visible="showMemInject"
        :conv-id="convId"
        @close="showMemInject = false"
      />
      <MessagesArea
        ref="msgAreaRef"
        :messages="messages"
        :char-name="charName"
        :char-avatar="charAvatar"
        :character-id="characterId"
        :sending="sending"
        :show-scroll-btn="showScrollBtn"
        :is-pulling="isPulling"
        :pull-ready="pullReady"
        :pull-loading="pullLoading"
        :pull-text="pullText"
        :extension-context="chatExtensionContext"
        @scroll="onScroll"
        @wheel="onWheel"
        @touch-start="onMsgTouchStart"
        @touch-move="onMsgTouchMove"
        @touch-end="onMsgTouchEnd"
        @retry="handleRetry"
        @reply="handleSetReply"
        @scroll-to-bottom="scrollToBottom(true)"
      />
      <aside v-if="hasSidebarExtensions" class="chat-sidebar-region"><ChatSidebarExtensionHost :context="chatExtensionContext" /></aside>
    </div>
    <div v-if="callActive || hasStatusExtensions" class="chat-status-region">
      <ChatStatusExtensionHost :context="chatExtensionContext" />
    <RealtimeCallWidget
      v-if="callActive"
      :visible="callActive"
      :api-key="ttsApiKey"
      :voice-type="ttsVoiceType"
      :resource-id="ttsResourceId"
      :conversation-id="convId"
    />
    </div>
    <div class="composer-region"><ChatInput
      ref="inputRef"
      :disabled="modelMissing"
      :sending="sending"
      :generating="generating"
      :is-submitting="isSubmitting"
      :reply-target="replyTarget"
      :character-id="characterId"
      :conversation-id="convId"
      :channel="chatExtensionContext.channel"
      @send="handleSend"
      @image="onImageAttached"
      @removeImage="onImageRemoved"
      @stop="handleStop"
      @voiceAudio="handleVoiceAudio"
      @voiceText="handleVoiceText"
      @video="onVideoAttached"
      @removeVideo="onVideoRemoved"
      @cancel-reply="replyTarget = null"
      @emote="handleEmoteSend"
    /></div>

    <ConversationDrawer
      v-model:visible="showDrawer"
      :characters="characters"
      :import-batches="importBatches"
      :active-char-id="characterId"
      :wechat-msg-count="wechatMsgCount"
      :is-wechat-active="isWechatActive"
      :wechat-online="wechatOnline"
      :qq-msg-count="qqMsgCount"
      :isQQActive="isQQActive"
      :qqOnline="qqOnline"
      @select-char="handleSwitchChar"
      @select-wechat="handleSelectWechat"
      @select-q-q="handleSelectQQ"
      @continue-import="handleContinueImport"
    />

    <CharacterPickerDialog
      v-model:visible="showCharPicker"
      :characters="characters"
      :character-id="characterId"
      @select="handleSwitchChar"
    />

    <MemoryPanel v-model:visible="showMemories" :memories="memories" />
    </section>
  </div>
</template>
<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick, watch, inject } from "vue";
import { useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import { useApi, isLoggedIn } from "../../composables/useApi";
import { useCachedApi } from "../../composables/useCachedApi";
import { useWebChatSSE } from "../../composables/useWebChatSSE";
import { useWebChatScroll } from "../../composables/useWebChatScroll";
import { useWebChatSend } from "../../composables/useWebChatSend";
import { useWebChatConversation } from "../../composables/useWebChatConversation";
import ChatBanners from "../../components/ChatBanners.vue";
import ChatHeaderBar from "../../components/ChatHeaderBar.vue";
import MessagesArea from "../../components/MessagesArea.vue";
import ChatInput from "../../components/ChatInput.vue";
import ConversationDrawer from "../../components/ConversationDrawer.vue";
import CharacterPickerDialog from "../../components/CharacterPickerDialog.vue";
import MemoryPanel from "../../components/MemoryPanel.vue";
import RealtimeCallWidget from "../../components/RealtimeCallWidget.vue";
import ProfileSummaryPanel from "./components/ProfileSummaryPanel.vue";
import MemoryInjectPanel from "./components/MemoryInjectPanel.vue";
import { normalizeRealtimeMessage } from "@/utils/message-order";
import ChatHeaderExtensionHost from "@/components/extension/chat/ChatHeaderExtensionHost.vue";
import ChatStatusExtensionHost from "@/components/extension/chat/ChatStatusExtensionHost.vue";
import ChatSidebarExtensionHost from "@/components/extension/chat/ChatSidebarExtensionHost.vue";
import { useExtensionUIStore } from "@/stores/extensionUI";

const router = useRouter();
const callActive = ref(false);
const ttsApiKey = ref("");
const ttsVoiceType = ref("");
const ttsResourceId = ref("");

async function fetchTtsConfig() {
  try {
    const token = localStorage.getItem("ai-companion-token") || "";
    const res = await fetch("/api/tts/configs", {
      headers: { Authorization: "Bearer " + token },
    });
    const data = await res.json();
    const list = Array.isArray(data?.data)
      ? data.data
      : data?.data?.items || data?.data?.configs || [];
    const active = list.find((c: any) => c.isActive || c.is_active);
    if (active) {
      ttsApiKey.value = active.apiKey || "";
      ttsVoiceType.value = active.voiceType || "";
      ttsResourceId.value = active.resourceId || "";
    }
  } catch {}
}

async function handleToggleCall() {
  await fetchTtsConfig();
  if (!ttsApiKey.value) {
    router.push("/model/voice");
    return;
  }
  callActive.value = !callActive.value;
}
const { get, post } = useApi();
const { cachedGet, invalidateCache } = useCachedApi();
const currentCharName = inject<any>("currentCharName", null);
const extensionUIStore = useExtensionUIStore();

const messages = ref<any[]>([]);
const convId = ref("");
const convTitle = ref("");
const characterId = ref("");
const cachedDef = (() => {
  try {
    const v = localStorage.getItem("uai-default-char");
    return v ? JSON.parse(v) : null;
  } catch {
    return null;
  }
})();
const charName = ref(cachedDef?.name || "");
const charIdentity = ref(cachedDef?.identity || "");
const charAvatar = ref("");

const sending = ref(false);
const modelMissing = ref(false);
const modelError = ref("");
const isOffline = ref(!navigator.onLine);
const showScrollBtn = ref(false);
const showProfiles = ref(false);
const showMemInject = ref(false);

const importContext = ref<any>(null);
const showImportDetail = ref(false);
const convSummary = ref("");
const showSummary = ref(false);
const replyTarget = ref<any>(null);
const chatExtensionContext = computed(() => ({
  characterId: characterId.value,
  conversationId: convId.value,
  channel: isWechatActive.value ? "wechat" : isQQActive.value ? "qq" : "web",
  platform: window.amitiaDesktop ? "desktop" : "web",
  conversationState: sending.value ? "generating" : isOffline.value ? "offline" : "idle",
  capabilities: window.amitiaDesktop ? ["browser", "desktop", "clipboard-host"] : ["browser"],
}));
const hasSidebarExtensions = computed(() => extensionUIStore.getVisibleContributions("chat.sidebar.panel").length > 0);
const hasStatusExtensions = computed(() => extensionUIStore.getVisibleContributions("chat.status.item").length > 0);

const msgAreaRef = ref<InstanceType<typeof MessagesArea>>();
const inputRef = ref<InstanceType<typeof ChatInput>>();

const currentImageBase64 = ref<string | null>(null);
const currentImageFile = ref<File | null>(null);
const pendingImageBase64 = ref<string | null>(null);
const pendingAudioUrl = ref<string | null>(null);
const pendingVideoUrl = ref<string | null>(null);

async function handleEmoteSend(emote: any) {
  if (!convId.value || !characterId.value) {
    ElMessage.warning("请先选择角色和会话");
    return;
  }
  try {
    const message = normalizeRealtimeMessage(
      await post<any>("/api/chat/send-emote", {
        conversationId: convId.value,
        characterId: characterId.value,
        emoteId: emote.id,
        replyToMessageId: replyTarget.value?.id || undefined,
      }),
    );
    if (!messages.value.some((item) => item.id === message.id))
      messages.value.push(message);
    replyTarget.value = null;
    nextTick(() => scrollToBottom());
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.msg || "表情发送失败");
  }
}

function toggleProfiles() {
  showProfiles.value = !showProfiles.value;
  if (showProfiles.value) {
    showMemInject.value = false;
  }
}

function toggleMemInject() {
  showMemInject.value = !showMemInject.value;
  if (showMemInject.value) {
    showProfiles.value = false;
  }
}

function handleSetReply(msg: any) {
  replyTarget.value = {
    id: msg.id,
    role: msg.role,
    content: (msg.content || "").slice(0, 100),
  };
}

const {
  scrollToBottom,
  onScroll,
  onWheel,
  onMsgTouchStart,
  onMsgTouchMove,
  onMsgTouchEnd,
  isPulling,
  pullReady,
  pullLoading,
  pullText,
  hasMoreHistory,
  isLoadingHistory,
  msgPage,
} = useWebChatScroll(msgAreaRef, messages, convId, showScrollBtn);

const {
  connectSSE,
  disconnectSSE,
  connectProactiveSSE,
  disconnectProactiveSSE,
  cleanup: cleanupSSE,
  setLastPolledMsgId,
} = useWebChatSSE(
  convId,
  messages,
  scrollToBottom,
  () => fetchWechatMsgCount(),
  () => fetchQQStatus(),
  sending,
);

const {
  canRegenerate,
  onImageAttached,
  onImageRemoved,
  onVideoAttached,
  onVideoRemoved,
  handleVoiceAudio,
  handleVoiceText,
  handleImageSend,
  handleSend,
  handleStop,
  handleRetry,
  handleRegenerate,
  handleClear,
  getLastPolledMsgId,
  generating,
  isSubmitting,
} = useWebChatSend(
  messages,
  convId,
  characterId,
  sending,
  modelError,
  modelMissing,
  currentImageBase64,
  currentImageFile,
  pendingImageBase64,
  pendingAudioUrl,
  pendingVideoUrl,
  scrollToBottom,
  disconnectSSE,
  inputRef,
  () => fetchWechatMsgCount(),
  () => fetchQQStatus(),
  undefined,
  replyTarget,
);

const {
  characters,
  conversations,
  importBatches,
  memories,
  isWechatActive,
  wechatOnline,
  wechatMsgCount,
  isQQActive,
  qqOnline,
  qqMsgCount,
  showDrawer,
  showCharPicker,
  showMemories,
  selectCharacter,
  handleSwitchChar,
  loadCharacterConversation,
  fetchConversations,
  handleSelectConv,
  handleSelectWechat,
  handleSelectQQ,
  handleContinueImport,
  handleViewMemories,
  fetchWechatMsgCount,
  fetchQQStatus,
  refreshCharacters,
  fetchConvSummary,
} = useWebChatConversation(
  messages,
  convId,
  characterId,
  convTitle,
  charName,
  charIdentity,
  charAvatar,
  hasMoreHistory,
  msgPage,
  scrollToBottom,
  disconnectSSE,
  connectSSE,
  setLastPolledMsgId,
  (name: string) => {
    if (currentCharName) currentCharName.value = name;
  },
);

watch(isOffline, (offline) => {
  if (
    !offline &&
    sending.value &&
    messages.value.some((m) => m.status === "sending")
  ) {
    ElMessage.info("网络已恢复，可重新发送消息");
  }
});

watch(showDrawer, (open) => {
  if (open) refreshCharacters();
});

onMounted(async () => {
  fetchWechatMsgCount();
  fetchQQStatus();
  setInterval(fetchWechatMsgCount, 30000);
  setInterval(() => fetchQQStatus(), 15000);

  connectProactiveSSE();
  history.scrollRestoration = "manual";

  window.addEventListener("online", () => {
    isOffline.value = false;
    ElMessage.success("网络已恢复");
  });
  window.addEventListener("offline", () => {
    isOffline.value = true;
    ElMessage.warning("网络已断开");
  });

  const h = await get<any>("/api/health").catch(() => null);
  if (h?.deployMode === "cloud-web" && !isLoggedIn()) {
    router.push("/login");
    return;
  }
  if (h?.model === "not_configured") {
    modelMissing.value = true;
  }

  const CACHE_VERSION = 2;
  const storedVersion = localStorage.getItem("char_cache_version");
  if (String(storedVersion) !== String(CACHE_VERSION)) {
    invalidateCache("_api_characters");
    localStorage.setItem("char_cache_version", String(CACHE_VERSION));
  }

  const { data: cachedChars, refresh: refreshChars } =
    await cachedGet<any[]>("/api/characters");
  if (cachedChars.value?.length) {
    characters.value = cachedChars.value;
    const lastConv = localStorage.getItem("webchat-last-conv");
    if (lastConv === "wechat") {
      await handleSelectWechat(true);
      return;
    }
    if (lastConv === "qq") {
      await handleSelectQQ(true);
      return;
    }
    const savedId = localStorage.getItem("webchat-char-id");
    const preferred = savedId
      ? characters.value.find((c: any) => c.id === savedId)
      : null;
    if (preferred) {
      selectCharacter(preferred);
    } else {
      const defaultChar = characters.value.find((c: any) => c.isDefault);
      if (defaultChar) selectCharacter(defaultChar);
      else {
        const active = characters.value.find((c: any) => c.isActive);
        if (active) selectCharacter(active);
        else if (characters.value.length > 0)
          selectCharacter(characters.value[0]);
      }
    }
    const def = characters.value.find((c: any) => c.isDefault);
    if (def) {
      localStorage.setItem(
        "uai-default-char",
        JSON.stringify({
          id: def.id,
          name: def.name,
          identity: def.identity || def.personality || "",
          updatedAt: Date.now(),
        }),
      );
    }
  }
  refreshChars().then(() => {
    if (cachedChars.value?.length) {
      characters.value = cachedChars.value;
      const active = characters.value.find((c: any) => c.isActive);
      if (!characterId.value) {
        const defaultChar = characters.value.find((c: any) => c.isDefault);
        if (defaultChar) selectCharacter(defaultChar);
        else if (active) selectCharacter(active);
      }
      if (characterId.value) {
        const cur = characters.value.find(
          (c: any) => c.id === characterId.value,
        );
        if (cur) selectCharacter(cur);
      }
      const def = characters.value.find((c: any) => c.isDefault);
      if (def) {
        localStorage.setItem(
          "uai-default-char",
          JSON.stringify({
            id: def.id,
            name: def.name,
            identity: def.identity || def.personality || "",
            updatedAt: Date.now(),
          }),
        );
      }
    }
  });
  await loadCharacterConversation();
  await fetchConversations();

  try {
    const r = await get<any>("/api/imports/batches");
    importBatches.value = r?.items || [];
  } catch {}

  nextTick(() => inputRef.value?.focus());
});

onUnmounted(() => {
  cleanupSSE();
  disconnectProactiveSSE();
});
</script>
<style scoped>
.webchat-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  width: 100%;
}
.chat-surface { display: flex; flex-direction: column; width: min(100%, 1440px); height: 100%; min-height: 0; margin: 0 auto; overflow: hidden; border: 1px solid var(--surface-border); border-radius: var(--radius-lg); background: var(--chat-surface-bg); }
.chat-header-region { order: 1; flex: 0 0 auto; }
.chat-status-region { order: 2; display: flex; flex-direction: column; gap: 6px; padding: 0 14px; }
.composer-region { order: 4; flex: 0 0 auto; }
@media (max-width: 768px) {
  .webchat-page {
    max-width: 100%;
  }
  .chat-surface { border: 0; border-radius: 0; }
  .chat-sidebar-region { display: none; }
}

.chat-body-wrapper {
  order: 3;
  position: relative;
  flex: 1 1 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
}
.chat-sidebar-region { width: min(320px, 32%); min-width: 220px; padding: 12px; border-left: 1px solid var(--surface-border); overflow: auto; }
@media (min-width: 1024px) { .chat-body-wrapper { flex-direction: row; } }
</style>
