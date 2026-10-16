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
  <div v-else class="webchat-page" data-amitia-ui="chat.page">
    <el-dialog v-if="boundCallScope" v-model="callActive" data-amitia-ui="component.dialog" fullscreen :show-close="false" :close-on-click-modal="false" :close-on-press-escape="false" destroy-on-close>
      <RealtimeCallDialog :mode="callMode" voice-type="" resource-id="" :conversation-id="boundCallConversation" :character-id="boundCallScope.roleId" :expected-execution-scope="boundCallScope" :historical-role-id="boundCallHistoricalRole" :char-name="charName" :char-avatar="charAvatar" @close="handleEndCall" @conversation-saved="handleOwnedCallSaved" />
    </el-dialog>
    <section ref="chatSurfaceRef" class="chat-surface" :class="{ 'chat-surface--sidebar': conversationSidebarVisible }" data-amitia-ui="chat.surface">
    <AutomationStatusIndicator :status="automationStatus" />

    <div ref="chatHeaderRef" class="chat-header-region">
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
    ><template #extension-actions><button type="button" class="sidebar-toggle-btn" aria-label="切换工作区" title="工作区" @click="workbench.toggleVisible()"><el-icon><GridIcon /></el-icon></button><button v-if="hasConversationSidebar && isSmallViewport" type="button" class="sidebar-toggle-btn" aria-label="展开插件侧栏" @click="sidebarDrawerOpen = true"><el-icon><MenuIcon /></el-icon></button><ChatHeaderExtensionHost :context="chatExtensionContext" /></template></UIProviderHost>
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
        :device-owned="owned.enabled.value"
        :model-error="modelError"
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
        :notice="owned.notice.value"
        :model-missing="modelMissing"
        :is-offline="isOffline"
        :import-context="importContext"
        :show-import-detail="showImportDetail"
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
        @close-import="importContext = null"
        @toggle-import-detail="showImportDetail = !showImportDetail"
        @scroll-to-bottom="scrollToBottom(true)"
      />
      <aside v-if="hasConversationSidebar && !isSmallViewport && !workbench.visible" class="chat-sidebar-region provider-sidebar-region">
        <UIProviderHost
          capability="conversation.sidebar"
          :fallback="BuiltinConversationSidebar"
          :context="chatExtensionContext"
          :actions="conversationHostActions"
          :ui-context-value="chatExtensionContext"
        />
      </aside>
    </div>
    <el-drawer v-if="hasConversationSidebar && isSmallViewport" v-model="sidebarDrawerOpen" data-amitia-ui="component.bottom_sheet" direction="rtl" size="320px" :with-header="false" modal-class="sidebar-drawer-modal">
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
    <div class="chat-scroll-fade chat-scroll-fade--top" aria-hidden="true"></div>
    <div class="chat-scroll-fade chat-scroll-fade--bottom" aria-hidden="true"></div>
    <div ref="composerRegionRef" class="composer-region" data-amitia-ui="chat.composer"><UIProviderHost
      capability="conversation.composer"
      :fallback="ChatInput"
      :context="chatExtensionContext"
      :actions="conversationHostActions"
      ref="inputRef"
      :disabled="modelMissing"
      :device-owned="owned.enabled.value"
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

    <DeviceOwnedMemoryPanel v-model:visible="showMemories" :character-id="characterId" :conversation-id="convId" />

    <el-drawer v-model="showSummaryDrawer" title="会话摘要" data-amitia-ui="component.drawer" direction="rtl" size="420px">
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
import { Grid as GridIcon, Menu as MenuIcon } from "@element-plus/icons-vue";
import { useApi } from "../../composables/useApi";
import { useCachedApi } from "../../composables/useCachedApi";
import { useChatStore } from "@/stores/chat";
import { useWorkbenchStore } from "@/stores/workbench";
import { useConversationRuntime } from "../../composables/useConversationRuntime";
import AutomationStatusIndicator from "../../components/AutomationStatusIndicator.vue";
import { useWebChatScroll } from "../../composables/useWebChatScroll";
import { useWebChatSend } from "../../composables/useWebChatSend";
import { useWebChatConversation } from "../../composables/useWebChatConversation";
import { ownedConversationSwitchNotice, useDeviceOwnedConversation } from "../../composables/useDeviceOwnedConversation";
import { OwnedChatStateChangedError, pollOwnedChat } from "@/runtime/owned-chat-poll";
import { useConversationWorkspace } from "../../composables/useConversationWorkspace";
import {
  loadChatPermissionMode,
  normalizeChatPermissionMode,
  saveChatPermissionMode,
} from "../../composables/useChatPermissionPreference";
import ChatHeaderBar from "../../components/ChatHeaderBar.vue";
import RealtimeCallDialog from "../../components/RealtimeCallDialog.vue";
import type { OwnedChatResponse, OwnedExecutionScope } from "../../runtime/device-owned-chat";
import { conversationReference } from "../../runtime/device-owned-conversation-reference";
import MessagesArea from "../../components/MessagesArea.vue";
import ChatInput from "../../components/ChatInput.vue";
import CharacterPickerDialog from "../../components/CharacterPickerDialog.vue";
import DeviceOwnedMemoryPanel from "../../components/DeviceOwnedMemoryPanel.vue";
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
const boundCallScope = ref<OwnedExecutionScope | null>(null);
const boundCallConversation = ref("");
const boundCallHistoricalRole = ref<string | undefined>();
const ttsVoiceType = ref("");
const ttsResourceId = ref("");
let stopCallWindowListener: (() => void) | null = null;

