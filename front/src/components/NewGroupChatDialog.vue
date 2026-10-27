<template>
  <el-dialog
    :model-value="visible"
    title="新建群聊"
    width="min(540px, calc(100vw - 32px))"
    align-center
    destroy-on-close
    :close-on-click-modal="!creating"
    :close-on-press-escape="!creating"
    @update:model-value="$emit('update:visible', $event)"
  >
    <div class="new-group-dialog">
      <section>
        <div class="selector-heading">
          <div><strong>默认发言角色</strong><span>发送未点名消息时使用此角色身份</span></div>
        </div>
        <div class="role-grid role-grid--single">
          <button
            v-for="character in characters"
            :key="character.id"
            type="button"
            class="role-option"
            :class="{ 'is-selected': baseRoleId === character.id }"
            @click="selectBaseRole(character.id)"
          >
            <el-avatar :size="34" :src="character.avatar || undefined" fit="cover">{{ character.name.slice(0, 1) }}</el-avatar>
            <span class="role-name">{{ character.name }}</span>
            <el-icon v-if="baseRoleId === character.id" class="selection-check"><Check /></el-icon>
          </button>
        </div>
      </section>
      <section>
        <div class="selector-heading">
          <div><strong>邀请角色</strong><span>至少选择 1 个，最多共 8 个群成员</span></div>
          <span>{{ selected.size }} 已选择</span>
        </div>
        <div class="role-grid">
          <button
            v-for="character in inviteCharacters"
            :key="character.id"
            type="button"
            class="role-option"
            :class="{ 'is-selected': selected.has(character.id) }"
            @click="toggleCharacter(character.id)"
          >
            <el-avatar :size="34" :src="character.avatar || undefined" fit="cover">{{ character.name.slice(0, 1) }}</el-avatar>
            <span class="role-name">{{ character.name }}</span>
            <el-icon v-if="selected.has(character.id)" class="selection-check"><Check /></el-icon>
          </button>
        </div>
        <div v-if="!inviteCharacters.length" class="empty-roles">没有其他可邀请角色</div>
      </section>
      <el-checkbox v-model="shareHistory">允许新成员查看加入前聊天历史</el-checkbox>
    </div>
    <template #footer>
      <el-button :disabled="creating" @click="$emit('update:visible', false)">取消</el-button>
      <el-button type="primary" :loading="creating" :disabled="!canCreate" @click="create">创建群聊</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { Check } from "@element-plus/icons-vue";
import { ElMessage } from "element-plus";
import { useDeviceOwnedConversation } from "@/composables/useDeviceOwnedConversation";
import type { OwnedGroupState } from "@/runtime/device-owned-chat";

const props = defineProps<{
  visible: boolean;
  characters: Array<{ id: string; name: string; avatar?: string }>;
  defaultRoleId?: string;
}>();
const emit = defineEmits<{
  "update:visible": [value: boolean];
  created: [group: OwnedGroupState, baseRoleId: string];
}>();

const owned = useDeviceOwnedConversation();
const baseRoleId = ref("");
const selected = ref(new Set<string>());
const shareHistory = ref(false);
const creating = ref(false);
const inviteCharacters = computed(() => props.characters.filter((character) => character.id !== baseRoleId.value));
const canCreate = computed(() => Boolean(baseRoleId.value && selected.value.size > 0 && selected.value.size < 8));

watch(() => props.visible, (visible) => {
  if (!visible) return;
  selected.value = new Set();
  shareHistory.value = false;
  baseRoleId.value = props.characters.some((character) => character.id === props.defaultRoleId)
    ? String(props.defaultRoleId)
    : props.characters[0]?.id || "";
});

function selectBaseRole(id: string) {
  baseRoleId.value = id;
  const next = new Set(selected.value);
  next.delete(id);
  selected.value = next;
}

function toggleCharacter(id: string) {
  const next = new Set(selected.value);
  if (next.has(id)) next.delete(id);
  else if (next.size < 7) next.add(id);
  selected.value = next;
}

async function create() {
  if (!canCreate.value || creating.value) return;
  creating.value = true;
  try {
    const group = await owned.createGroupConversation(baseRoleId.value, Array.from(selected.value).map((characterId) => ({ characterId, shareHistory: shareHistory.value })));
    emit("created", group, baseRoleId.value);
    emit("update:visible", false);
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "创建群聊失败");
  } finally {
    creating.value = false;
  }
}
</script>

<style scoped>
.new-group-dialog { display: flex; flex-direction: column; gap: 20px; }
.new-group-dialog section { display: flex; flex-direction: column; gap: 10px; }
.selector-heading { display: flex; align-items: flex-end; justify-content: space-between; gap: 12px; }
.selector-heading div { display: flex; flex-direction: column; gap: 3px; }
.selector-heading strong { color: var(--text-primary); font-size: 13px; }
.selector-heading span, .empty-roles { color: var(--text-muted); font-size: 11px; }
.role-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; max-height: 260px; overflow-y: auto; }
.role-grid--single { grid-template-columns: 1fr; max-height: 150px; }
.role-option { display: flex; align-items: center; gap: 10px; min-width: 0; min-height: 52px; padding: 8px 10px; border: 1px solid var(--border-subtle); border-radius: 10px; background: var(--surface-bg-elevated); color: var(--text-secondary); cursor: pointer; text-align: left; }
.role-option:hover, .role-option:focus-visible { border-color: var(--ac-color-primary); color: var(--text-primary); outline: none; }
.role-option.is-selected { border-color: var(--ac-color-primary); background: color-mix(in srgb, var(--ac-color-primary) 10%, var(--surface-bg-elevated)); color: var(--text-primary); }
.role-option :deep(.el-avatar) { flex: 0 0 34px; }
.role-name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.selection-check { flex: 0 0 auto; color: var(--ac-color-primary); }
@media (max-width: 560px) { .role-grid { grid-template-columns: 1fr; } }
</style>
