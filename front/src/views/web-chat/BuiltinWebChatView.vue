<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <UIProviderHost
    v-if="externalConversationProvider"
    capability="conversation.shell"
    :provider-id="externalConversationProvider.providerId"
    :fallback="ProviderUnavailableSurface"
    :context="conversationProviderContext"
    :actions="conversationHostActions"
  />
  <div v-else class="webchat-page">
    <section class="chat-surface">
<ChatBanners
      :model-missing="modelMissing"
      :is-offline="isOffline"
      :model-error="modelError"
      :import-context="importContext"
      :show-import-detail="showImportDetail"
      @close-error="modelError = ''"
      @close-import="importContext = null"
    />

    <div class="chat-header-region">
    <UIProviderHost
      capability="conversation.header"
      :fallback="ChatHeaderBar"
      :context="chatExtensionContext"
      :actions="conversationHostActions"
      :char-name="charName"
      :char-avatar="charAvatar"
      :char-identity="charIdentity"
      :conv-title="convTitle"
      :messages-count="messages.length"
      :conv-id="convId"
      :show-profiles="showProfiles"
      :show-mem-inject="showMemInject"
      :call-active="callActive"
      :has-summary="!!convSummary"
      @clear="handleClear"
      @view-memories="handleViewMemories"
      @toggle-char-picker="showCharPicker = true"
      @toggle-profiles="toggleProfiles"
      @toggle-mem-inject="toggleMemInject"
      @toggle-call="handleEndCall"
      @start-call="handleStartCall"
      @view-summary="handleViewSummary"
    ><template #extension-actions><button v-if="hasConversationSidebar && isSmallViewport" type="button" class="sidebar-toggle-btn" aria-label="展开插件侧栏" @click="sidebarDrawerOpen = true"><el-icon><MenuIcon /></el-icon></button><ChatHeaderExtensionHost :context="chatExtensionContext" /></template></UIProviderHost>
    </div>
      <div class="chat-body-wrapper">
      <ExtensionSlot
        slot-id="chat.profile_summary.panel"
        :context="chatExtensionContext"
        fallback="default"
        layout="stack"
        surface-role="overlay"
        bare
      >
        <ProfileSummaryPanel
          :visible="showProfiles"
          :character-id="characterId"
          @close="showProfiles = false"
        />
      </ExtensionSlot>
      <ExtensionSlot
        slot-id="chat.memory_context.panel"
        :context="chatExtensionContext"
        fallback="default"
        layout="stack"
        surface-role="overlay"
        bare
      >
        <MemoryInjectPanel
          :visible="showMemInject"
          :conv-id="convId"
          :character-id="characterId"
          @close="showMemInject = false"
        />
      </ExtensionSlot>
      <UIProviderHost
        capability="conversation.messages"
        :fallback="MessagesArea"
        :context="chatExtensionContext"
        :actions="conversationHostActions"
        ref="msgAreaRef"
        :messages="messages"
        :history-messages="persistedMessages"
        :char-name="charName"
        :char-avatar="charAvatar"
        :character-id="characterId"
        :sending="sending"
        :show-scroll-btn="showScrollBtn"
        :is-pulling="isPulling"
        :pull-ready="pullReady"
        :pull-loading="pullLoading"
        :pull-text="pullText"
        :characters="characters"
        :extension-context="chatExtensionContext"
        :provider-actions="conversationHostActions"
        @scroll="onScroll"
        @wheel="onWheel"
        @touch-start="onMsgTouchStart"
        @touch-move="onMsgTouchMove"
        @touch-end="onMsgTouchEnd"
        @retry="handleRetry"
        @reply="handleSetReply"
        @edit="handleEditMessage"
        @scroll-to-bottom="scrollToBottom(true)"
      />
      <aside v-if="hasConversationSidebar && !isSmallViewport" class="chat-sidebar-region provider-sidebar-region">
        <UIProviderHost
          capability="conversation.sidebar"
          :fallback="BuiltinConversationSidebar"
          :context="chatExtensionContext"
          :actions="conversationHostActions"
          :ui-context-value="chatExtensionContext"
        />
      </aside>
    </div>
    <el-drawer v-if="hasConversationSidebar && isSmallViewport" v-model="sidebarDrawerOpen" direction="rtl" size="320px" :with-header="false" modal-class="sidebar-drawer-modal">
      <UIProviderHost
        capability="conversation.sidebar"
        :fallback="BuiltinConversationSidebar"
        :context="chatExtensionContext"
        :actions="conversationHostActions"
        :ui-context-value="chatExtensionContext"
      />
    </el-drawer>
    <div v-if="hasConversationOverlay" class="chat-status-region provider-overlay-region">
      <UIProviderHost
        capability="conversation.overlay"
        :fallback="BuiltinConversationOverlay"
        :context="chatExtensionContext"
        :actions="conversationHostActions"
        :ui-context-value="chatExtensionContext"
        :call-active="callActive"
        :has-status-extensions="hasStatusExtensions"
      />
    </div>
    <div class="composer-region"><UIProviderHost
      capability="conversation.composer"
      :fallback="ChatInput"
      :context="chatExtensionContext"
      :actions="conversationHostActions"
      ref="inputRef"
      :disabled="modelMissing"
      :sending="sending"
      :generating="generating"
      :is-submitting="isSubmitting"
      :reply-target="replyTarget"
      :character-id="characterId"
      :conversation-id="convId"
      :channel="chatExtensionContext.channel"
      :models="llmModels"
      :selected-model-id="selectedModelId"
      :reasoning-effort="selectedReasoningEffort"
      :supports-reasoning="selectedModelSupportsReasoning"
      :reasoning-enabled="selectedReasoningEnabled"
      :permission-mode="selectedPermissionMode"
      :model-preview-change="handleModelSettingPreviewChange"
      :model-commit-change="handleModelSettingChange"
      @send="handleSend"
      @image="onImageAttached"
      @removeImage="onImageRemoved"
      @stop="handleStop"
      @voiceAudio="handleVoiceAudio"
      @voiceText="handleVoiceText"
      @video="onVideoAttached"
      @removeVideo="onVideoRemoved"
      @file="handleFileSend"
      @cancel-reply="replyTarget = null"
      @update:permission="handlePermissionModeChange"
    /></div>

    <CharacterPickerDialog
      v-model:visible="showCharPicker"
      :characters="characters"
      :character-id="characterId"
      @select="handleSwitchChar"
    />

    <MemoryPanel v-model:visible="showMemories" :memories="memories" />

    <el-drawer v-model="showSummaryDrawer" title="会话摘要" direction="rtl" size="420px">
      <div v-if="convSummary" class="summary-drawer-text">{{ convSummary }}</div>
      <el-empty v-else description="暂无会话摘要" :image-size="88" />
    </el-drawer>

    </section>
  </div>