async function handleStartCall(mode: "voice" | "video" | "screen") {
  callMode.value = mode;
  try {
    if (await owned.refresh()) {
      const originalRole = characterId.value;
      const originalConversation = convId.value;
      if (!originalRole) throw new Error("请先选择一个已保存的角色，再发起通话");
      const snapshot = originalConversation ? await owned.query(originalConversation, originalRole) : await owned.data("working", originalRole);
      if (characterId.value !== originalRole || convId.value !== originalConversation || !snapshot?.executionScope || snapshot.executionScope.roleId !== originalRole) throw new Error("通话角色或会话已变化，请重新发起");
      boundCallScope.value = JSON.parse(JSON.stringify(snapshot.executionScope));
      boundCallConversation.value = originalConversation;
      const historicalConversation = snapshot.historicalSnapshot?.resources.find((item) => item.kind === "conversation");
      boundCallHistoricalRole.value = historicalConversation?.roleId || snapshot.historicalSnapshot?.legacyConversations?.[0]?.characterId || undefined;
      callActive.value = true;
      return;
    }
  } catch (error) { ElMessage.error(error instanceof Error ? error.message : "无法确认通话服务"); return; }
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
  if (boundCallScope.value) {
    callActive.value = false;
    boundCallScope.value = null;
    return;
  }
  await window.amitiaDesktop?.closeRealtimeCallWindow?.();
  callActive.value = false;
}

async function handleOwnedCallSaved(result: OwnedChatResponse) {
  if (!boundCallScope.value || !callActive.value || result.executionScope.coreId !== boundCallScope.value.coreId || result.executionScope.roleId !== characterId.value) return;
  const origin = result.conversationOrigin || { ownerId: result.executionScope.resourceOwnerId, id: result.conversationId };
  const reference = conversationReference(origin);
  if (convId.value !== boundCallConversation.value) return;
  convId.value = reference;
  boundCallConversation.value = reference;
  const rows = owned.messages(await owned.query(reference, characterId.value));
  if (callActive.value && convId.value === reference) messages.value = rows;
}

const { get, post, put, del } = useApi();
const owned = useDeviceOwnedConversation();
let ownedPolicyTimer: ReturnType<typeof setInterval> | null = null;
let ownedPolling = false;
let ownedDisposed = false;
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
const workbench = useWorkbenchStore();

