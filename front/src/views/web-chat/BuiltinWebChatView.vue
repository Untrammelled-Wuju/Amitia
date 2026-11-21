<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <ExtensionSlot
    slot-id="chat.conversation.root"
    :context="chatExtensionContext"
    fallback="default"
    layout="stack"
    surface-role="main"
    bare
    render-fallback-while-scope-pending
  >
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
    <ExtensionSlot
      slot-id="chat.conversation.session"
      :context="chatExtensionContext"
      fallback="default"
      layout="stack"
      surface-role="main"
      bare
      render-fallback-while-scope-pending
    >
    <section ref="chatSurfaceRef" class="chat-surface" :class="{ 'chat-surface--sidebar': conversationSidebarVisible, 'chat-surface--embedded': props.embedded }" data-amitia-ui="chat.surface">
    <AutomationStatusIndicator :status="automationStatus" />

    <Teleport v-if="titlebarTeleportReady" :to="titlebarTeleportTarget" :disabled="!useTitlebarChrome">
    <div ref="chatHeaderRef" class="chat-header-region" :class="{ 'is-titlebar': useTitlebarChrome }">
    <ExtensionSlot
      slot-id="chat.conversation.header"
      :context="chatExtensionContext"
      fallback="default"
      layout="inline"
      surface-role="header"
      bare
      render-fallback-while-scope-pending
    >
    <ExtensionSlot
      slot-id="chat.conversation.session.header"
      :context="chatExtensionContext"
      fallback="default"
      layout="inline"
      surface-role="header"
      bare
      render-fallback-while-scope-pending
    >
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
      :is-group="Boolean(group)"
      :group-members="group?.members?.filter((member) => member.status === 'active') || []"
      @clear="handleClear"
      @view-memories="handleViewMemories"
      @toggle-char-picker="showCharPicker = true"
      @toggle-profiles="toggleProfiles"
      @toggle-mem-inject="toggleMemInject"
      @toggle-call="handleEndCall"
      @start-call="handleStartCall"
      @view-summary="handleViewSummary"
      @manage-group="showGroupMembers = true"
    ><template #extension-actions><button type="button" class="sidebar-toggle-btn" aria-label="切换工作区" title="工作区" @click="workbench.toggleVisible()"><el-icon><GridIcon /></el-icon></button><button v-if="hasConversationSidebar && isSmallViewport" type="button" class="sidebar-toggle-btn" aria-label="展开插件侧栏" @click="sidebarDrawerOpen = true"><el-icon><MenuIcon /></el-icon></button><ChatHeaderExtensionHost :context="chatExtensionContext" /></template></UIProviderHost>
    </ExtensionSlot>
    <ExtensionSlot
      slot-id="chat.conversation.subagent.footer"
      :context="chatExtensionContext"
      fallback="none"
      layout="stack"
      surface-role="header"
      render-fallback-while-scope-pending
    />
    </ExtensionSlot>
    </div>
    </Teleport>
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
      <div class="conversation-messages-region" :aria-busy="conversationSwitching">
        <ExtensionSlot
          slot-id="chat.conversation.views"
          :context="chatExtensionContext"
          fallback="none"
          layout="tabs"
          surface-role="main"
          render-fallback-while-scope-pending
        />
        <UIProviderHost
          capability="conversation.messages"
          :fallback="MessagesArea"
          :context="chatExtensionContext"
          :actions="conversationHostActions"
          ref="msgAreaRef"
          :messages="messages"
          :is-group="Boolean(group)"
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
          :no-characters="!characters.length"
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
          @content-loading="handleConversationContentLoading"
          @content-ready="handleConversationContentReady"
        />
        <div v-if="conversationSwitching" class="conversation-switch-loading" role="status" aria-live="polite">
          <div class="conversation-switch-loading__content">
            <el-icon class="is-loading conversation-switch-loading__spinner" :size="24"><LoadingIcon /></el-icon>
            <span>正在加载对话</span>
          </div>
        </div>
      </div>
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
    <ExtensionSlot
      slot-id="chat.composer.root"
      :context="chatExtensionContext"
      fallback="default"
      layout="stack"
      surface-role="composer"
      bare
      render-fallback-while-scope-pending
    >
    <div ref="composerRegionRef" class="composer-region" data-amitia-ui="chat.composer">
      <QueuedMessages :items="queuedMessages" :busy="queueBusy" :can-steer="sending || isSubmitting" :error="queueError"
        :editing-id="editingQueue?.requestId || ''"
        @reorder="ids => runQueueAction(() => queue.reorder(ids))"
        @remove="id => runQueueAction(() => queue.remove(id))"
        @steer="id => runQueueAction(() => queue.steer(id))"
        @edit="beginQueueEdit"
        @open-side="openQueueInSideWorkspace" />
      <div v-if="group" class="group-composer-members" aria-label="群成员点名">
        <button v-for="member in activeGroupMembers" :key="member.memberId" type="button" @click="insertGroupMention(member)">@{{ member.characterName }}</button>
        <button type="button" class="mention-all" @click="insertAllGroupMentions">@所有成员</button>
        <button
          type="button"
          class="discussion-mode"
          :class="{ 'is-active': groupDiscussion }"
          :aria-pressed="groupDiscussion"
          :disabled="activeGroupMembers.length < 2 || !group.groupSettings.allowCharacterMentions"
          :title="!group.groupSettings.allowCharacterMentions ? '请先在群成员设置中开启角色互相 @' : '让多个角色按顺序进行多轮讨论'"
          @click="groupDiscussion = !groupDiscussion"
        >{{ groupDiscussion ? "讨论模式已开启" : "多角色讨论" }}</button>
      </div>
      <div v-if="editingQueue" class="queue-edit-bar" role="status" data-amitia-ui="chat.queue.edit">
        <span class="queue-edit-label">正在编辑排队消息</span>
        <span v-for="(attachment, index) in editingAttachments" :key="`${attachment.artifactId}-${index}`" class="queue-edit-chip">
          <span class="queue-edit-chip-name">{{ attachment.name }}</span>
          <button type="button" :aria-label="`移除附件 ${attachment.name}`" title="移除附件" @click="removeEditingAttachment(index)">×</button>
        </span>
        <button type="button" class="queue-edit-cancel" @click="cancelQueueEdit()">取消编辑</button>
      </div>
      <UIProviderHost
      capability="conversation.composer"
      :fallback="ChatInput"
      :context="chatExtensionContext"
      :actions="conversationHostActions"
      ref="inputRef"
      :disabled="Boolean(sendGuardReason)"
      :disabled-reason="sendGuardReason"
      :sending="sending"
      :generating="generating"
      :is-submitting="isSubmitting"
      :reply-target="replyTarget"
      :character-id="characterId"
      :conversation-id="convId"
      :is-group="Boolean(group)"
      :group-members="activeGroupMembers"
      :role-model-configs="roleModelConfigs"
      :channel="chatExtensionContext.channel"
      :models="llmModels"
      :selected-model-id="selectedModelId"
      :reasoning-effort="selectedReasoningEffort"
      :supports-reasoning="selectedModelSupportsReasoning"
      :reasoning-enabled="selectedReasoningEnabled"
      :permission-mode="selectedPermissionMode"
      :model-preview-change="handleModelSettingPreviewChange"
      :model-commit-change="handleModelSettingChange"
      :role-model-preview-change="handleRoleModelSettingPreviewChange"
      :role-model-commit-change="handleRoleModelSettingChange"
      @send="handleQueuedOrSend"
      @sendFiles="handleSendFiles"
      @stop="handleStop"
      @voiceAudio="handleVoiceAudio"
      @voiceText="handleVoiceText"
      @cancel-reply="replyTarget = null"
      @update:permission="handlePermissionModeChange"
    /></div>
    </ExtensionSlot>

    <CharacterPickerDialog
      v-model:visible="showCharPicker"
      :characters="characters"
      :character-id="characterId"
      @select="handleSwitchChar"
    />

    <GroupMemberPanel
      v-model:visible="showGroupMembers"
      :group="group"
      :characters="characters"
      :role-id="characterId"
      :conversation-id="convId"
      :reply-source-message-id="lastUserMessageId"
      @changed="handleGroupChanged"
      @mention="insertGroupMention"
      @replied="handleGroupReplied"
    />

    <DeviceOwnedMemoryPanel v-model:visible="showMemories" :character-id="characterId" :conversation-id="convId" />

    <el-drawer v-model="showSummaryDrawer" title="会话摘要" data-amitia-ui="component.drawer" direction="rtl" size="420px">
      <div v-if="convSummary" class="summary-drawer-text">{{ convSummary }}</div>
      <el-empty v-else description="暂无会话摘要" :image-size="88" />
    </el-drawer>

    </section>
    </ExtensionSlot>
  </div>
  </ExtensionSlot>