</template>
<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick, watch, inject } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import { Menu as MenuIcon } from "@element-plus/icons-vue";
import { useApi } from "../../composables/useApi";
import { useCachedApi } from "../../composables/useCachedApi";
import { useChatStore } from "@/stores/chat";
import { useConversationRuntime } from "../../composables/useConversationRuntime";
import { useWebChatScroll } from "../../composables/useWebChatScroll";
import { useWebChatSend } from "../../composables/useWebChatSend";
import { useWebChatConversation } from "../../composables/useWebChatConversation";
import { useConversationWorkspace } from "../../composables/useConversationWorkspace";
import ChatBanners from "../../components/ChatBanners.vue";
import ChatHeaderBar from "../../components/ChatHeaderBar.vue";
import MessagesArea from "../../components/MessagesArea.vue";
import ChatInput from "../../components/ChatInput.vue";
import CharacterPickerDialog from "../../components/CharacterPickerDialog.vue";
import MemoryPanel from "../../components/MemoryPanel.vue";
import ProfileSummaryPanel from "./components/ProfileSummaryPanel.vue";
import MemoryInjectPanel from "./components/MemoryInjectPanel.vue";
import { normalizeRealtimeMessage } from "@/utils/message-order";
import ChatHeaderExtensionHost from "@/components/extension/chat/ChatHeaderExtensionHost.vue";
import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";
import { useExtensionUIStore } from "@/stores/extensionUI";
import { resolveHostEnvironment } from "@/composables/useHostEnvironment";
import UIProviderHost from "@/components/ui-runtime/UIProviderHost.vue";
import ProviderUnavailableSurface from "@/components/ui-runtime/ProviderUnavailableSurface.vue";
import BuiltinConversationSidebar from "@/components/ui-runtime/BuiltinConversationSidebar.vue";
import BuiltinConversationOverlay from "@/components/ui-runtime/BuiltinConversationOverlay.vue";
import { provideConversationUIContext, readonlyMessages } from "@/ui-runtime/conversationContext";
import { browserClientPluginRuntime } from "@/ui-runtime/clientPluginRuntime";
import { hasUnifiedSlotItem } from "@/ui-runtime/slotLedger";

