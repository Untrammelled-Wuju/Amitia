<template>
  <div
    ref="pickerRef"
    class="composer-picker"
    role="dialog"
    :aria-label="dialogLabel"
    aria-modal="false"
    :tabindex="dismissible ? -1 : undefined"
    @keydown.esc.stop.prevent="handleEscape"
  >
    <template v-if="mode === 'custom'">
      <slot name="header">
        <div class="picker-heading">
          <span>{{ headingLabel }}</span>
          <button
            v-if="dismissible"
            type="button"
            class="picker-close"
            aria-label="关闭面板"
            @click="$emit('dismiss')"
          >
            <el-icon><CloseBold /></el-icon>
          </button>
        </div>
      </slot>

      <slot name="toolbar" />

      <div
        :id="listId"
        class="picker-list picker-list--custom"
        role="region"
        :aria-label="listLabel"
      >
        <slot />
        <slot name="empty" />
      </div>
      <slot name="footer" />
    </template>

    <template v-else>
      <button
        v-if="mode === 'menu'"
        :id="uploadOptionId"
        type="button"
        class="picker-action"
        :class="{ 'is-active': activeIndex === 0 }"
        @mouseenter="$emit('update:activeIndex', 0)"
        @mousedown.prevent
        @click="$emit('upload')"
      >
        <span class="picker-icon"><el-icon><Document /></el-icon></span>
        <span class="picker-copy">
          <strong>上传文件</strong>
          <small>支持图片、视频、文档和其他文件</small>
        </span>
      </button>

      <div class="picker-heading">
        <span>{{ headingLabel }}</span>
        <small v-if="mode !== 'menu'">{{ triggerSymbol }}{{ query }}</small>
      </div>

      <div
        :id="listId"
        class="picker-list"
        role="listbox"
        :aria-label="listLabel"
        @scroll="handleScroll"
      >
        <template v-if="mode === 'mentions'">
          <button
            v-if="showAllMention"
            :id="mentionOptionId(0)"
            type="button"
            class="picker-option"
            :class="{ 'is-active': activeIndex === 0 }"
            role="option"
            :aria-selected="activeIndex === 0"
            @mouseenter="$emit('update:activeIndex', 0)"
            @mousedown.prevent
            @click="$emit('selectMention', null)"
          >
            <span class="picker-icon"><el-icon><User /></el-icon></span>
            <span class="picker-copy">
              <strong>@所有成员</strong>
              <small>提醒当前群聊中的所有有效成员</small>
            </span>
          </button>
          <button
            v-for="(member, index) in members"
            :id="mentionOptionId(showAllMention ? index + 1 : index)"
            :key="member.memberId"
            type="button"
            class="picker-option"
            :class="{ 'is-active': activeIndex === (showAllMention ? index + 1 : index) }"
            role="option"
            :aria-selected="activeIndex === (showAllMention ? index + 1 : index)"
            @mouseenter="$emit('update:activeIndex', showAllMention ? index + 1 : index)"
            @mousedown.prevent
            @click="$emit('selectMention', member)"
          >
            <span class="picker-icon"><el-icon><User /></el-icon></span>
            <span class="picker-copy">
              <strong>@{{ member.characterName }}</strong>
              <small>群聊角色</small>
            </span>
          </button>
          <div v-if="!showAllMention && members.length === 0" class="picker-empty">
            没有匹配的群成员
          </div>
        </template>

        <template v-else>
          <button
            v-for="(skill, index) in skills"
            :id="skillOptionId(menuOffset + index)"
            :key="skillKey(skill)"
            type="button"
            class="picker-option"
            :class="{
              'is-active': activeIndex === menuOffset + index,
              'is-selected': selectedSkillNames.includes(skill.name),
            }"
            role="option"
            :aria-selected="selectedSkillNames.includes(skill.name)"
            @mouseenter="$emit('update:activeIndex', menuOffset + index)"
            @mousedown.prevent
            @click="$emit('selectSkill', skill.name)"
          >
            <span class="picker-icon"><el-icon><MagicStick /></el-icon></span>
            <span class="picker-copy">
              <strong>{{ skill.displayName || skill.name }}</strong>
              <small>{{ skill.shortDescription || skill.description || "暂无描述" }}</small>
            </span>
            <el-icon
              v-if="selectedSkillNames.includes(skill.name)"
              class="picker-check"
            >
              <Check />
            </el-icon>
          </button>
          <div
            v-if="skills.length === 0 && (loading || loadingMore)"
            class="picker-empty"
          >
            正在加载技能...
          </div>
          <div v-else-if="skills.length === 0" class="picker-empty">
            暂无匹配技能
          </div>
          <div v-if="loadingMore" class="picker-loading">
            正在加载更多技能...
          </div>
          <div v-else-if="skills.length > 0 && !hasMore" class="picker-end">
            已显示全部技能
          </div>
        </template>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import {
  Check,
  CloseBold,
  Document,
  MagicStick,
  User,
} from "@element-plus/icons-vue";
import type { AgentSkillDefinition } from "../views/extensions/types";
import type { OwnedGroupMember } from "@/runtime/device-owned-chat";