</template>
<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick, watch, inject } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import { Grid as GridIcon, Loading as LoadingIcon, Menu as MenuIcon } from "@element-plus/icons-vue";
import { useApi } from "../../composables/useApi";
import { useCachedApi } from "../../composables/useCachedApi";
import { useChatStore } from "@/stores/chat";
import { useWorkbenchStore } from "@/stores/workbench";
import { useConversationRuntime } from "../../composables/useConversationRuntime";
import AutomationStatusIndicator from "../../components/AutomationStatusIndicator.vue";
import { useWebChatScroll } from "../../composables/useWebChatScroll";
import { ownedQuoteReferenceForMessage, useWebChatSend } from "../../composables/useWebChatSend";
import { useConversationQueue, type QueuedMessage } from "../../composables/useConversationQueue";
import { resolveGroupMentions } from "../../composables/groupMentions";
import { uploadOwnedAttachment, ownedAttachmentKindOf, type OwnedAttachment } from "@/runtime/device-owned-attachments";
import QueuedMessages from "../../components/QueuedMessages.vue";
import { useWebChatConversation } from "../../composables/useWebChatConversation";
import { useDeviceOwnedConversation } from "../../composables/useDeviceOwnedConversation";
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
import { conversationReference, parseConversationReference } from "../../runtime/device-owned-conversation-reference";
import { prepareOwnedChatRequest } from "../../runtime/owned-chat-management";
import MessagesArea from "../../components/MessagesArea.vue";
import ChatInput from "../../components/ChatInput.vue";
import CharacterPickerDialog from "../../components/CharacterPickerDialog.vue";
import DeviceOwnedMemoryPanel from "../../components/DeviceOwnedMemoryPanel.vue";
import GroupMemberPanel from "../../components/GroupMemberPanel.vue";
import ProfileSummaryPanel from "./components/ProfileSummaryPanel.vue";
import MemoryInjectPanel from "./components/MemoryInjectPanel.vue";
import { chatSendDisabledReason } from "./chatSendGuard";
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
import { roleAuthorityConfig } from "@/runtime/role-authority";
import { hasUnifiedSlotItem } from "@/ui-runtime/slotLedger";
import type { OwnedGroupMember, OwnedGroupState } from "@/runtime/device-owned-chat";

const router = useRouter();
const route = useRoute();
const props = withDefaults(defineProps<{
  embedded?: boolean;
  initialConversationId?: string;
  initialCharacterId?: string;
  initialDraftText?: string;
}>(), { embedded: false, initialConversationId: "", initialCharacterId: "", initialDraftText: "" });
const emit = defineEmits<{ conversationCreated: [conversationId: string] }>();
const callActive = ref(false);
const callMode = ref<"voice" | "video" | "screen">("voice");
const boundCallScope = ref<OwnedExecutionScope | null>(null);
const boundCallConversation = ref("");
const boundCallHistoricalRole = ref<string | undefined>();
const ttsVoiceType = ref("");
const ttsResourceId = ref("");

async function handleStartCall(mode: "voice" | "video" | "screen") {
  callMode.value = mode;
  try {
    if (!(await owned.refresh())) throw new Error("设备对话服务未就绪，无法发起通话");
    const originalRole = characterId.value;
    const originalConversation = convId.value;
    if (!originalRole) throw new Error("请先选择一个已保存的角色，再发起通话");
    const snapshot = originalConversation ? await owned.query(originalConversation, originalRole) : await owned.data("working", originalRole);
    if (characterId.value !== originalRole || convId.value !== originalConversation || !snapshot?.executionScope || snapshot.executionScope.roleId !== originalRole) throw new Error("通话角色或会话已变化，请重新发起");
    boundCallScope.value = JSON.parse(JSON.stringify(snapshot.executionScope));
    boundCallConversation.value = originalConversation;
    const historicalConversation = snapshot.historicalSnapshot?.resources.find((item) => item.kind === "conversation");
    boundCallHistoricalRole.value = historicalConversation?.roleId || undefined;
    callActive.value = true;
  } catch (error) {
    callActive.value = false;
    boundCallScope.value = null;
    ElMessage.error(error instanceof Error ? error.message : "无法确认通话服务");
  }
}