const router = useRouter();
const route = useRoute();
const callActive = ref(false);
const callMode = ref<"voice" | "video" | "screen">("voice");
const ttsVoiceType = ref("");
const ttsResourceId = ref("");
let stopCallWindowListener: (() => void) | null = null;

async function handleStartCall(mode: "voice" | "video" | "screen") {
  callMode.value = mode;
  const desktopApi = window.amitiaDesktop;
  if (!desktopApi?.openRealtimeCallWindow) {
    ElMessage.error("当前环境不支持独立通话窗口");
    return;
  }
  try {
    const result = await desktopApi.openRealtimeCallWindow({
      mode,
      voiceType: ttsVoiceType.value,
      resourceId: ttsResourceId.value,
      conversationId: convId.value,
      charName: charName.value,
      charAvatar: charAvatar.value,
    });
    callActive.value = result.opened;
  } catch (error) {
    callActive.value = false;
    ElMessage.error(error instanceof Error ? error.message : "打开通话窗口失败");
  }
}

async function handleEndCall() {
  await window.amitiaDesktop?.closeRealtimeCallWindow?.();
  callActive.value = false;
}

const { get, post, put, del } = useApi();
const {
  currentWorkspace,
  recentWorkspaces,
  refreshRecentWorkspaces,
  loadConversationWorkspace,
  applySnapshotWorkspace,
  startDraftConversation,
  chooseWorkspaceDirectory,
  selectWorkspaceMount,
  clearWorkspace,
} = useConversationWorkspace();
const { cachedGet, invalidateCache } = useCachedApi();
const currentCharName = inject<any>("currentCharName", null);
const extensionUIStore = useExtensionUIStore();
const chatStore = useChatStore();

const persistedMessages = ref<any[]>([]);
const convId = ref(String(route.query.conversationId || ""));
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
const showSummaryDrawer = ref(false);
const replyTarget = ref<any>(null);
const llmModels = ref<any[]>([]);
const selectedModelId = ref(0);
const selectedReasoningEffort = ref("high");
const selectedReasoningEnabled = ref(true);
const selectedPermissionMode = ref("request_approval");
const selectedModelSupportsReasoning = computed(() => {
  const model = llmModels.value.find(
    (item: any) => Number(item.id) === selectedModelId.value,
  );
  return model?.supportsReasoning === true;
});
const modelSettingsDraftKey = "amitia.chat.model-settings.draft.v1";
const chatExtensionContext = computed(() => {
  const env = resolveHostEnvironment();
  return {
    characterId: characterId.value,
    conversationId: convId.value,
    channel: "web",
    platform: env.platform,
    host: env.host,
    os: env.os,
    conversationState: sending.value ? "generating" : isOffline.value ? "offline" : "idle",
    capabilities: env.host === "desktop" ? ["browser", "desktop", "clipboard-host"] : ["browser"],
    workspace: currentWorkspace.value,
    recentWorkspaces: recentWorkspaces.value,
  };
});
const externalConversationProvider = computed(() => {
  const provider = extensionUIStore.getResolvedProvider("conversation.shell");
  return provider && !provider.builtin ? provider : null;
});
const conversationProviderContext = computed(() => ({
  ...chatExtensionContext.value,
  character: { id: characterId.value, name: charName.value, identity: charIdentity.value, avatar: charAvatar.value },
  conversation: { id: convId.value, title: convTitle.value },
  messages: messages.value,
  sending: sending.value,
  generating: generating.value,
  offline: isOffline.value,
}));
function hasSlotExtensions(slotId: string): boolean {
  browserClientPluginRuntime.slots.revision.value;
  const contract = extensionUIStore.slotsById.get(slotId) ?? browserClientPluginRuntime.slots.getDefinition(slotId);
  return hasUnifiedSlotItem(
    contract,
    extensionUIStore.getVisibleContributions(slotId, chatExtensionContext.value),
    browserClientPluginRuntime.slots.listContributions(slotId),
  );
}
const hasSidebarExtensions = computed(() => hasSlotExtensions("chat.sidebar.panel"));
const hasStatusExtensions = computed(() => hasSlotExtensions("chat.status.item"));
const hasExternalSidebarProvider = computed(() => {
  const provider = extensionUIStore.getResolvedProvider("conversation.sidebar");
  return !!provider && provider.enabled && !provider.builtin;
});
const hasExternalOverlayProvider = computed(() => {
  const provider = extensionUIStore.getResolvedProvider("conversation.overlay");
  return !!provider && provider.enabled && !provider.builtin;
});
const hasConversationSidebar = computed(() => hasSidebarExtensions.value || hasExternalSidebarProvider.value);
const hasConversationOverlay = computed(() => callActive.value || hasStatusExtensions.value || hasExternalOverlayProvider.value);
const sidebarDrawerOpen = ref(false);
const isSmallViewport = ref(window.innerWidth < 1024);

