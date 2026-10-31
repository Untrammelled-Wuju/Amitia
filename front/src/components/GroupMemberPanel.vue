<template>
  <el-drawer :model-value="visible" title="群聊成员" size="420px" direction="rtl" append-to-body @update:model-value="$emit('update:visible', $event)">
    <div class="group-panel">
      <template v-if="group">
        <el-input v-model="nameDraft" maxlength="128" show-word-limit placeholder="群聊名称" @change="saveName" />
        <section class="group-summary">
          <div>
            <strong>{{ group.title || "多角色群聊" }}</strong>
            <span>{{ activeMembers.length }} 位角色</span>
          </div>
          <el-tag size="small" type="success">群聊</el-tag>
        </section>
        <section class="group-section">
          <h4>成员</h4>
          <div class="member-list">
            <div v-for="member in activeMembers" :key="member.memberId" class="member-row">
              <el-avatar :size="34" :src="member.characterAvatar || undefined" fit="cover">{{ member.characterName.slice(0, 1) }}</el-avatar>
              <div class="member-copy">
                <strong>{{ member.characterName }}</strong>
                <span>{{ member.historyStartSequence > 0 ? "加入后历史可见" : "可见完整历史" }}</span>
              </div>
              <button type="button" class="mention-member" @click="$emit('mention', member)">@</button>
              <button v-if="replySourceMessageId" type="button" class="reply-member" @click="requestReply(member)">让 TA 回复</button>
              <button v-if="activeMembers.length > 1" type="button" class="remove-member" @click="remove(member)">移除</button>
            </div>
          </div>
        </section>
        <section class="group-section">
          <h4>成员设置</h4>
          <div class="setting-row">
            <div><strong>允许角色互相 @</strong><span>角色可邀请另一位成员接话，也可参与多角色讨论</span></div>
            <el-switch :model-value="group.groupSettings.allowCharacterMentions" :loading="savingSettings" @update:model-value="toggleMentions" />
          </div>
        </section>
        <section v-if="failedDeliveries.length" class="group-section">
          <h4>失败任务</h4>
          <div class="member-list">
            <div v-for="delivery in failedDeliveries" :key="delivery.deliveryId" class="failed-row">
              <div class="member-copy"><strong>{{ delivery.characterName }}</strong><span>{{ delivery.error || "生成失败" }}</span></div>
              <button type="button" class="retry-delivery" @click="retry(delivery)">重试</button>
            </div>
          </div>
        </section>
        <section v-if="availableCharacters.length" class="group-section">
          <h4>邀请角色</h4>
          <div class="invite-list">
            <label v-for="character in availableCharacters" :key="character.id" class="invite-row">
              <el-checkbox :model-value="selected.has(character.id)" @update:model-value="toggleSelection(character.id, Boolean($event))" />
              <el-avatar :size="30" :src="character.avatar || undefined" fit="cover">{{ character.name.slice(0, 1) }}</el-avatar>
              <span>{{ character.name }}</span>
            </label>
          </div>
          <el-checkbox v-model="shareHistory">允许新成员查看加入前聊天历史</el-checkbox>
          <el-button class="invite-submit" :loading="inviting" :disabled="selected.size === 0 || activeMembers.length + selected.size > 8" @click="invite">邀请 {{ selected.size }} 位角色</el-button>
        </section>
      </template>
      <template v-else>
        <div class="group-empty">
          <strong>当前是单聊</strong>
          <p>邀请其他角色后，原会话和消息保持不变。</p>
        </div>
        <section class="group-section">
          <h4>邀请角色</h4>
          <div class="invite-list">
            <label v-for="character in availableCharacters" :key="character.id" class="invite-row">
              <el-checkbox :model-value="selected.has(character.id)" @update:model-value="toggleSelection(character.id, Boolean($event))" />
              <el-avatar :size="30" :src="character.avatar || undefined" fit="cover">{{ character.name.slice(0, 1) }}</el-avatar>
              <span>{{ character.name }}</span>
            </label>
          </div>
          <p v-if="!availableCharacters.length" class="group-help">没有可邀请的其他角色。</p>
          <el-checkbox v-model="shareHistory">允许新成员查看加入前聊天历史</el-checkbox>
          <el-button class="invite-submit" :loading="inviting" :disabled="selected.size === 0 || selected.size > 7" @click="invite">创建群聊</el-button>
        </section>
      </template>
    </div>
  </el-drawer>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { useDeviceOwnedConversation } from "@/composables/useDeviceOwnedConversation";
import type { OwnedGroupDelivery, OwnedGroupMember, OwnedGroupState } from "@/runtime/device-owned-chat";

const props = defineProps<{
  visible: boolean;
  group: OwnedGroupState | null;
  characters: Array<{ id: string; name: string; avatar?: string }>;
  roleId: string;
  conversationId: string;
  replySourceMessageId?: string;
}>();
const emit = defineEmits<{
  "update:visible": [value: boolean];
  changed: [group: OwnedGroupState];
  mention: [member: OwnedGroupMember];
  replied: [];
}>();

const owned = useDeviceOwnedConversation();
const selected = ref(new Set<string>());
const shareHistory = ref(false);
const inviting = ref(false);
const savingSettings = ref(false);
const nameDraft = ref("");
const failedDeliveries = ref<OwnedGroupDelivery[]>([]);
const activeMembers = computed(() => (props.group?.members || []).filter((member) => member.status === "active").sort((left, right) => left.orderIndex - right.orderIndex));
const activeCharacterIDs = computed(() => new Set(activeMembers.value.map((member) => member.characterId)));
const availableCharacters = computed(() => props.characters.filter((character) => character.id !== props.roleId && !activeCharacterIDs.value.has(character.id)));

