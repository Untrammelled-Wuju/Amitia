<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div v-if="visible" class="fa-panel mem-inject-panel">
    <div class="mi-header">
      <h4>记忆上下文</h4>
      <button class="mi-close-btn" type="button" aria-label="关闭记忆上下文" @click="close">✕</button>
    </div>
    <div v-if="loading" class="mi-loading">加载中...</div>
    <div v-else>
      <div v-if="memories.length" class="mi-section">
        <h5>相关记忆 ({{ memories.length }})</h5>
        <div v-for="m in memories" :key="m.id" class="mi-item">
          <div class="mi-item-head">
            <span class="mi-layer">{{ memoryTypeLabel(m.memoryType || m.type) }}</span>
            <span class="mi-score">置信度 {{ m.confidence ?? 0 }}%</span>
          </div>
          <div class="mi-key" v-if="m.key">{{ m.key }}</div>
          <div class="mi-content">{{ m.value || m.memory?.value || "" }}</div>
        </div>
      </div>

      <div v-if="profiles.length" class="mi-section">
        <h5>用户画像 ({{ profiles.length }})</h5>
        <div v-for="p in profiles" :key="p.id" class="mi-profile-card">
          <span>{{ p.attributeName }}: {{ p.attributeValue }}</span>
          <el-tag :type="p.confidence >= 80 ? 'success' : 'warning'" size="small">{{ p.confidence }}%</el-tag>
        </div>
      </div>

      <div class="mi-section">
        <h5>压缩状态</h5>
        <div class="mi-compress">
          <span>已压缩 {{ compression.compressedRounds || 0 }} / {{ compression.totalRounds || 0 }} 轮</span>
          <span v-if="compression.lastCompressedAt">上次: {{ compression.lastCompressedAt }}</span>
        </div>
      </div>

      <div class="mi-section">
        <h5>管线状态</h5>
        <div v-if="pipeline?.layers?.length" class="mi-pipeline">
          <template v-for="l in pipeline.layers" :key="l.layer">
            <el-tooltip :content="l.name + ': ' + l.status + ' (' + l.durationMs + 'ms)'" placement="top">
              <span
                class="mi-pl-dot"
                :style="{
                  backgroundColor:
                    l.status === 'completed'
                      ? 'var(--ac-color-success)'
                      : l.status === 'skipped'
                        ? 'var(--ac-color-text-muted)'
                        : 'var(--ac-color-primary)',
                }"
              />
            </el-tooltip>
          </template>
        </div>
        <span v-else class="mi-muted">暂无管线状态</span>
      </div>

      <div v-if="!memories.length && !profiles.length" class="mi-empty">暂无可展示的记忆上下文</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import { useApi } from "../../../composables/useApi";

const props = defineProps<{
  visible: boolean;
  convId: string;
  characterId?: string;
}>();

const emit = defineEmits<{ close: [] }>();
const { get } = useApi();
const loading = ref(false);
const memories = ref<any[]>([]);
const profiles = ref<any[]>([]);
const compression = ref<any>({});
const pipeline = ref<any>(null);
let requestVersion = 0;

function close() {
  emit("close");
}

function memoryTypeLabel(type?: string) {
  const labels: Record<string, string> = {
    fact: "事实",
    preference: "偏好",
    episodic: "情景",
    relationship: "关系",
    custom: "记忆",
  };
  return labels[type || ""] || type || "记忆";
}

async function loadData() {
  if (!props.visible) return;
  const version = ++requestVersion;
  loading.value = true;

  const [memoryResult, profileResult, compressionResult, pipelineResult] = await Promise.allSettled([
    props.characterId
      ? get<any>("/api/memories", {
          page: 1,
          pageSize: 8,
          characterId: props.characterId,
          sort: "recently_used",
        })
      : Promise.resolve({ items: [] }),
    get<any>("/api/profiles", { page: 1, pageSize: 5 }),
    props.convId
      ? get<any>(`/api/chats/conversations/${encodeURIComponent(props.convId)}/compression-status`)
      : Promise.resolve({}),
    get<any>("/api/memory/pipeline/status"),
  ]);

  if (version !== requestVersion) return;
  memories.value = memoryResult.status === "fulfilled" ? memoryResult.value?.items || [] : [];
  profiles.value = profileResult.status === "fulfilled" ? profileResult.value?.items || [] : [];
  compression.value = compressionResult.status === "fulfilled" ? compressionResult.value || {} : {};
  pipeline.value = pipelineResult.status === "fulfilled" ? pipelineResult.value || null : null;
  loading.value = false;
}

watch(
  () => [props.visible, props.convId, props.characterId] as const,
  () => void loadData(),
  { immediate: true },
);
</script>

<style scoped>
.fa-panel {
  position: absolute;
  right: 14px;
  top: 8px;
  z-index: 19;
  overflow-y: auto;
  border: 1px solid var(--surface-border);
  border-radius: 10px;
  background: var(--surface-bg-elevated);
  box-shadow: var(--tp-shadow-float);
}
.mem-inject-panel { width: 330px; max-height: min(560px, 72vh); }
.mi-header { display: flex; align-items: center; justify-content: space-between; padding: 10px 12px; border-bottom: 1px solid var(--surface-border); }
.mi-header h4 { margin: 0; color: var(--text-primary); font-size: 13px; font-weight: 600; }
.mi-close-btn { display: grid; place-items: center; width: 26px; height: 26px; border: 0; border-radius: 6px; background: transparent; color: var(--text-muted); cursor: pointer; }
.mi-close-btn:hover { background: var(--control-hover-bg); color: var(--text-primary); }
.mi-loading, .mi-empty { padding: 22px 14px; text-align: center; color: var(--text-muted); font-size: 12px; }
.mi-section { padding: 9px 12px; border-bottom: 1px solid var(--surface-border); }
.mi-section:last-child { border-bottom: 0; }
.mi-section h5 { margin: 0 0 7px; color: var(--text-secondary); font-size: 11px; font-weight: 600; }
.mi-item { padding: 7px 0; border-bottom: 1px solid color-mix(in srgb, var(--surface-border) 70%, transparent); }
.mi-item:last-child { border-bottom: 0; }
.mi-item-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.mi-layer { color: var(--ac-color-primary); font-size: 10px; }
.mi-score { color: var(--text-muted); font-size: 9px; }
.mi-key { margin-top: 4px; color: var(--text-secondary); font-size: 11px; font-weight: 550; }
.mi-content { margin-top: 3px; color: var(--text-primary); font-size: 11px; line-height: 1.5; word-break: break-word; }
.mi-profile-card { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 5px 0; color: var(--text-secondary); font-size: 11px; }
.mi-compress { display: flex; flex-direction: column; gap: 3px; color: var(--text-muted); font-size: 10px; }
.mi-pipeline { display: flex; align-items: center; gap: 7px; min-height: 18px; }
.mi-pl-dot { display: inline-block; width: 7px; height: 7px; border-radius: 50%; }
.mi-muted { color: var(--text-muted); font-size: 10px; }
@media (max-width: 768px) { .mem-inject-panel { right: 8px; left: 8px; width: auto; max-height: 66vh; } }
</style>