function updateViewport() {
  isSmallViewport.value = window.innerWidth < 1024;
}

const msgAreaRef = ref<InstanceType<typeof MessagesArea>>();
const inputRef = ref<InstanceType<typeof ChatInput>>();

const currentImageBase64 = ref<string | null>(null);
const currentImageFile = ref<File | null>(null);
const pendingImageBase64 = ref<string | null>(null);
const pendingAudioUrl = ref<string | null>(null);
const pendingVideoUrl = ref<string | null>(null);

async function handleNewChat(event?: CustomEvent) {
  const providedId = event?.detail?.conversationId;
  try {
    if (providedId) {
      disconnectAndResetConversation();
      convId.value = providedId;
      convTitle.value = "";
      replyTarget.value = null;
      await router.replace({ path: "/chat", query: { conversationId: providedId } });
      nextTick(() => scrollToBottom(true));
      connectSSE();
      return;
    }
    disconnectAndResetConversation();
    convId.value = "";
    convTitle.value = "";
    replyTarget.value = null;
    await startDraftConversation();
    await router.replace({ path: "/chat" });
    nextTick(() => inputRef.value?.focus());
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.msg || e?.message || "新建对话失败");
  }
}

async function handleFileSend(file: File) {
  if (!file) return;
  if (!convId.value || !characterId.value) {
    ElMessage.warning("请先选择角色和会话");
    return;
  }
  try {
    const form = new FormData();
    form.append("kind", "file");
    form.append("source", "chat_upload");
    form.append("file", file, file.name);
    const response = await post<any>("/api/artifacts/v1", form);
    const artifact = response?.artifact ?? response;
    const artifactId = String(artifact?.artifactId ?? artifact?.id ?? "").trim();
    if (!artifactId) throw new Error("上传结果缺少 artifactId");
    const filename = String(artifact?.filename || file.name || "文件");
    const resourceUri = `amitia://artifacts/${artifactId}`;
    await handleSend(`[文件] ${filename}\n${resourceUri}`);
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.message || error?.message || "文件上传失败");
  }
}

async function deleteConversationMessage(messageId: string) {
  const id = String(messageId || "").trim();
  if (!id) return;
  await del(`/api/chats/messages/${encodeURIComponent(id)}`);
  const index = persistedMessages.value.findIndex((item) => String(item.id) === id);
  if (index >= 0) persistedMessages.value.splice(index, 1);
}