watch(() => props.visible, (visible) => {
  if (!visible) selected.value = new Set();
  else {
    nameDraft.value = props.group?.title || "";
    void loadFailures();
  }
});
watch(() => props.group?.title, (title) => { nameDraft.value = title || ""; });

function toggleSelection(id: string, checked: boolean) {
  const next = new Set(selected.value);
  if (checked) next.add(id);
  else next.delete(id);
  selected.value = next;
}

async function invite() {
  if (!props.conversationId || !props.roleId || selected.value.size === 0) return;
  inviting.value = true;
  try {
    const next = await owned.inviteGroupMembers(props.conversationId, props.roleId, Array.from(selected.value).map((characterId) => ({ characterId, shareHistory: shareHistory.value })), props.group?.membershipRevision || 1);
    selected.value = new Set();
    emit("changed", next);
    ElMessage.success("群成员已保存");
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "邀请成员失败");
  } finally {
    inviting.value = false;
  }
}

async function remove(member: OwnedGroupMember) {
  if (!props.group || !props.conversationId || !props.roleId) return;
  try {
    const next = await owned.removeGroupMember(props.conversationId, props.roleId, member.memberId, props.group.membershipRevision);
    emit("changed", next);
    ElMessage.success("成员已移除");
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "移除成员失败");
  }
}

async function toggleMentions(value: string | number | boolean) {
  if (!props.group || !props.conversationId || !props.roleId) return;
  savingSettings.value = true;
  try {
    const next = await owned.updateGroupSettings(props.conversationId, props.roleId, props.group.settingsRevision, { allowCharacterMentions: Boolean(value) });
    emit("changed", next);
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "群设置保存失败");
  } finally {
    savingSettings.value = false;
  }
}

async function requestReply(member: OwnedGroupMember) {
  if (!props.group || !props.conversationId || !props.roleId || !props.replySourceMessageId) return;
  try {
    await owned.requestGroupReplies(props.conversationId, props.roleId, props.replySourceMessageId, [{ memberId: member.memberId, characterId: member.characterId, roleOwnerId: member.roleOwnerId }]);
    emit("replied");
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "点名回复失败");
  }
}

async function saveName(value: string) {
  if (!props.group || !props.conversationId || !props.roleId) return;
  const name = value.trim();
  if (!name || name === props.group.title) return;
  try {
    const next = await owned.updateGroupSettings(props.conversationId, props.roleId, props.group.settingsRevision, { name });
    emit("changed", next);
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "群名称保存失败");
  }
}

async function loadFailures() {
  if (!props.group || !props.conversationId || !props.roleId) {
    failedDeliveries.value = [];
    return;
  }
  try {
    failedDeliveries.value = await owned.failedGroupDeliveries(props.conversationId, props.roleId);
  } catch (error) {
  console.error("[original-error]", error);
failedDeliveries.value = [];
  }
}

async function retry(delivery: OwnedGroupDelivery) {
  if (!props.conversationId || !props.roleId) return;
  try {
    await owned.retryGroupDelivery(props.conversationId, props.roleId, delivery.groupRoundId, delivery.deliveryId);
    await loadFailures();
    emit("replied");
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "群成员重试失败");
  }
}
</script>

<style scoped>
.group-panel { display: flex; flex-direction: column; gap: 22px; color: var(--text-primary); }
.group-summary, .member-row, .setting-row, .invite-row { display: flex; align-items: center; gap: 10px; }
.group-summary { justify-content: space-between; padding: 12px 0; border-bottom: 1px solid var(--border-subtle); }
.group-summary div, .member-copy, .setting-row div { display: flex; flex-direction: column; gap: 3px; min-width: 0; }
.group-summary span, .member-copy span, .setting-row span, .group-help { color: var(--text-muted); font-size: 12px; }
.group-section { display: flex; flex-direction: column; gap: 12px; }
.group-section h4 { margin: 0; font-size: 13px; }
.member-list, .invite-list { display: flex; flex-direction: column; gap: 8px; }
.member-row { padding: 8px 10px; border: 1px solid var(--border-subtle); border-radius: 10px; }
.member-row :deep(.el-avatar), .invite-row :deep(.el-avatar) { flex: 0 0 auto; }
.member-row :deep(.el-avatar img), .invite-row :deep(.el-avatar img) { width: 100%; height: 100%; object-fit: cover; }
.failed-row { display: flex; align-items: center; gap: 10px; padding: 8px 10px; border: 1px solid color-mix(in srgb, var(--danger-color, #d14343) 35%, var(--border-subtle)); border-radius: 10px; }
.member-copy { flex: 1; overflow: hidden; }
.member-copy strong, .invite-row span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mention-member, .remove-member, .reply-member { border: 0; background: transparent; color: var(--text-muted); cursor: pointer; padding: 4px; }
.mention-member { color: var(--ac-color-primary); font-weight: 700; }
.reply-member { color: var(--ac-color-primary); font-size: 11px; }
.retry-delivery { border: 1px solid var(--surface-border); border-radius: 7px; background: transparent; color: var(--text-primary); padding: 4px 9px; cursor: pointer; }
.remove-member:hover { color: var(--danger-color, #d14343); }
.setting-row { justify-content: space-between; align-items: flex-start; }
.setting-row div { flex: 1; }
.invite-row { min-height: 38px; cursor: pointer; }
.invite-row span { flex: 1; }
.invite-submit { width: 100%; }
.group-empty { padding: 10px 0; }
.group-empty p { margin: 6px 0 0; color: var(--text-muted); font-size: 12px; }
</style>