async function handleEndCall() {
  callActive.value = false;
  boundCallScope.value = null;
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
let ownedPolling = false;
let ownedDisposed = false;
let refreshOwnedRuntimeHandler: (() => Promise<void>) | undefined;
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
const convId = ref(props.embedded ? props.initialConversationId : String(route.query.conversationId || ""));
const convTitle = ref("");
const characterId = ref(props.embedded ? props.initialCharacterId : "");
const showGroupMembers = ref(false);
const messageContentLoading = ref(false);
watch([convId, characterId], ([id, role]) => {
  if (props.embedded) return;
  workbench.setConversation(id, role);
}, { immediate: true });
const cachedDef = (() => {
  try {
    const v = localStorage.getItem("uai-default-char");
    return v ? JSON.parse(v) : null;
  } catch (error) {
  console.error("[original-error]", error);
return null;
  }
})();
const charName = ref(cachedDef?.name || "");
const charIdentity = ref(cachedDef?.identity || "");
const charAvatar = ref("");

const sending = ref(false);
const modelMissing = ref(false);
const sendGuardReason = computed(() => chatSendDisabledReason(modelMissing.value, characters.value.length));
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
const roleAuthorityToken = ref("");
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
    conversationLoading: conversationLoading.value,
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
const useTitlebarChrome = ref(!props.embedded && window.innerWidth >= 900);
const titlebarTargetElement = ref<HTMLElement | null>(null);
const titlebarTeleportTarget = computed<string | HTMLElement>(() => titlebarTargetElement.value || "body");
const titlebarTeleportReady = computed(() => !useTitlebarChrome.value || Boolean(titlebarTargetElement.value));
let titlebarTargetObserver: MutationObserver | null = null;

function syncTitlebarTarget() {
  const target = document.querySelector<HTMLElement>("#window-chat-slot");
  if (target) titlebarTargetElement.value = target;
  if (target || !useTitlebarChrome.value) {
    titlebarTargetObserver?.disconnect();
    titlebarTargetObserver = null;
  }
}

function startTitlebarTargetObserver() {
  titlebarTargetObserver?.disconnect();
  titlebarTargetObserver = null;
  syncTitlebarTarget();
  if (!useTitlebarChrome.value || titlebarTargetElement.value || typeof MutationObserver === "undefined") return;
  titlebarTargetObserver = new MutationObserver(syncTitlebarTarget);
  titlebarTargetObserver.observe(document.body, { childList: true, subtree: true });
}

function updateViewport() {
  isSmallViewport.value = window.innerWidth < 1024;
  useTitlebarChrome.value = !props.embedded && window.innerWidth >= 900;
}

const chatSurfaceRef = ref<HTMLElement>();
const chatHeaderRef = ref<HTMLElement>();
const composerRegionRef = ref<HTMLElement>();
const conversationSidebarVisible = computed(() => hasConversationSidebar.value && !isSmallViewport.value && !workbench.visible);
let chatLayoutObserver: ResizeObserver | null = null;

function syncChatLayoutVars() {
  const surface = chatSurfaceRef.value;
  if (!surface) return;
  const headerHeight = useTitlebarChrome.value ? 0 : (chatHeaderRef.value?.getBoundingClientRect().height ?? 0);
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

watch(useTitlebarChrome, async () => {
  titlebarTargetElement.value = null;
  await nextTick();
  startTitlebarTargetObserver();
  chatLayoutObserver?.disconnect();
  chatLayoutObserver = null;
  startChatLayoutObserver();
});

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
      resetConversationScope();
      convId.value = providedId;
      await router.replace({ path: "/chat", query: { conversationId: providedId } });
      nextTick(() => scrollToBottom(true));
      connectSSE();
      return;
    }
    resetForNewDraft();
    resetConversationScope();
    convId.value = "";
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
  if (editingQueue.value) {
    try {
      const uploaded = await uploadOwnedAttachment({ file, kind: "file", name: file.name });
      editingAttachments.value = [...editingAttachments.value, uploaded];
    } catch (cause) {
      await ElMessageBox.alert(queue.reportError(cause), "附件上传失败", { confirmButtonText: "关闭" });
    }
    return;
  }
  if (sending.value || isSubmitting.value) {
    try { await enqueueQueuedMessage(`[文件] ${file.name}`, [await uploadOwnedAttachment({ file, kind: "file", name: file.name })]); }
    catch (cause) { await ElMessageBox.alert(queue.reportError(cause), "文件加入队列失败", { confirmButtonText: "关闭" }); }
    return;
  }
  try { if (await owned.refresh()) { if (!characterId.value) { ElMessage.warning("请先选择调用角色"); return; } await handleOwnedFileSend(file); return; } } catch (error: any) { ElMessage.error(error?.message || "文件发送失败"); return; }
  ElMessage.error(owned.notice.value || "设备对话服务尚未就绪，附件未发送");
}

async function handleSendFiles(text: string, files: File[]) {
  if (!files.length) {
    if (text.trim()) await handleQueuedOrSend(text);
    return;
  }
  if (editingQueue.value) {
    try {
      const uploaded = await Promise.all(files.map((file) => uploadOwnedAttachment({ file, kind: ownedAttachmentKindOf(file), name: file.name })));
      editingAttachments.value = [...editingAttachments.value, ...uploaded];
    } catch (cause) {
      await ElMessageBox.alert(queue.reportError(cause), "附件上传失败", { confirmButtonText: "关闭" });
    }
    return;
  }
  await handleSendWithFiles(text, files);
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
  } catch (error) {
  console.error("[original-error]", error);
llmModels.value = [];
  }
}

async function loadRoleAuthority() {
  try {
    const authority = await get<{ roleAuthority: string }>("/api/characters/authority");
    roleAuthorityToken.value = String(authority?.roleAuthority || "");
  } catch (error) {
    console.error("[original-error]", error);
    roleAuthorityToken.value = "";
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
  } catch (error) {
  console.error("[original-error]", error);
}
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
  } catch (error) {
  console.error("[original-error]", error);
}
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
  } catch (error) {
  console.error("[original-error]", error);
await reloadConversationSnapshot();
    ElMessage.error(error instanceof Error ? error.message : String(error));
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
  } catch (error) {
  console.error("[original-error]", error);
await reloadConversationSnapshot();
    ElMessage.error(error instanceof Error ? error.message : String(error));
  }
}