const persistedMessages = ref<any[]>([]);
const convId = ref(String(route.query.conversationId || ""));
const convTitle = ref("");
const characterId = ref("");
watch([convId, characterId], ([id, role]) => workbench.setConversation(id, role), { immediate: true });
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
const selectedPermissionMode = ref(loadChatPermissionMode());
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

const chatSurfaceRef = ref<HTMLElement>();
const chatHeaderRef = ref<HTMLElement>();
const composerRegionRef = ref<HTMLElement>();
const conversationSidebarVisible = computed(() => hasConversationSidebar.value && !isSmallViewport.value && !workbench.visible);
let chatLayoutObserver: ResizeObserver | null = null;

function syncChatLayoutVars() {
  const surface = chatSurfaceRef.value;
  if (!surface) return;
  const headerHeight = chatHeaderRef.value?.getBoundingClientRect().height ?? 0;
  const composerHeight = composerRegionRef.value?.getBoundingClientRect().height ?? 0;
  surface.style.setProperty("--chat-header-h", `${Math.round(headerHeight)}px`);
  surface.style.setProperty("--chat-composer-h", `${Math.round(composerHeight)}px`);
}

function startChatLayoutObserver() {
  if (typeof ResizeObserver === "undefined") return;
  chatLayoutObserver = new ResizeObserver(() => syncChatLayoutVars());
  for (const element of [chatHeaderRef.value, composerRegionRef.value]) {
    if (element) chatLayoutObserver.observe(element);
  }
  syncChatLayoutVars();
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
    selectedPermissionMode.value = loadChatPermissionMode();
    await startDraftConversation();
    await router.replace({ path: "/chat" });
    nextTick(() => inputRef.value?.focus());
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.msg || e?.message || "新建对话失败");
  }
}

async function handleFileSend(file: File) {
  if (!file) return;
  try { if (await owned.refresh()) { if (!characterId.value) { ElMessage.warning("请先选择调用角色"); return; } await handleOwnedFileSend(file); return; } } catch (error: any) { ElMessage.error(error?.message || "文件发送失败"); return; }
  ElMessage.error(owned.notice.value || "设备对话服务尚未就绪，附件未发送");
}

function findConversationMessage(messageId: string, ownerId?: string) {
  const matches = persistedMessages.value.filter((item: any) => String(item.id) === messageId && (ownerId === undefined || item.ownerId === ownerId));
  if (matches.length !== 1) throw new Error("消息来源不唯一或已变化，请重新选择消息");
  return matches[0];
}

async function deleteConversationMessage(messageId: string, ownerId?: string) {
  const id = String(messageId || "").trim();
  if (!id) return;
  if (await owned.refresh()) {
    const message = findConversationMessage(id, ownerId);
    if (!message?.executionScope || !message?.ownerId || !message?.sourceRevision) throw new Error("该消息缺少原始数据来源，请重新加载或在原设备管理历史数据");
    await owned.edit("message", id, {}, { deleted: true, characterId: message.executionScope.roleId, expectedExecutionScope: message.executionScope, expectedOwnerId: message.ownerId, expectedRevision: message.sourceRevision });
  }
  else throw new Error(owned.notice.value || "设备对话服务尚未就绪");
  const index = persistedMessages.value.findIndex((item: any) => String(item.id) === id && (ownerId === undefined || item.ownerId === ownerId));
  if (index >= 0) persistedMessages.value.splice(index, 1);
}