const props = withDefaults(
  defineProps<{
    mode: "menu" | "skills" | "mentions" | "custom";
    query?: string;
    title?: string;
    skills?: AgentSkillDefinition[];
    members?: OwnedGroupMember[];
    selectedSkillNames?: string[];
    activeIndex?: number;
    loading?: boolean;
    loadingMore?: boolean;
    hasMore?: boolean;
    dismissible?: boolean;
    instanceId?: string;
  }>(),
  {
    query: "",
    title: "输入框扩展",
    skills: () => [],
    members: () => [],
    selectedSkillNames: () => [],
    activeIndex: 0,
    loading: false,
    loadingMore: false,
    hasMore: false,
    dismissible: false,
    instanceId: "",
  },
);

const emit = defineEmits<{
  upload: [];
  selectSkill: [name: string];
  selectMention: [member: OwnedGroupMember | null];
  loadMore: [];
  dismiss: [];
  "update:activeIndex": [index: number];
}>();

const pickerRef = ref<HTMLElement>();
const listId = "composer-picker-list";
const uploadOptionId = "composer-picker-upload";
const menuOffset = computed(() => (props.mode === "menu" ? 1 : 0));
const showAllMention = computed(
  () => !props.query || "所有成员".includes(props.query),
);
const headingLabel = computed(() => {
  if (props.mode === "custom") return props.title;
  if (props.mode === "mentions") return "群成员";
  return props.mode === "menu" ? "使用技能" : "选择技能";
});
const triggerSymbol = computed(() => (props.mode === "mentions" ? "@" : "/"));
const dialogLabel = computed(() => {
  if (props.mode === "custom") return props.title;
  if (props.mode === "mentions") return "选择群成员";
  return props.mode === "menu" ? "上传文件或选择技能" : "选择技能";
});
const listLabel = computed(() =>
  props.mode === "custom"
    ? props.title
    : props.mode === "mentions"
      ? "群成员列表"
      : "技能列表",
);
const activeOptionId = computed(() => {
  if (props.mode === "custom") return undefined;
  if (props.mode === "menu") {
    if (props.activeIndex === 0) return uploadOptionId;
    return skillOptionId(props.activeIndex - 1);
  }
  if (props.mode === "mentions") return mentionOptionId(props.activeIndex);
  return skillOptionId(props.activeIndex);
});

function skillKey(skill: AgentSkillDefinition) {
  return `${skill.extensionId}:${skill.name}`;
}

function skillOptionId(index: number) {
  return `composer-picker-skill-${Math.max(0, index)}`;
}

function mentionOptionId(index: number) {
  return `composer-picker-mention-${Math.max(0, index)}`;
}

function handleScroll(event: Event) {
  if (!props.hasMore || props.loading || props.loadingMore) return;
  const target = event.target as HTMLElement;
  if (target.scrollHeight - target.scrollTop - target.clientHeight < 56) {
    emit("loadMore");
  }
}

function handleEscape() {
  if (props.dismissible) emit("dismiss");
}

function handleOutsidePointer(event: PointerEvent) {
  if (!props.dismissible) return;
  const target = event.target as Element | null;
  if (!target || pickerRef.value?.contains(target)) return;
  if (target.closest?.("[data-composer-picker-trigger]")) return;
  emit("dismiss");
}