function handleRoleModelSettingPreviewChange(
  roleId: string,
  modelId: number,
  reasoningEffort: string,
  reasoningEnabled: boolean,
) {
  if (roleId !== characterId.value) return;
  handleModelSettingPreviewChange(modelId, reasoningEffort, reasoningEnabled);
}

async function handleRoleModelSettingChange(
  roleId: string,
  modelId: number,
  reasoningEffort: string,
  reasoningEnabled: boolean,
) {
  const role = characters.value.find((item: any) => item.id === roleId);
  if (!role) {
    ElMessage.error("角色已变化，请重新选择后设置模型");
    return;
  }
  try {
    if (!role.roleAuthority && !roleAuthorityToken.value) {
      await loadRoleAuthority();
    }
    const updated = await put<any>(
      `/api/characters/${encodeURIComponent(roleId)}`,
      {
        modelConfigId: Number(modelId || 0),
        reasoningEffort: reasoningEffort || "",
        reasoningEnabled: reasoningEnabled ? 1 : 0,
      },
      roleAuthorityConfig(role.roleAuthority || roleAuthorityToken.value),
    );
    Object.assign(role, updated || {}, {
      modelConfigId: Number(updated?.modelConfigId ?? modelId ?? 0),
      reasoningEffort: String(updated?.reasoningEffort ?? reasoningEffort ?? ""),
      reasoningEnabled: Number(updated?.reasoningEnabled ?? (reasoningEnabled ? 1 : 0)),
    });
    if (roleId === characterId.value) {
      handleModelSettingPreviewChange(modelId, reasoningEffort, reasoningEnabled);
    }
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : String(error));
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

const groupSending = ref(false);
const groupDiscussion = ref(false);
const groupConversation = ref<OwnedGroupState | null>(null);

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
      await chatStore.markConversationRead(String(conversation.id), Number(conversation.lastTerminalTurnSequence || 0), characterId.value).catch((error) => {
  console.error("[original-error]", error);
  return undefined;
});
    }
  },
  async () => {
    await chatStore.fetchSidebar();
  },
  groupSending,
  characterId,
);

loadOlderRuntimeTurns = loadOlderConversationTurns;

function resetConversationMessages() {
  clearRuntime();
  persistedMessages.value = [];
}

function disconnectAndResetConversation() {
  disconnectSSE();
  resetConversationMessages();
  sending.value = false;
  isSubmitting.value = false;
  if (groupSending.value) groupSending.value = false;
  void chatStore.fetchSidebar().catch((error) => {
  console.error("[original-error]", error);
  return undefined;
});
}

function resetConversationScope() {
  disconnectAndResetConversation();
  convTitle.value = "";
  replyTarget.value = null;
  convSummary.value = "";
  showSummaryDrawer.value = false;
  showGroupMembers.value = false;
  groupConversation.value = null;
  groupDiscussion.value = false;
  messageContentLoading.value = false;
  clearGroupConversation();
}

function handleSidebarDraftCreated() {
  if (props.embedded) return;
  resetForNewDraft();
  resetConversationScope();
  convId.value = "";
  modelError.value = "";
  selectedPermissionMode.value = loadChatPermissionMode();
  nextTick(() => inputRef.value?.focus());
}

const {
  onImageRemoved,
  onVideoRemoved,
  handleVoiceAudio,
  handleVoiceText,
  handleFileSend: handleOwnedFileSend,
  handleImageSend,
  handleSend,
  handleSendWithFiles,
  imageAttachmentInput,
  handleStop,
  handleRetry,
  handleClear,
  resetForNewDraft,
  getPendingVideoFile,
  generating,
  isSubmitting,
  messageStyle,
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
    if (convId.value !== conversationId) return;
    if (props.embedded) {
      emit("conversationCreated", conversationId);
      if (convId.value !== conversationId) return;
      void connectSSE(false);
      return;
    }
    await router.replace({
      path: "/chat",
      query: { conversationId },
    });
    if (convId.value !== conversationId) return;
    void connectSSE(false);
    await fetchConversations();
    await chatStore.fetchSidebar();
    syncConversationTitleFromSidebar(conversationId);
  },
  selectedModelId,
  selectedReasoningEffort,
  selectedReasoningEnabled,
  selectedPermissionMode,
  activeTurnId,
  beginPendingAssistant,
  failPendingAssistant,
  groupSending,
  groupDiscussion,
  groupConversation,
  async (text, attachments) => enqueueQueuedMessage(text, attachments),
);

const queue = useConversationQueue(convId, characterId, activeTurnId);
const { items: queuedMessages, busy: queueBusy, error: queueError } = queue;

const editingQueue = ref<{ requestId: string; message: string } | null>(null);
const editingAttachments = ref<OwnedAttachment[]>([]);

function toOwnedAttachments(items: QueuedMessage["attachments"]): OwnedAttachment[] {
  return (items || [])
    .filter((item) => item.artifactId && item.ownerDeviceId && item.storageDeviceId && item.sha256 && typeof item.sizeBytes === "number")
    .map((item) => ({
      artifactId: item.artifactId as string,
      ownerDeviceId: item.ownerDeviceId as string,
      storageDeviceId: item.storageDeviceId as string,
      kind: item.kind as OwnedAttachment["kind"],
      name: item.name,
      mimeType: item.mimeType || "application/octet-stream",
      sha256: item.sha256 as string,
      sizeBytes: item.sizeBytes as number,
    }));
}

function beginQueueEdit(requestId: string) {
  const item = queuedMessages.value.find((row) => row.requestId === requestId);
  if (!item) return;
  if (item.status !== "queued") {
    ElMessage.warning("该排队消息已开始执行，无法编辑");
    return;
  }
  editingQueue.value = { requestId, message: item.message };
  editingAttachments.value = toOwnedAttachments(item.attachments);
  inputRef.value?.setText?.(item.message);
  inputRef.value?.focus?.();
}

function cancelQueueEdit(clearComposer = true) {
  if (clearComposer) inputRef.value?.clear?.();
  editingQueue.value = null;
  editingAttachments.value = [];
}

function removeEditingAttachment(index: number) {
  editingAttachments.value = editingAttachments.value.filter((_, position) => position !== index);
}

watch(queuedMessages, (items) => {
  const editing = editingQueue.value;
  if (!editing) return;
  const current = items.find((item) => item.requestId === editing.requestId);
  if (!current || current.status !== "queued") {
    cancelQueueEdit(false);
    ElMessage.info("该排队消息已开始执行，已退出编辑");
  }
});

