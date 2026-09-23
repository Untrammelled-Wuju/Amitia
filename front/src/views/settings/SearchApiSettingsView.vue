<template>
  <div class="search-api-settings">
    <header class="page-header">
      <h2>搜索 API</h2>
      <p>配置商业搜索与内容引擎凭据，保存后由搜索工具直接调用。</p>
    </header>

    <div v-loading="loading" class="credential-list">
      <section
        v-for="item in items"
        :key="item.engineId"
        class="credential-item"
      >
        <div class="credential-head">
          <label :for="inputID(item.engineId)">{{ item.name }}</label>
          <span :class="['credential-status', { configured: item.configured }]">
            {{ item.configured ? "已配置" : "未配置" }}
          </span>
        </div>
        <el-input
          :id="inputID(item.engineId)"
          v-model="drafts[item.engineId]"
          type="password"
          show-password
          autocomplete="new-password"
          placeholder="请输入 API Key"
          clearable
        />
        <div class="credential-actions">
          <a
            v-if="item.keyUrl"
            :href="item.keyUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="key-link"
          >
            获取 API Key
          </a>
          <span v-else />
          <div class="action-buttons">
            <el-button
              text
              type="danger"
              :disabled="!item.configured || clearing[item.engineId]"
              @click="clearCredential(item)"
            >
              清除
            </el-button>
            <el-button
              type="primary"
              :loading="saving[item.engineId]"
              :disabled="!drafts[item.engineId]?.trim()"
              @click="saveCredential(item)"
            >
              保存
            </el-button>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { apiClient } from "@/composables/useApi";

interface CredentialItem {
  engineId: string;
  name: string;
  keyUrl: string;
  configured: boolean;
  updatedAt?: string;
}

const items = ref<CredentialItem[]>([]);
const drafts = reactive<Record<string, string>>({});
const saving = reactive<Record<string, boolean>>({});
const clearing = reactive<Record<string, boolean>>({});
const loading = ref(true);

function inputID(engineId: string) {
  return `search-credential-${engineId.replace(/[^a-zA-Z0-9_-]/g, "-")}`;
}

async function load() {
  loading.value = true;
  try {
    const response = await apiClient.get<{ items: CredentialItem[] }>(
      "/api/search/credentials",
    );
    const payload = response.data;
    items.value = payload.items ?? [];
    for (const item of items.value) {
      if (!(item.engineId in drafts)) {
        drafts[item.engineId] = "";
      }
    }
  } finally {
    loading.value = false;
  }
}

async function saveCredential(item: CredentialItem) {
  const value = drafts[item.engineId]?.trim() ?? "";
  if (!value) return;
  saving[item.engineId] = true;
  try {
    const response = await apiClient.put<CredentialItem>(
      `/api/search/credentials/${encodeURIComponent(item.engineId)}`,
      { value },
    );
    const updated = response.data;
    const current = items.value.find(
      (candidate) => candidate.engineId === item.engineId,
    );
    if (current && updated) {
      current.configured = updated.configured;
      current.updatedAt = updated.updatedAt;
    }
    drafts[item.engineId] = "";
    ElMessage.success(`${item.name} 已保存`);
  } finally {
    saving[item.engineId] = false;
  }
}

async function clearCredential(item: CredentialItem) {
  try {
    await ElMessageBox.confirm(
      `确定清除 ${item.name} 吗？`,
      "清除凭据",
      {
        type: "warning",
        confirmButtonText: "清除",
        cancelButtonText: "取消",
      },
    );
  } catch {
    return;
  }
  clearing[item.engineId] = true;
  try {
    await apiClient.delete(
      `/api/search/credentials/${encodeURIComponent(item.engineId)}`,
    );
    const current = items.value.find(
      (candidate) => candidate.engineId === item.engineId,
    );
    if (current) {
      current.configured = false;
      current.updatedAt = "";
    }
    drafts[item.engineId] = "";
    ElMessage.success(`${item.name} 已清除`);
  } finally {
    clearing[item.engineId] = false;
  }
}

onMounted(load);
</script>

<style scoped>
.search-api-settings {
  width: 100%;
  max-width: 920px;
  margin: 0 auto;
  padding: 20px 16px 40px;
}

.page-header {
  margin-bottom: 18px;
}

.page-header h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.page-header p {
  margin: 8px 0 0;
  font-size: 14px;
  line-height: 1.6;
  color: var(--el-text-color-secondary);
}

.credential-list {
  min-height: 180px;
}

.credential-item {
  padding: 22px 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.credential-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 12px;
}

.credential-head label {
  font-size: 15px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.credential-status {
  flex: none;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.credential-status.configured {
  color: var(--el-color-success);
}

.credential-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-top: 12px;
}

.key-link {
  font-size: 13px;
  color: var(--el-color-primary);
  text-decoration: none;
}

.key-link:hover {
  text-decoration: underline;
}

.action-buttons {
  display: flex;
  align-items: center;
  gap: 8px;
}

@media (max-width: 640px) {
  .search-api-settings {
    padding: 16px 12px 32px;
  }

  .credential-actions {
    align-items: flex-start;
    flex-direction: column;
  }

  .action-buttons {
    align-self: flex-end;
  }
}
</style>
