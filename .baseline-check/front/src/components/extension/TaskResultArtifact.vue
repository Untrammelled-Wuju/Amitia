<template>
  <div class="task-result-artifact">
    <template v-if="!result">
      <el-empty
        description="暂无结果"
        :image-size="60"
      />
    </template>
    <template v-else-if="result.resultType === 'inline_json'">
      <div class="result-header">
        <el-tag type="info" size="small">内联 JSON</el-tag>
        <span v-if="result.resultHash" class="result-hash" :title="result.resultHash">
          哈希：{{ shortHash(result.resultHash) }}
        </span>
      </div>
      <pre class="json-block">{{ prettyJson }}</pre>
    </template>
    <template v-else-if="result.resultType === 'artifact'">
      <div class="artifact-card">
        <div class="artifact-head">
          <el-icon class="artifact-icon"><Document /></el-icon>
          <div class="artifact-meta">
            <div class="artifact-name">{{ artifactName }}</div>
            <div class="artifact-sub">
              <span v-if="result.artifactId" class="artifact-id"
                >ArtifactID：{{ result.artifactId }}</span
              >
              <span v-if="formattedSize" class="artifact-size"
                >{{ formattedSize }}</span
              >
              <span v-if="result.artifactMime" class="artifact-mime"
                >{{ result.artifactMime }}</span
              >
            </div>
            <div v-if="result.resultHash" class="artifact-hash" :title="result.resultHash">
              哈希：{{ shortHash(result.resultHash) }}
            </div>
          </div>
        </div>
        <el-button
          type="primary"
          plain
          :icon="Download"
          :disabled="!downloadUrl"
          @click="download"
        >
          下载产物
        </el-button>
      </div>
    </template>
    <template v-else>
      <el-alert
        type="warning"
        :closable="false"
        :title="`未知结果类型：${result.resultType}`"
      />
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { Document, Download } from "@element-plus/icons-vue";
import { apiClient } from "@/composables/useApi";
import type { TaskResult } from "@/views/extensions/types";

const props = defineProps<{
  result?: TaskResult | null;
  taskRunId?: string;
}>();

const prettyJson = computed(() => {
  const data = props.result?.resultJson;
  if (data === undefined || data === null) return "—";
  if (typeof data === "string") {
    try {
      return JSON.stringify(JSON.parse(data), null, 2);
    } catch {
      return data;
    }
  }
  try {
    return JSON.stringify(data, null, 2);
  } catch {
    return String(data);
  }
});

const artifactName = computed(
  () => props.result?.artifactName || "任务结果产物",
);

const formattedSize = computed(() => {
  const size = Number(props.result?.artifactSize);
  if (!Number.isFinite(size) || size <= 0) return "";
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  if (size < 1024 * 1024 * 1024)
    return `${(size / 1024 / 1024).toFixed(1)} MB`;
  return `${(size / 1024 / 1024 / 1024).toFixed(2)} GB`;
});

const downloadUrl = computed(() => {
  const taskRunId = props.taskRunId;
  const artifactId = props.result?.artifactId;
  if (!taskRunId) return "";
  const base = `/api/extensions/tasks/${encodeURIComponent(taskRunId)}/result/artifact`;
  return artifactId ? `${base}?artifactId=${encodeURIComponent(artifactId)}` : base;
});

function shortHash(hash?: string) {
  if (!hash) return "—";
  return hash.length > 16 ? `${hash.slice(0, 12)}…` : hash;
}

async function download() {
  const url = downloadUrl.value;
  if (!url) return;
  const response = await apiClient.get(url, { responseType: "blob" });
  const blob = response.data as Blob;
  const objectUrl = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = objectUrl;
  anchor.download = artifactName.value || "task-result";
  anchor.click();
  URL.revokeObjectURL(objectUrl);
}
</script>

<style scoped>
.task-result-artifact {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
}
.result-header {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.result-hash {
  font-size: 12px;
  color: var(--ac-color-text-muted);
  font-family: "SFMono-Regular", Consolas, monospace;
}
.json-block {
  max-height: 360px;
  overflow: auto;
  margin: 0;
  padding: 14px;
  border-radius: var(--ac-radius-sm);
  background: var(--ac-color-bg-secondary);
  color: var(--ac-color-text);
  font:
    12px/1.6 "SFMono-Regular",
    Consolas,
    monospace;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.artifact-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px;
  border: 1px solid var(--console-border, var(--el-border-color));
  border-radius: 8px;
  background: var(--ac-color-surface, var(--el-fill-color-blank));
}
.artifact-head {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  min-width: 0;
}
.artifact-icon {
  font-size: 28px;
  color: var(--el-color-primary);
  flex: 0 0 auto;
}
.artifact-meta {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}
.artifact-name {
  font-weight: 600;
  color: var(--ac-color-text);
  overflow-wrap: anywhere;
}
.artifact-sub {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  font-size: 12px;
  color: var(--ac-color-text-muted);
}
.artifact-id {
  font-family: "SFMono-Regular", Consolas, monospace;
}
.artifact-hash {
  font-size: 12px;
  color: var(--ac-color-text-muted);
  font-family: "SFMono-Regular", Consolas, monospace;
}
@media (max-width: 720px) {
  .artifact-card {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