async function handleEditMessage(msg: any) {
  if (!msg?.id || msg.role !== "user") return;
  msg = JSON.parse(JSON.stringify(msg));
  try {
    const bound = await owned.refresh();
    if (!bound) throw new Error(owned.notice.value || "设备对话服务尚未就绪");
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
    if (bound && (!msg.executionScope || !msg.ownerId || !msg.sourceRevision)) throw new Error("该消息缺少原始数据来源，请重新加载或在原设备管理历史数据");
    await owned.edit("message", msg.id, { content }, { characterId: msg.executionScope.roleId, expectedExecutionScope: msg.executionScope, expectedOwnerId: msg.ownerId, expectedRevision: msg.sourceRevision });
    const updated = { content, sourceRevision: msg.sourceRevision + 1, updatedAt: new Date().toISOString() };
    const index = persistedMessages.value.findIndex((item: any) => String(item.id) === String(msg.id) && item.ownerId === msg.ownerId);
    if (index >= 0) {
      persistedMessages.value[index] = {
        ...persistedMessages.value[index],
        content: updated?.content ?? content,
        sourceRevision: updated?.sourceRevision ?? persistedMessages.value[index].sourceRevision,
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
    ownerId: msg.ownerId,
    executionScope: msg.executionScope ? JSON.parse(JSON.stringify(msg.executionScope)) : undefined,
    id: msg.id,
    characterId: msg.characterId || msg.executionScope?.roleId,
    sourceConversationId: msg.sourceConversationId || msg.conversationId,
    sourceRevision: msg.sourceRevision ?? 0,
    fullContent: String(msg.content || ""),
    role: msg.role,
    content: (msg.content || "").slice(0, 100),
  };
}

async function loadLlmModels() {
  try {
    const models = await get<any[]>("/api/model/available");
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
    selectedPermissionMode.value = loadChatPermissionMode();
  } catch {}
}

function applyConversationSettings(conversation?: Record<string, any>) {
  if (!conversation) return;
  selectedModelId.value = Number(conversation.modelConfigId || 0);
  selectedReasoningEffort.value = String(conversation.reasoningEffort || "high");
  selectedReasoningEnabled.value =
    conversation.reasoningEnabled === 1 || conversation.reasoningEnabled === true;
  selectedPermissionMode.value = normalizeChatPermissionMode(
    conversation.permissionMode,
  );
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
      }),
    );
  } catch {}
}

