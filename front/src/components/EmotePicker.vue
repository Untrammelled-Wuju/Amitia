<template>
  <el-popover
    v-model:visible="visible"
    placement="top-start"
    :width="360"
    trigger="click"
    :teleported="true"
    append-to="#amitia-overlay-root"
    @show="loadAll"
  >
    <template #reference
      ><el-button
        circle
        size="small"
        :icon="ChatDotRound"
        :disabled="disabled"
        title="发送表情"
        aria-label="发送表情"
        class="emote-btn"
    /></template>
    <div class="picker">
      <div class="picker-head">
        <el-select
          v-model="groupId"
          placeholder="全部表情"
          clearable
          @change="loadEmotes"
          ><el-option label="最近使用" value="recent" /><el-option
            v-for="group in groups"
            :key="group.id"
            :label="group.name"
            :value="group.id" /></el-select
        ><el-input
          v-model="query"
          :prefix-icon="Search"
          clearable
          placeholder="搜索"
          @input="debounceLoad"
        />
      </div>
      <div v-loading="loading" class="picker-grid">
        <button
          v-for="item in emotes"
          :key="item.id"
          type="button"
          :title="item.meaning || item.name"
          @click="choose(item)"
        >
          <img
            :src="assetUrl(item.thumbnailPath)"
            :alt="item.meaning || item.name"
            loading="lazy"
          />
          <span>{{ item.name }}</span>
        </button>
        <div v-if="!loading && errorText" class="empty is-error">
          <span>{{ errorText }}</span
          ><el-button link type="primary" @click="loadEmotes">重试</el-button>
        </div>
        <div v-else-if="!loading && !emotes.length" class="empty">
          没有可用表情
        </div>
      </div>
    </div>
  </el-popover>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { ChatDotRound, Search } from "@element-plus/icons-vue";
import { useApi } from "@/composables/useApi";
import { useAssetUrl } from "@/composables/useAssetUrl";

defineProps<{ disabled?: boolean }>();
const emit = defineEmits<{ select: [emote: any] }>();
const { get } = useApi();
const { assetUrl } = useAssetUrl();
const visible = ref(false);
const loading = ref(false);
const groups = ref<any[]>([]);
const emotes = ref<any[]>([]);
const groupId = ref("");
const query = ref("");
const errorText = ref("");
let timer: ReturnType<typeof setTimeout> | undefined;
async function loadAll() {
  if (!groups.value.length) {
    try {
      groups.value = await get<any[]>("/api/emote-groups");
    } catch {
      groups.value = [];
    }
  }
  await loadEmotes();
}
async function loadEmotes() {
  loading.value = true;
  errorText.value = "";
  try {
    const recent = groupId.value === "recent";
    const data = await get<any>("/api/emotes", {
      groupId: recent ? undefined : groupId.value || undefined,
      view: recent ? "recent" : undefined,
      q: query.value,
      pageSize: 100,
    });
    emotes.value = data.items || [];
  } catch {
    emotes.value = [];
    errorText.value = "表情加载失败";
  } finally {
    loading.value = false;
  }
}
function debounceLoad() {
  clearTimeout(timer);
  timer = setTimeout(loadEmotes, 200);
}
function choose(item: any) {
  emit("select", item);
  visible.value = false;
}
</script>

<style scoped>
.emote-btn {
  width: 30px;
  height: 30px;
  border-color: transparent !important;
  background: transparent !important;
  color: var(--ac-color-text-muted) !important;
  font-size: 16px;
  box-shadow: none !important;
}

.emote-btn:hover,
.emote-btn:focus-visible {
  border-color: var(--ac-color-border) !important;
  background: var(--ac-color-bg-secondary) !important;
  color: var(--ac-color-text) !important;
}

.emote-btn:disabled {
  opacity: 0.55;
}

.picker {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.picker-head {
  display: grid;
  grid-template-columns: 130px 1fr;
  gap: 8px;
}

.picker-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 8px;
  max-height: 300px;
  overflow: auto;
}

.picker-grid > button {
  min-width: 0;
  padding: 6px;
  border: 1px solid var(--ac-color-border-light);
  border-radius: 8px;
  background: var(--ac-color-surface);
  color: var(--ac-color-text);
  cursor: pointer;
}

.picker-grid > button:hover,
.picker-grid > button:focus-visible {
  border-color: var(--ac-color-primary);
  outline: none;
}

.picker-grid img {
  width: 100%;
  height: 58px;
  object-fit: contain;
}

.picker-grid > button span {
  display: block;
  overflow: hidden;
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.empty {
  grid-column: 1 / -1;
  padding: 40px 0;
  color: var(--ac-color-text-muted);
  text-align: center;
}

.empty.is-error {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

@media (max-width: 480px) {
  .picker-head {
    grid-template-columns: 1fr;
  }
}
</style>