async function runQueueAction(action: () => Promise<void>) {
  try {
    await action();
  } catch (cause) {
    const detail = queue.reportError(cause);
    await ElMessageBox.alert(detail, "排队消息操作失败", { confirmButtonText: "关闭" });
  }
}

async function handleQueuedOrSend(text: string, imageBase64?: string, videoBase64?: string) {
  if (editingQueue.value) return submitQueueEdit(text, imageBase64, videoBase64);
  if (!sending.value && !isSubmitting.value) return handleSend(text, imageBase64, videoBase64);
  const id = convId.value;
  const videoFile = getPendingVideoFile();
  try {
    const attachments: OwnedAttachment[] = [];
    if (imageBase64) attachments.push(await uploadOwnedAttachment(await imageAttachmentInput(imageBase64)));
    if (videoBase64) {
      if (!videoFile) throw new Error("视频文件已变化，请重新选择视频");
      attachments.push(await uploadOwnedAttachment({ file: videoFile, kind: "video", name: videoFile.name }));
    }
    await enqueueQueuedMessage(text.trim() || (imageBase64 ? "[图片]" : "[视频]"), attachments);
    if (imageBase64) onImageRemoved();
    if (videoBase64) onVideoRemoved();
  } catch (cause) {
    if (convId.value === id && !inputRef.value?.getText?.()) inputRef.value?.setText?.(text);
    const detail = queue.reportError(cause);
    await ElMessageBox.alert(`${detail}\n\n未入队的消息：\n${text}`, "加入队列失败", { confirmButtonText: "关闭" });
  }
}

async function submitQueueEdit(text: string, imageBase64?: string, videoBase64?: string) {
  const editing = editingQueue.value;
  if (!editing) return;
  const conversationId = convId.value;
  const attachments = [...editingAttachments.value];
  try {
    if (imageBase64) attachments.push(await uploadOwnedAttachment(await imageAttachmentInput(imageBase64)));
    if (videoBase64) {
      const videoFile = getPendingVideoFile();
      if (!videoFile) throw new Error("视频文件已变化，请重新选择视频");
      attachments.push(await uploadOwnedAttachment({ file: videoFile, kind: "video", name: videoFile.name }));
    }
    const content = text.trim() || (attachments.length ? `[${attachments.map((attachment) => attachment.name).join("、")}]` : "");
    if (!content) throw new Error("消息内容为空");
    await queue.update(editing.requestId, content, attachments);
    if (convId.value !== conversationId) return;
    editingQueue.value = null;
    editingAttachments.value = [];
    inputRef.value?.clear?.();
    if (imageBase64) onImageRemoved();
    if (videoBase64) onVideoRemoved();
  } catch (cause) {
    if (convId.value === conversationId && !inputRef.value?.getText?.()) inputRef.value?.setText?.(text);
    const detail = queue.reportError(cause);
    await ElMessageBox.alert(detail, "更新排队消息失败", { confirmButtonText: "关闭" });
  }
}

function openQueueInSideWorkspace(requestId: string) {
  const item = queuedMessages.value.find((row) => row.requestId === requestId);
  if (!item) return;
  workbench.openSideChat({
    queueRequestId: item.requestId,
    message: item.message,
    attachments: item.attachments || [],
    sourceConversationId: convId.value,
    roleId: characterId.value,
  });
}

async function enqueueQueuedMessage(text: string, attachments: OwnedAttachment[] = []) {
  const conversationId = convId.value;
  const roleId = characterId.value;
  if (!conversationId || !roleId) throw new Error("当前会话或角色不可用");
  const prepared = await prepareOwnedChatRequest({ conversationId }, roleId);
  if (convId.value !== conversationId || characterId.value !== roleId) throw new Error("会话或角色已变化，排队消息未提交");
  const quote = replyTarget.value ? await ownedQuoteReferenceForMessage(replyTarget.value, prepared.expected) : undefined;
  const previousCore = [...persistedMessages.value].reverse().find((message) => message.executionScope?.coreId && message.executionScope.coreId !== owned.coreId.value)?.executionScope?.coreId;
  const context = previousCore ? {
    previousCoreId: previousCore,
    conversationId,
    summary: owned.previousSummary(previousCore, conversationId),
    messages: persistedMessages.value.filter((message) => ["user", "assistant"].includes(message.role) && !["sending", "queued", "failed"].includes(message.status)).slice(-128).map((message) => ({ id: message.id, ownerId: message.ownerId || message.executionScope?.resourceOwnerId, role: message.role, content: String(message.content || ""), status: message.status })),
  } : undefined;
  const origin = parseConversationReference(conversationId);
  const currentGroup = group.value;
  await queue.enqueue(text, {
    modelConfigId: selectedModelId.value,
    reasoningEffort: selectedReasoningEffort.value,
    reasoningEnabled: selectedReasoningEnabled.value,
    permissionMode: selectedPermissionMode.value,
    messageStyle: messageStyle.value,
    ...(quote ? { quote } : {}),
    ...(context ? { context } : {}),
    ...(origin ? { conversationOrigin: origin } : {}),
    ...(replyTarget.value?.content ? { replyExcerpt: String(replyTarget.value.content).slice(0, 512) } : {}),
    ...(attachments.length ? { attachments } : {}),
    ...(currentGroup ? {
      mentions: groupDiscussion.value ? [] : resolveGroupMentions(text, currentGroup.members),
      group: { expectedMembershipRevision: currentGroup.membershipRevision, expectedSettingsRevision: currentGroup.settingsRevision, mode: groupDiscussion.value ? "discussion" : "standard" },
    } : {}),
  });
}

const {
  characters,
  conversations,
  group,
  memories,
  conversationLoading,
  showCharPicker,
  showMemories,
  selectCharacter,
  clearGroupConversation,
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
  reloadConversationSnapshot,
);
watch(group, (value) => { groupConversation.value = value; }, { immediate: true, flush: "sync" });
const activeGroupMembers = computed<OwnedGroupMember[]>(() => (group.value?.members || []).filter((member) => member.status === "active").sort((left, right) => left.orderIndex - right.orderIndex));
const roleModelConfigs = computed(() => {
  const members = activeGroupMembers.value;
  const roleIDs = members.length > 0
    ? members.map((member) => member.characterId)
    : [characterId.value].filter(Boolean);
  return roleIDs.map((roleID) => {
    const member = members.find((item) => item.characterId === roleID);
    const role = characters.value.find((item: any) => item.id === roleID);
    return {
      roleId: roleID,
      memberId: member?.memberId || "",
      name: member?.characterName || role?.name || roleID,
      avatar: member?.characterAvatar || role?.avatar || "",
      modelConfigId: Number(role?.modelConfigId || 0),
      reasoningEffort: String(role?.reasoningEffort || ""),
      reasoningEnabled: Number(role?.reasoningEnabled ?? -1),
    };
  });
});
const lastUserMessageId = computed(() => [...persistedMessages.value].reverse().find((message) => message.role === "user" && message.id && !String(message.id).startsWith("client:"))?.id || "");