async function handleEditMessage(msg: any) {
  if (!msg?.id || msg.role !== "user") return;
  try {
    const result = await ElMessageBox.prompt("修改后将更新当前用户消息内容。", "修改消息", {
      inputValue: String(msg.content || ""),
      inputType: "textarea",
      inputPlaceholder: "输入新的消息内容",
      confirmButtonText: "保存",
      cancelButtonText: "取消",
      inputValidator: (value) => String(value || "").trim() ? true : "消息内容不能为空",
    });
    const content = String(result.value || "").trim();
    if (!content || content === String(msg.content || "")) return;
    const updated = await put<any>(`/api/web-chat/messages/${encodeURIComponent(msg.id)}`, { content });
    const index = persistedMessages.value.findIndex((item) => String(item.id) === String(msg.id));
    if (index >= 0) {
      persistedMessages.value[index] = {
        ...persistedMessages.value[index],
        content: updated?.content ?? content,
        updatedAt: updated?.updatedAt ?? new Date().toISOString(),
      };
    }
    ElMessage.success("消息已修改");
  } catch (error: any) {
    if (error === "cancel" || error === "close") return;
    ElMessage.error(error?.message || "修改失败");
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

async function loadLlmModels() {
  try {
    const models = await get<any[]>("/api/model/configs");
    llmModels.value = Array.isArray(models)
      ? models.filter((model: any) => {
          const type = String(model.apiType || "").toLowerCase();
          return !["voice", "asr", "embedding", "vector", "vision", "imagegen"].includes(type);
        })
      : [];
    if (!selectedModelId.value && llmModels.value.length > 0) {
      const active = llmModels.value.find((model: any) => !!model.isActive);
      selectedModelId.value = Number((active || llmModels.value[0]).id || 0);
    }
  } catch {
    llmModels.value = [];
  }
}

function loadDraftModelSettings() {
  try {
    const draft = JSON.parse(
      localStorage.getItem(modelSettingsDraftKey) || "{}",
    );
    selectedModelId.value = Number(draft.modelConfigId || 0);
    selectedReasoningEffort.value = String(draft.reasoningEffort || "high");
    selectedReasoningEnabled.value = draft.reasoningEnabled === true;
    selectedPermissionMode.value =
      draft.permissionMode === "full_access" ? "full_access" : "request_approval";
  } catch {}
}

function applyConversationSettings(conversation?: Record<string, any>) {
  if (!conversation) return;
  selectedModelId.value = Number(conversation.modelConfigId || 0);
  selectedReasoningEffort.value = String(conversation.reasoningEffort || "high");
  selectedReasoningEnabled.value =
    conversation.reasoningEnabled === 1 || conversation.reasoningEnabled === true;
  selectedPermissionMode.value =
    conversation.permissionMode === "full_access" ? "full_access" : "request_approval";
  localStorage.removeItem(modelSettingsDraftKey);
}

function saveModelSettingsDraft(
  modelId: number,
  reasoningEffort: string,
  reasoningEnabled: boolean,
) {
  try {
    localStorage.setItem(
      modelSettingsDraftKey,
      JSON.stringify({
        modelConfigId: modelId,
        reasoningEffort,
        reasoningEnabled,
        permissionMode: selectedPermissionMode.value,
      }),
    );
  } catch {}
}

async function handlePermissionModeChange(mode: string) {
  const next = mode === "full_access" ? "full_access" : "request_approval";
  selectedPermissionMode.value = next;
  if (!convId.value) {
    saveModelSettingsDraft(
      selectedModelId.value,
      selectedReasoningEffort.value,
      selectedReasoningEnabled.value,
    );
    return;
  }
  try {
    await put(
      `/api/web-chat/conversations/${encodeURIComponent(convId.value)}`,
      { permissionMode: next },
    );
  } catch {
    await reloadConversationSnapshot();
    ElMessage.error("保存权限模式失败");
  }
}

async function handleModelSettingChange(
  modelId: number,
  reasoningEffort: string,
  reasoningEnabled: boolean,
) {
  const nextModelId = Number(modelId || 0);
  const nextReasoningEffort = reasoningEffort || "high";
  const nextReasoningEnabled = reasoningEnabled === true;
  handleModelSettingPreviewChange(
    nextModelId,
    nextReasoningEffort,
    nextReasoningEnabled,
  );
  if (!convId.value) {
    saveModelSettingsDraft(
      nextModelId,
      nextReasoningEffort,
      nextReasoningEnabled,
    );
    return;
  }
  try {
    await put(
      `/api/web-chat/conversations/${encodeURIComponent(convId.value)}`,
      {
        modelConfigId: nextModelId,
        reasoningEffort: nextReasoningEffort,
        reasoningEnabled: nextReasoningEnabled,
      },
    );
  } catch {
    await reloadConversationSnapshot();
    ElMessage.error("保存模型设置失败");
  }
}

function handleModelSettingPreviewChange(
  modelId: number,
  reasoningEffort: string,
  reasoningEnabled: boolean,
) {
  selectedModelId.value = Number(modelId || 0);
  selectedReasoningEffort.value = reasoningEffort || "high";
  selectedReasoningEnabled.value = reasoningEnabled === true;
}

let loadOlderRuntimeTurns: () => Promise<unknown> = async () => undefined;

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
  applyHistorySnapshot,
} = useWebChatScroll(
  msgAreaRef,
  persistedMessages,
  convId,
  showScrollBtn,
  () => loadOlderRuntimeTurns(),
);

const {
  messages,
  activeTurnId,
  loadSnapshot: reloadConversationSnapshot,
  loadOlderTurns: loadOlderConversationTurns,
  connect: connectSSE,
  disconnect: disconnectSSE,
  clear: clearRuntime,
  beginPendingAssistant,
  failPendingAssistant,
  cleanup: cleanupSSE,
  connectProactiveMessages: connectProactiveSSE,
  disconnectProactiveMessages: disconnectProactiveSSE,
} = useConversationRuntime(
  convId,
  persistedMessages,
  sending,
  scrollToBottom,
  async (conversation, workspace, snapshot) => {
    const title = String(conversation?.title || "").trim();
    if (title) convTitle.value = title;
    applyConversationSettings(conversation);
    applySnapshotWorkspace(workspace, String(conversation?.projectId || ""));
    applyHistorySnapshot(snapshot?.messageHistory);
  },
);

loadOlderRuntimeTurns = loadOlderConversationTurns;

function resetConversationMessages() {
  clearRuntime();
  persistedMessages.value = [];
}

function disconnectAndResetConversation() {
  disconnectSSE();
  resetConversationMessages();
}

const {
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
  handleClear,
  generating,
  isSubmitting,
} = useWebChatSend(
  persistedMessages,
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
  disconnectAndResetConversation,
  inputRef,
  undefined,
  replyTarget,
  async (conversationId) => {
    await router.replace({
      path: "/chat",
      query: { conversationId },
    });
    void connectSSE(false);
  },
  selectedModelId,
  selectedReasoningEffort,
  selectedReasoningEnabled,
  selectedPermissionMode,
  activeTurnId,
  beginPendingAssistant,
  failPendingAssistant,
);

const {
  characters,
  conversations,
  memories,
  showCharPicker,
  showMemories,
  selectCharacter,
  handleSwitchChar,
  loadCharacterConversation,
  fetchConversations,
  handleViewMemories,
  refreshCharacters,
  fetchConvSummary,
} = useWebChatConversation(
  persistedMessages,
  convId,
  characterId,
  convTitle,
  charName,
  charIdentity,
  charAvatar,
  hasMoreHistory,
  disconnectAndResetConversation,
  connectSSE,
);

const conversationHostActions: Record<string, (input?: any) => unknown | Promise<unknown>> = {
  "conversation.send": async (input) => handleSend(String(input?.text ?? input ?? ""), input?.imageBase64, input?.videoBase64),
  "conversation.stop": async () => handleStop(),
  "conversation.retry": async (input) => { const id = String(input?.messageId ?? input ?? ""); const msg = messages.value.find((item) => item.id === id); if (msg) await handleRetry(msg); },
  "conversation.delete": async (input) => deleteConversationMessage(String(input?.messageId ?? input ?? "")),
  "conversation.new": async () => handleNewChat(),
  "conversation.clear": async () => handleClear(),
  "conversation.reply": async (input) => { const msg = messages.value.find((item) => item.id === String(input?.messageId ?? input ?? "")); if (msg) handleSetReply(msg); },
  "conversation.edit": async (input) => { const msg = messages.value.find((item) => item.id === String(input?.messageId ?? input ?? "")); if (msg) await handleEditMessage(msg); },
  "conversation.sendFile": async (input) => {
    if (input?.file instanceof File) return handleFileSend(input.file);
    const resourceUri = String(input?.resourceUri ?? "").trim();
    const fileName = String(input?.fileName ?? input?.filename ?? "文件").trim() || "文件";
    if (resourceUri) return handleSend(`[文件] ${fileName}\n${resourceUri}`);
  },
  "conversation.sendImage": async (input) => {
    const imageBase64 = String(input?.imageBase64 ?? "").trim();
    if (imageBase64) return handleSend(String(input?.text ?? ""), imageBase64);
  },
  "conversation.sendCode": async (input) => {
    const language = String(input?.language ?? "text").trim() || "text";
    const code = String(input?.code ?? "");
    if (code) return handleSend(`\`\`\`${language}\n${code}\n\`\`\``);
  },
  "conversation.sendVoice": async (input) => {
    if (input?.blob instanceof Blob) return handleVoiceAudio(input.blob, input?.transcript, input?.duration);
    if (input?.text) return handleVoiceText(String(input.text));
  },
  "conversation.workspace.choose": async () => chooseWorkspaceDirectory(),
  "conversation.workspace.select": async (input) => {
    const workspaceId = String(input?.workspaceId ?? input ?? "").trim();
    if (!workspaceId) return chooseWorkspaceDirectory();
    let mount = recentWorkspaces.value.find((item) => item.id === workspaceId);
    if (!mount) {
      await refreshRecentWorkspaces();
      mount = recentWorkspaces.value.find((item) => item.id === workspaceId);
    }
    if (mount) return selectWorkspaceMount(mount);
  },
  "conversation.workspace.clear": async () => clearWorkspace(),
  "conversation.workspace.refresh": async () => refreshRecentWorkspaces(),
};

watch(
  convId,
  (conversationId) => {
    const id = conversationId || "";
    loadConversationWorkspace(id, id ? "" : String(route.query.projectId || ""));
    const requestedId = id;
    void fetchConvSummary(requestedId).then((summary) => {
      if ((convId.value || "") === requestedId) {
        convSummary.value = summary;
      }
    });
  },
  { immediate: true },
);

watch(
  convId,
  (conversationId) => {
    if (!conversationId) loadDraftModelSettings();
  },
  { immediate: true },
);

watch(
  () => String(route.query.conversationId || ""),
  async (nextId) => {
    if (!nextId) {
      disconnectAndResetConversation();
      convId.value = "";
      convTitle.value = "";
      replyTarget.value = null;
      await loadConversationWorkspace("", String(route.query.projectId || ""));
      return;
    }
    if (nextId === convId.value) return;
    convId.value = nextId;
    convTitle.value = "新对话";
    await loadCharacterConversation();
  },
);

watch(
  () => String(route.query.projectId || ""),
  async (projectId) => {
    if (convId.value) return;
    await loadConversationWorkspace("", projectId);
  },
);

watch(showSummaryDrawer, (visible) => {
  if (!visible) return;
  const requestedId = convId.value || "";
  void fetchConvSummary(requestedId).then((summary) => {
    if ((convId.value || "") === requestedId) {
      convSummary.value = summary;
    }
  });
});

function handleViewSummary() {
  showSummaryDrawer.value = true;
}

provideConversationUIContext({
  conversationId: convId,
  characterId,
  messages: readonlyMessages(messages),
  sending,
  generating,
  offline: isOffline,
  actions: {
    async send(input) { await handleSend(input.text ?? ""); },
    async stop() { await handleStop(); },
    async retry(messageId) {
      const index = messages.value.findIndex((item) => item.id === messageId);
      if (index >= 0) await handleRetry(messages.value[index]);
    },
    async createConversation() { await handleNewChat(); },
  },
});

watch(isOffline, (offline) => {
  if (
    !offline &&
    sending.value &&
    persistedMessages.value.some((m) => m.status === "sending")
  ) {
    ElMessage.info("网络已恢复，可重新发送消息");
  }
});

onMounted(async () => {
  stopCallWindowListener =
    window.amitiaDesktop?.onRealtimeCallWindowClosed?.(() => {
      callActive.value = false;
    }) ?? null;
  void loadLlmModels();
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

  window.addEventListener("resize", updateViewport);
  updateViewport();

  const h = await get<any>("/api/health").catch(() => null);
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
  if (convId.value) convTitle.value = "新对话";
  await loadCharacterConversation();
  await fetchConversations();

  nextTick(() => inputRef.value?.focus());
});

onUnmounted(() => {
  stopCallWindowListener?.();
  stopCallWindowListener = null;
  cleanupSSE();
  disconnectProactiveSSE();
  window.removeEventListener("resize", updateViewport);
});
</script>
<style scoped>
.webchat-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  width: 100%;
}
.summary-drawer-text {
  white-space: pre-wrap;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.7;
}
.chat-surface { display: flex; flex-direction: column; width: min(100%, 1440px); height: 100%; min-height: 0; margin: 0 auto; overflow: hidden; border-radius: var(--radius-lg); background: var(--chat-surface-bg); }
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
.chat-sidebar-region { width: min(320px, 32%); min-width: 220px; padding: 12px; border-left: 1px solid var(--surface-border); height: 100%; min-height: 0; overflow: hidden; display: flex; flex-direction: column; flex: 0 0 auto; }
@media (min-width: 1024px) { .chat-body-wrapper { flex-direction: row; } }
.sidebar-toggle-btn { display: grid; place-items: center; width: 32px; height: 32px; border: 1px solid transparent; border-radius: var(--radius-sm); background: transparent; color: var(--text-secondary); cursor: pointer; }
.sidebar-toggle-btn:hover { border-color: var(--surface-border); background: var(--control-hover-bg); color: var(--text-primary); }
</style>