function handleOtherPanel(event: Event) {
  if (!props.dismissible) return;
  const detail = (event as CustomEvent<{ instanceId?: string }>).detail;
  if (!detail?.instanceId || detail.instanceId === props.instanceId) return;
  emit("dismiss");
}

watch(
  activeOptionId,
  async (id) => {
    if (!id) return;
    await nextTick();
    document.getElementById(id)?.scrollIntoView?.({ block: "nearest" });
  },
);

onMounted(() => {
  if (!props.dismissible) return;
  document.addEventListener("pointerdown", handleOutsidePointer, true);
  window.addEventListener(
    "amitia:composer-floating-open",
    handleOtherPanel,
  );
  window.dispatchEvent(
    new CustomEvent("amitia:composer-floating-open", {
      detail: { instanceId: props.instanceId },
    }),
  );
  nextTick(() => pickerRef.value?.focus());
});

onUnmounted(() => {
  document.removeEventListener("pointerdown", handleOutsidePointer, true);
  window.removeEventListener(
    "amitia:composer-floating-open",
    handleOtherPanel,
  );
});

defineExpose({
  activeOptionId,
  listId,
});
</script>

<style scoped>
.composer-picker {
  position: absolute;
  z-index: 40;
  right: 0;
  bottom: calc(100% + 8px);
  left: 0;
  display: flex;
  max-height: min(396px, 48vh);
  flex-direction: column;
  overflow: hidden;
  padding: 7px;
  border: 1px solid var(--composer-border);
  border-radius: var(--radius-composer);
  background: var(--composer-bg);
  color: var(--text-primary);
}

.picker-action,
.picker-option {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 10px;
  border: 0;
  border-radius: 9px;
  background: transparent;
  color: var(--text-primary);
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.picker-action {
  min-height: 52px;
  flex: 0 0 auto;
  padding: 7px 9px;
}

.picker-option {
  min-height: 50px;
  padding: 7px 9px;
}

.picker-action:hover,
.picker-action:focus-visible,
.picker-action.is-active,
.picker-option:hover,
.picker-option:focus-visible,
.picker-option.is-active {
  outline: 0;
  background: var(--surface-hover);
}

.picker-option.is-selected {
  background: var(--ac-color-primary-bg);
}

.picker-icon {
  display: grid;
  width: 32px;
  height: 32px;
  flex: 0 0 auto;
  place-items: center;
  border-radius: 9px;
  background: var(--ac-color-bg-secondary);
  color: var(--text-secondary);
  font-size: 15px;
}

.picker-copy {
  min-width: 0;
  flex: 1;
}

.picker-copy strong,
.picker-copy small {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.picker-copy strong {
  font-size: 12px;
  font-weight: 600;
}

.picker-copy small {
  margin-top: 3px;
  color: var(--text-muted);
  font-size: 10px;
  line-height: 1.35;
}

.picker-heading {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 9px 6px;
  color: var(--text-muted);
  font-size: 11px;
  font-weight: 600;
}

.picker-heading small {
  min-width: 0;
  overflow: hidden;
  font-weight: 400;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.picker-close {
  display: grid;
  width: 26px;
  height: 26px;
  flex: 0 0 auto;
  place-items: center;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
}

.picker-close:hover,
.picker-close:focus-visible {
  outline: 0;
  background: var(--surface-hover);
  color: var(--text-primary);
}

.picker-list {
  display: grid;
  min-height: 0;
  align-content: start;
  gap: 2px;
  overflow-y: auto;
  overscroll-behavior: contain;
}

.picker-list--custom {
  display: block;
}

.picker-check {
  flex: 0 0 auto;
  color: var(--ac-color-primary);
}

.picker-empty,
.picker-loading,
.picker-end {
  padding: 16px 10px;
  color: var(--text-muted);
  font-size: 11px;
  text-align: center;
}

.picker-loading,
.picker-end {
  padding: 8px 10px 6px;
}

@media (max-width: 768px) {
  .composer-picker {
    max-height: min(340px, 44vh);
  }
}
</style>