async function handlePermissionModeChange(mode: string) {
  const next = normalizeChatPermissionMode(mode);
  selectedPermissionMode.value = next;
  saveChatPermissionMode(next);
  if (!convId.value) {
    saveModelSettingsDraft(
      selectedModelId.value,
      selectedReasoningEffort.value,
      selectedReasoningEnabled.value,
    );
    return;
  }
  try {
    await owned.edit(
      "conversation", convId.value,
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
    await owned.edit(
      "conversation", convId.value,
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
  characterId,
);

const {
  messages,
  activeTurnId,
  automationStatus,
  loadSnapshot: reloadConversationSnapshot,
  loadOlderTurns: loadOlderConversationTurns,
  connect: connectSSE,
  disconnect: disconnectSSE,
  clear: clearRuntime,
  beginPendingAssistant,
  failPendingAssistant,
  cleanup: cleanupSSE,
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
    if (conversation?.id) {
      await chatStore.markConversationRead(String(conversation.id), Number(conversation.lastTerminalTurnSequence || 0)).catch(() => undefined);
    }
  },
  async () => {
    await chatStore.fetchSidebar();
  },
);

loadOlderRuntimeTurns = loadOlderConversationTurns;

function resetConversationMessages() {
  clearRuntime();
  persistedMessages.value = [];
}

function disconnectAndResetConversation() {
  if (owned.enabled.value && sending.value) owned.stopLocal(ownedConversationSwitchNotice, true);
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
  handleFileSend: handleOwnedFileSend,
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
    await fetchConversations();
    await chatStore.fetchSidebar();
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
  "conversation.delete": async (input) => deleteConversationMessage(String(input?.messageId ?? input ?? ""), input?.ownerId),
  "conversation.new": async () => handleNewChat(),
  "conversation.clear": async () => handleClear(),
  "conversation.reply": async (input) => { const msg = messages.value.find((item) => item.id === String(input?.messageId ?? input ?? "")); if (msg) handleSetReply(msg); },
  "conversation.edit": async (input) => { const msg = findConversationMessage(String(input?.messageId ?? input ?? ""), input?.ownerId); await handleEditMessage(msg); },
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
  "conversation.workbench.open": async (input) => workbench.openTab(input),
  "conversation.workbench.toggle": async () => workbench.toggleVisible(),
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
  () => [String(route.query.conversationId || ""), String(route.query.characterId || ""), String(route.query.ownerId || "")],
  async ([nextId, nextRole, nextOwner]) => {
    if (nextOwner) {
      try { await owned.refresh(); } catch (error: any) { modelError.value = error?.message || '无法确认通知执行核心'; return; }
      if (owned.coreId.value !== nextOwner) {
        disconnectAndResetConversation();
        convId.value = '';
        modelError.value = '通知所属核心与当前核心不同，请先切换至原核心后打开';
        return;
      }
    }
    const roleChanged = Boolean(nextRole && nextRole !== characterId.value);
    if (nextRole && nextRole !== characterId.value) {
      await owned.refresh();
      const selected = owned.roles.value.find((role) => role.id === nextRole);
      if (!selected) { modelError.value = '通知所属角色不可用，无法打开会话'; return; }
      selectCharacter(selected);
    }
    if (!nextId) {
      disconnectAndResetConversation();
      convId.value = "";
      convTitle.value = "";
      replyTarget.value = null;
      selectedPermissionMode.value = loadChatPermissionMode();
      await loadConversationWorkspace("", String(route.query.projectId || ""));
      return;
    }
    if (nextId === convId.value && !roleChanged) return;
    if (owned.enabled.value && sending.value) owned.stopLocal(ownedConversationSwitchNotice, true);
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
  startChatLayoutObserver();
  window.addEventListener("amitia:execution-scope-changed", handleScopeChanged);
  stopCallWindowListener =
    window.amitiaDesktop?.onRealtimeCallWindowClosed?.(() => {
      callActive.value = false;
    }) ?? null;
  void loadLlmModels();
  let ownedInitialized = false;
  let ownedDataRefreshPending = false;
  let ownedLoadError = "";
  const reportOwnedLoadError = (error: unknown) => {
    ownedLoadError = error instanceof Error ? error.message : "聊天数据加载失败，请重试";
    modelError.value = ownedLoadError;
  };
  const clearOwnedLoadError = () => {
    if (ownedLoadError && modelError.value === ownedLoadError) modelError.value = "";
    ownedLoadError = "";
  };
  const initializeOwned = async () => {
    characters.value = owned.roles.value;
    const notificationOwner = String(route.query.ownerId || '');
    if (notificationOwner && notificationOwner !== owned.coreId.value) {
      disconnectAndResetConversation();
      convId.value = '';
      modelError.value = '通知所属核心与当前核心不同，请先切换至原核心后打开';
      return;
    }
    const notificationRole = String(route.query.characterId || '');
    if (notificationRole && !characters.value.some((role) => role.id === notificationRole)) {
      disconnectAndResetConversation();
      convId.value = '';
      modelError.value = '通知所属角色不可用，无法打开会话';
      return;
    }
    const selectedId = owned.selectInitialRole(String(route.query.characterId || "") || localStorage.getItem(`webchat-char-id:${owned.coreId.value}:${owned.roleOwnerId.value}`) || "");
    const selected = characters.value.find((character) => character.id === selectedId);
    if (selected) selectCharacter(selected);
    else { characterId.value = ""; modelError.value = characters.value.length ? "请选择调用角色后发送消息" : "没有可用角色，拒绝调用"; }
    if (characterId.value) { await chatStore.fetchSidebar(); await loadCharacterConversation(); await fetchConversations(); }
    clearOwnedLoadError();
    ownedInitialized = true;
  };
  const startOwnedPolling = () => {
    if (ownedDisposed || ownedPolicyTimer) return;
    ownedPolicyTimer = setInterval(async () => {
      if (ownedDisposed || ownedPolling) return;
      ownedPolling = true;
      try {
        const previousProvider = `${owned.coreId.value}/${owned.policy.value?.providerEpoch}/${owned.policy.value?.modeRevision}/${owned.policy.value?.permissionRevision}`;
        await pollOwnedChat(owned, async () => {
          if (!ownedInitialized) { await initializeOwned(); return; }
          characters.value = owned.roles.value;
          const notificationOwner = String(route.query.ownerId || '');
          if (notificationOwner && notificationOwner !== owned.coreId.value) {
            disconnectAndResetConversation(); convId.value = '';
            modelError.value = '通知所属核心与当前核心不同，请先切换至原核心后打开';
            return;
          }
          const currentProvider = `${owned.coreId.value}/${owned.policy.value?.providerEpoch}/${owned.policy.value?.modeRevision}/${owned.policy.value?.permissionRevision}`;
          if (previousProvider !== currentProvider || ownedDataRefreshPending) {
            await chatStore.fetchSidebar();
            ownedDataRefreshPending = false;
            clearOwnedLoadError();
          }
          if (!characters.value.some((character) => character.id === characterId.value)) { characterId.value = ""; modelError.value = "当前角色已失效，请重新选择角色"; }
        }, (error) => {
          ownedDataRefreshPending = true;
          reportOwnedLoadError(error);
        }, () => ownedDisposed);
      } finally { ownedPolling = false; }
    }, 3000);
  };
  let available = false;
  try { available = await owned.refresh(); }
  catch (error) {
    if (ownedDisposed) return;
    if (!(error instanceof OwnedChatStateChangedError) && !owned.notice.value.includes("等待新服务批准")) owned.markUnavailable();
    reportOwnedLoadError(error);
    startOwnedPolling();
    return;
  }
  if (ownedDisposed) return;
  if (available) {
    try { await initializeOwned(); }
    catch (error) { if (!ownedDisposed) reportOwnedLoadError(error); }
    finally { startOwnedPolling(); }
    return;
  }
  reportOwnedLoadError(new Error(owned.notice.value || "设备对话服务尚未就绪"));
  startOwnedPolling();
});

onUnmounted(() => {
  chatLayoutObserver?.disconnect();
  chatLayoutObserver = null;
  window.removeEventListener("amitia:execution-scope-changed", handleScopeChanged);
  ownedDisposed = true;
  if (ownedPolicyTimer) clearInterval(ownedPolicyTimer);
  ownedPolicyTimer = null;
  stopCallWindowListener?.();
  stopCallWindowListener = null;
  cleanupSSE();
  window.removeEventListener("resize", updateViewport);
});

function handleScopeChanged() {
  disconnectSSE();
  persistedMessages.value = persistedMessages.value.map((message) => ["streaming", "sending", "queued"].includes(message.status) ? { ...message, status: "interrupted" } : message);
  sending.value = false;
  llmModels.value = [];
  replyTarget.value = null;
  void handleEndCall();
}
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
.chat-surface { position: relative; display: flex; flex-direction: column; width: 100%; height: 100%; min-height: 0; overflow: hidden; border-radius: var(--radius-lg); background: var(--chat-surface-bg); }
.chat-header-region { position: absolute; top: 0; left: 0; right: 0; z-index: 4; }
.chat-status-region { position: absolute; top: var(--chat-header-h, 44px); left: 0; right: 0; z-index: 2; display: flex; flex-direction: column; gap: 6px; padding: 0 14px; }
.composer-region { position: absolute; left: 0; right: 0; bottom: 0; z-index: 3; }
.chat-surface--sidebar .composer-region { right: min(320px, 32%); }
.chat-scroll-fade { position: absolute; left: 0; right: 0; z-index: 2; pointer-events: none; }
.chat-scroll-fade--top { top: 0; height: var(--chat-header-h, 44px); background: linear-gradient(to bottom, color-mix(in srgb, var(--chat-surface-bg) 96%, transparent), color-mix(in srgb, var(--chat-surface-bg) 0%, transparent)); }
.chat-scroll-fade--bottom { bottom: 0; height: var(--chat-composer-h, 120px); background: linear-gradient(to top, color-mix(in srgb, var(--chat-surface-bg) 96%, transparent), color-mix(in srgb, var(--chat-surface-bg) 0%, transparent)); }
.chat-surface--sidebar .chat-scroll-fade { right: min(320px, 32%); }
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