function syncConversationTitleFromSidebar(conversationId: string) {
  const id = String(conversationId || "").trim();
  if (!id || id !== String(convId.value || "")) return;
  const rows = [
    ...chatStore.sidebar.pinned,
    ...chatStore.sidebar.recent,
    ...chatStore.sidebar.projects.flatMap((project) => project.conversations),
  ];
  const title = String(rows.find((conversation) => conversation.id === id)?.title || "").trim();
  if (!title) return;
  convTitle.value = title;
  if (group.value) group.value = { ...group.value, title };
}

watch(() => chatStore.sidebar, () => syncConversationTitleFromSidebar(convId.value));

const conversationSwitching = computed(() => conversationLoading.value || messageContentLoading.value);

function handleConversationContentLoading(id: string) {
  if (String(id || "") === String(convId.value || "")) messageContentLoading.value = true;
}

function handleConversationContentReady(id: string) {
  if (String(id || "") === String(convId.value || "")) messageContentLoading.value = false;
}

watch(() => chatStore.currentConversationId, (id) => {
  const target = String(id || "").trim();
  if (target && target !== String(convId.value || "")) conversationLoading.value = true;
});

const conversationHostActions: Record<string, (input?: any) => unknown | Promise<unknown>> = {
  "conversation.send": async (input) => handleQueuedOrSend(String(input?.text ?? input ?? ""), input?.imageBase64, input?.videoBase64),
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
  [convId, characterId],
  ([conversationId, roleId]) => {
    const id = conversationId || "";
    void loadConversationWorkspace(id, id ? "" : String(route.query.projectId || "")).catch((error) => {
      if ((convId.value || "") === id) modelError.value = error instanceof Error ? error.message : "聊天数据加载失败，请重试";
    });
    if (!id || !roleId) return;
    const requestedId = id;
    const requestedRole = roleId;
    void fetchConvSummary(requestedId).then((summary) => {
      if ((convId.value || "") === requestedId && characterId.value === requestedRole) {
        convSummary.value = summary;
      }
    }).catch((error) => {
  console.error("[original-error]", error);
  return undefined;
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
    if (props.embedded) return;
    if (nextOwner) {
      try { await owned.refresh(); } catch (error: any) { modelError.value = error?.message || '无法确认通知执行核心'; return; }
      if (owned.coreId.value !== nextOwner) {
        resetConversationScope();
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
      resetConversationScope();
      convId.value = "";
      selectedPermissionMode.value = loadChatPermissionMode();
      await loadConversationWorkspace("", String(route.query.projectId || ""));
      return;
    }
    if (nextId === convId.value && !roleChanged) return;
    convId.value = nextId;
    try {
      await loadCharacterConversation({ retryOnFailure: true });
    } catch (error) {
      if ((convId.value || "") === nextId) modelError.value = error instanceof Error ? error.message : "聊天数据加载失败，请重试";
    }
  },
);

watch(
  () => String(route.query.projectId || ""),
  async (projectId) => {
    if (props.embedded || convId.value) return;
    await loadConversationWorkspace("", projectId);
  },
);

watch(
  () => props.initialConversationId,
  async (nextId) => {
    if (!props.embedded || !nextId || nextId === convId.value) return;
    convId.value = nextId;
    try {
      await loadCharacterConversation({ retryOnFailure: true });
    } catch (error) {
      if (convId.value === nextId) modelError.value = error instanceof Error ? error.message : "聊天数据加载失败，请重试";
    }
  },
);

watch(showSummaryDrawer, (visible) => {
  if (!visible) return;
  const requestedId = convId.value || "";
  void fetchConvSummary(requestedId).then((summary) => {
    if ((convId.value || "") === requestedId) {
      convSummary.value = summary;
    }
    }).catch((error) => {
  console.error("[original-error]", error);
  return undefined;
});
});

function handleViewSummary() {
  showSummaryDrawer.value = true;
}

function insertGroupMention(member: OwnedGroupMember) {
  inputRef.value?.insertText?.(`@${member.characterName} `);
  inputRef.value?.focus?.();
}

function insertAllGroupMentions() {
  inputRef.value?.insertText?.("@所有成员 ");
  inputRef.value?.focus?.();
}

function handleGroupChanged(next: OwnedGroupState) {
  if (String(next.conversationId || "") !== String(convId.value || "")) return;
  group.value = next;
  convTitle.value = next.title || convTitle.value;
}

watch(convId, () => {
  groupDiscussion.value = false;
}, { flush: "sync" });
watch(
  () => [group.value?.groupSettings.allowCharacterMentions, activeGroupMembers.value.length] as const,
  ([allowed, memberCount]) => {
    if (!allowed || memberCount < 2) groupDiscussion.value = false;
  },
);

async function handleGroupReplied() {
  await loadCharacterConversation();
  await fetchConversations();
}

provideConversationUIContext({
  conversationId: convId,
  characterId,
  messages: readonlyMessages(messages),
  sending,
  generating,
  offline: isOffline,
  actions: {
    async send(input) { await handleQueuedOrSend(input.text ?? ""); },
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
  if (props.embedded) {
    await nextTick();
    if (!convId.value && props.initialDraftText) inputRef.value?.setText?.(props.initialDraftText);
    inputRef.value?.focus?.();
  }
  startTitlebarTargetObserver();
  startChatLayoutObserver();
  window.addEventListener("resize", updateViewport);
  window.addEventListener("amitia:execution-scope-changed", handleScopeChanged);
  window.addEventListener("amitia:draft-created", handleSidebarDraftCreated);
  void loadLlmModels();
  void loadRoleAuthority();
  let ownedInitialized = false;
  let ownedInitializationPromise: Promise<void> | null = null;
  let ownedDataRefreshPending = false;
  let ownedDataRefreshPromise: Promise<void> | null = null;
  let ownedDataRetryAt = 0;
  let ownedLoadError = "";
  const reportOwnedLoadError = (error: unknown) => {
    ownedLoadError = error instanceof Error ? error.message : "聊天数据加载失败，请重试";
    modelError.value = ownedLoadError;
  };
  const scheduleOwnedDataRetry = () => {
    ownedDataRefreshPending = true;
    ownedDataRetryAt = Date.now() + 15000;
  };
  const clearOwnedLoadError = () => {
    if (ownedLoadError && modelError.value === ownedLoadError) modelError.value = "";
    ownedLoadError = "";
  };
  const syncOwnedCharacters = () => {
    const current = characters.value;
    const next = owned.roles.value;
    const unchanged = current.length === next.length && next.every((role, index) => {
      const previous = current[index];
      return previous?.id === role.id
        && previous?.revision === role.revision
        && previous?.name === role.name
        && previous?.avatar === role.avatar
        && previous?.modelConfigId === role.modelConfigId
        && previous?.reasoningEffort === role.reasoningEffort
        && previous?.reasoningEnabled === role.reasoningEnabled;
    });
    if (!unchanged) characters.value = next;
  };
  const refreshOwnedData = async (preserve: boolean) => {
    if (ownedDataRefreshPromise) return ownedDataRefreshPromise;
    ownedDataRefreshPromise = (async () => {
      const tasks: Promise<unknown>[] = [chatStore.fetchSidebar()];
      if (convId.value) tasks.push(loadCharacterConversation({ preserve, retryOnFailure: true }));
      const results = await Promise.allSettled(tasks);
      const failure = results.find((result): result is PromiseRejectedResult => result.status === "rejected");
      if (failure) throw failure.reason;
      ownedDataRefreshPending = false;
      ownedDataRetryAt = 0;
      clearOwnedLoadError();
    })().finally(() => {
      ownedDataRefreshPromise = null;
    });
    return ownedDataRefreshPromise;
  };
  const initializeOwned = async () => {
    ownedInitialized = true;
    if (ownedInitializationPromise) return ownedInitializationPromise;
    ownedInitializationPromise = (async () => {
      syncOwnedCharacters();
      const notificationOwner = String(route.query.ownerId || '');
      if (notificationOwner && notificationOwner !== owned.coreId.value) {
        resetConversationScope();
        convId.value = '';
        modelError.value = '通知所属核心与当前核心不同，请先切换至原核心后打开';
        return;
      }
      const notificationRole = String(route.query.characterId || '');
      if (notificationRole && !characters.value.some((role) => role.id === notificationRole)) {
        resetConversationScope();
        convId.value = '';
        modelError.value = '通知所属角色不可用，无法打开会话';
        return;
      }
      const selectedId = owned.selectInitialRole(String(route.query.characterId || "") || localStorage.getItem(`webchat-char-id:${owned.coreId.value}:${owned.roleOwnerId.value}`) || "");
      const selected = characters.value.find((character) => character.id === selectedId);
      if (selected) selectCharacter(selected);
      else { characterId.value = ""; modelError.value = characters.value.length ? "请选择调用角色后发送消息" : "没有可用角色，拒绝调用"; }
      if (characterId.value) await refreshOwnedData(false);
      else clearOwnedLoadError();
    })().finally(() => {
      ownedInitializationPromise = null;
    });
    return ownedInitializationPromise;
  };
  const refreshOwnedRuntime = async () => {
    if (ownedDisposed || ownedPolling) return;
    ownedPolling = true;
    try {
      const previousProvider = `${owned.coreId.value}/${owned.policy.value?.providerEpoch}/${owned.policy.value?.modeRevision}/${owned.policy.value?.permissionRevision}`;
      await pollOwnedChat(owned, async () => {
        if (!ownedInitialized) { await initializeOwned(); return; }
        syncOwnedCharacters();
        const notificationOwner = String(route.query.ownerId || '');
        if (notificationOwner && notificationOwner !== owned.coreId.value) {
          resetConversationScope(); convId.value = '';
          modelError.value = '通知所属核心与当前核心不同，请先切换至原核心后打开';
          return;
        }
        const currentProvider = `${owned.coreId.value}/${owned.policy.value?.providerEpoch}/${owned.policy.value?.modeRevision}/${owned.policy.value?.permissionRevision}`;
        const shouldRefreshOwnedData = previousProvider !== currentProvider || (ownedDataRefreshPending && Date.now() >= ownedDataRetryAt);
        if (shouldRefreshOwnedData) {
          try {
            await refreshOwnedData(true);
          } catch (error) {
            scheduleOwnedDataRetry();
            reportOwnedLoadError(error);
          }
        }
        if (!characters.value.some((character) => character.id === characterId.value)) { characterId.value = ""; modelError.value = "当前角色已失效，请重新选择角色"; }
      }, (error) => {
        scheduleOwnedDataRetry();
        reportOwnedLoadError(error);
      }, () => ownedDisposed);
    } finally { ownedPolling = false; }
  };
  refreshOwnedRuntimeHandler = refreshOwnedRuntime;
  window.addEventListener("amitia:owned-runtime-refresh", refreshOwnedRuntime);
  window.addEventListener("amitia:runtime-connection-changed", refreshOwnedRuntime);
  window.addEventListener("amitia:execution-scope-changed", refreshOwnedRuntime);
  window.addEventListener("focus", refreshOwnedRuntime);
  window.addEventListener("online", refreshOwnedRuntime);
  document.addEventListener("visibilitychange", refreshOwnedRuntime);
  let available = false;
  try { available = await owned.refresh(); }
  catch (error) {
    if (ownedDisposed) return;
    if (!(error instanceof OwnedChatStateChangedError) && !owned.notice.value.includes("等待新服务批准")) owned.markUnavailable();
    scheduleOwnedDataRetry();
    reportOwnedLoadError(error);
    void refreshOwnedRuntime();
    return;
  }
  if (ownedDisposed) return;
  if (available) {
    try { await initializeOwned(); }
    catch (error) {
      if (!ownedDisposed) {
        scheduleOwnedDataRetry();
        reportOwnedLoadError(error);
      }
    }
    finally { void refreshOwnedRuntime(); }
    return;
  }
  reportOwnedLoadError(new Error(owned.notice.value || "设备对话服务尚未就绪"));
  void refreshOwnedRuntime();
});

onUnmounted(() => {
  titlebarTargetObserver?.disconnect();
  titlebarTargetObserver = null;
  chatLayoutObserver?.disconnect();
  chatLayoutObserver = null;
  window.removeEventListener("amitia:execution-scope-changed", handleScopeChanged);
  if (refreshOwnedRuntimeHandler) {
    window.removeEventListener("amitia:owned-runtime-refresh", refreshOwnedRuntimeHandler);
    window.removeEventListener("amitia:runtime-connection-changed", refreshOwnedRuntimeHandler);
    window.removeEventListener("amitia:execution-scope-changed", refreshOwnedRuntimeHandler);
    window.removeEventListener("focus", refreshOwnedRuntimeHandler);
    window.removeEventListener("online", refreshOwnedRuntimeHandler);
    document.removeEventListener("visibilitychange", refreshOwnedRuntimeHandler);
    refreshOwnedRuntimeHandler = undefined;
  }
  window.removeEventListener("amitia:draft-created", handleSidebarDraftCreated);
  ownedDisposed = true;
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
.chat-header-region.is-titlebar {
  position: relative;
  inset: auto;
  width: 100%;
  height: 100%;
  min-height: 0;
  display: flex;
  padding: 0 6px 0 12px;
  box-sizing: border-box;
}
.chat-header-region.is-titlebar :deep(.chat-header) {
  width: 100%;
  height: 100%;
  min-height: 0;
  padding: 0;
  gap: 8px;
}
.chat-header-region.is-titlebar :deep(.header-leading) { flex: 1; }
.chat-header-region.is-titlebar :deep(.header-actions),
.chat-header-region.is-titlebar :deep(.header-actions *) { -webkit-app-region: no-drag; }
.chat-status-region { position: absolute; top: var(--chat-header-h, 44px); left: 0; right: 0; z-index: 2; display: flex; flex-direction: column; gap: 6px; padding: 0 14px; }
.composer-region { position: absolute; left: 0; right: 0; bottom: 0; z-index: 3; }
.queue-edit-bar { display: flex; align-items: center; gap: 6px; width: min(760px, calc(100% - 48px)); margin: 0 auto 6px; padding: 4px 10px; border-radius: 10px; background: color-mix(in srgb, var(--ac-color-primary, var(--el-color-primary)) 12%, transparent); font-size: 11px; color: var(--el-text-color-secondary); }
.queue-edit-label { flex: none; font-weight: 600; color: var(--ac-color-primary, var(--el-color-primary)); }
.queue-edit-chip { display: inline-flex; align-items: center; gap: 2px; max-width: 180px; padding: 1px 4px 1px 7px; border-radius: 999px; background: var(--el-fill-color-light); color: var(--el-text-color-regular); }
.queue-edit-chip-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.queue-edit-chip button { border: 0; padding: 0 2px; background: transparent; color: var(--el-text-color-secondary); font-size: 12px; line-height: 1; cursor: pointer; }
.queue-edit-bar .queue-edit-cancel { margin-left: auto; border: 0; padding: 0 2px; background: transparent; color: var(--ac-color-primary, var(--el-color-primary)); font-size: 11px; cursor: pointer; }
.group-composer-members { display: flex; gap: 6px; width: min(760px, calc(100% - 48px)); margin: 0 auto; padding-bottom: 6px; overflow-x: auto; scrollbar-width: none; }
.group-composer-members::-webkit-scrollbar { display: none; }
.group-composer-members button { flex: 0 0 auto; min-height: 26px; padding: 2px 9px; border: 1px solid var(--surface-border); border-radius: 999px; background: var(--bg-surface, var(--chat-surface-bg)); color: var(--text-secondary); cursor: pointer; font-size: 11px; }
.group-composer-members button:hover, .group-composer-members button:focus-visible { border-color: var(--ac-color-primary); color: var(--text-primary); outline: none; }
.group-composer-members .mention-all { color: var(--ac-color-primary); }
.group-composer-members .discussion-mode { color: var(--ac-color-primary); }
.group-composer-members .discussion-mode.is-active { border-color: var(--ac-color-primary); background: color-mix(in srgb, var(--ac-color-primary) 14%, var(--bg-surface, var(--chat-surface-bg))); color: var(--text-primary); }
.group-composer-members .discussion-mode:disabled { cursor: not-allowed; opacity: 0.5; }
.chat-surface--sidebar .composer-region { right: min(320px, 32%); }
.chat-scroll-fade { position: absolute; left: 0; right: 0; z-index: 2; pointer-events: none; }
.chat-scroll-fade--top { top: 0; height: var(--chat-header-h, 44px); background: linear-gradient(to bottom, color-mix(in srgb, var(--chat-surface-bg) 96%, transparent), color-mix(in srgb, var(--chat-surface-bg) 0%, transparent)); }
.chat-scroll-fade--bottom { bottom: 0; height: var(--chat-composer-h, 120px); background: linear-gradient(to top, color-mix(in srgb, var(--chat-surface-bg) 96%, transparent), color-mix(in srgb, var(--chat-surface-bg) 0%, transparent)); }
.chat-surface--sidebar .chat-scroll-fade { right: min(320px, 32%); }
.chat-surface--embedded .chat-header-region,
.chat-surface--embedded .chat-sidebar-region,
.chat-surface--embedded .chat-status-region,
.chat-surface--embedded .chat-scroll-fade--top { display: none; }
.chat-surface--embedded .chat-scroll-fade--bottom { height: 84px; }
@media (max-width: 768px) {
  .group-composer-members { width: calc(100% - 16px); }
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
.conversation-messages-region {
  position: relative;
  display: flex;
  flex: 1 1 0;
  min-width: 0;
  min-height: 0;
}
.conversation-switch-loading {
  position: absolute;
  inset: 0;
  z-index: 12;
  display: grid;
  place-items: center;
  background: var(--chat-surface-bg);
}
.conversation-switch-loading__content {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  color: var(--text-secondary);
  font-size: 13px;
}
.conversation-switch-loading__spinner {
  color: var(--ac-color-primary, var(--text-secondary));
}
.chat-sidebar-region { width: min(320px, 32%); min-width: 220px; padding: 12px; border-left: 1px solid var(--surface-border); height: 100%; min-height: 0; overflow: hidden; display: flex; flex-direction: column; flex: 0 0 auto; }
@media (min-width: 1024px) { .chat-body-wrapper { flex-direction: row; } }
.sidebar-toggle-btn { display: grid; place-items: center; width: 32px; height: 32px; border: 1px solid transparent; border-radius: var(--radius-sm); background: transparent; color: var(--text-secondary); cursor: pointer; }
.sidebar-toggle-btn:hover { border-color: var(--surface-border); background: var(--control-hover-bg); color: var(--text-primary); }
</style>
