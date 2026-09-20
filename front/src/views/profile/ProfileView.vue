<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="profile-page">
    <div class="page-header">
      <h2>用户画像</h2>
      <div class="header-actions">
        <el-select
          v-model="filterCategory"
          placeholder="全部类别"
          clearable
          size="small"
          style="width: 140px"
          @change="onFilterChange"
        >
          <el-option label="全部类别" value="" />
          <el-option
            v-for="(label, key) in categoryMap"
            :key="key"
            :label="label"
            :value="key"
          />
        </el-select>
        <el-button type="primary" size="small" @click="openCreate"
          >+ 新增画像</el-button
        >
      </div>
    </div>

    <div v-if="loading" class="loading">加载中...</div>

    <div v-else class="profile-grid">
      <article
        v-for="p in profiles"
        :key="p.id"
        class="profile-card"
      >
        <header class="card-header">
          <span class="category-badge">{{ categoryLabel(p.category) }}</span>
          <div class="card-actions">
            <el-tooltip content="编辑画像" placement="top">
              <el-button
                class="card-action-button"
                size="small"
                text
                circle
                :aria-label="`编辑${p.attributeName}画像`"
                @click="editProfile(p)"
              >
                <el-icon><EditPen /></el-icon>
              </el-button>
            </el-tooltip>
            <el-tooltip content="删除画像" placement="top">
              <el-button
                class="card-action-button card-action-button--danger"
                size="small"
                text
                circle
                :aria-label="`删除${p.attributeName}画像`"
                @click="handleDelete(p.id)"
              >
                <el-icon><Delete /></el-icon>
              </el-button>
            </el-tooltip>
          </div>
        </header>
        <div class="card-body">
          <h3 class="attr-name">{{ p.attributeName }}</h3>
          <p class="attr-value">{{ p.attributeValue }}</p>
        </div>
        <footer class="card-footer">
          <div class="confidence-bar">
            <div
              class="confidence-track"
              role="progressbar"
              :aria-label="`${p.attributeName}可信度`"
              :aria-valuenow="p.confidence"
              aria-valuemin="0"
              aria-valuemax="100"
            >
              <div
                class="confidence-fill"
                :style="{ width: p.confidence + '%' }"
              ></div>
            </div>
            <span class="confidence-text">{{ p.confidence }}%</span>
          </div>
          <el-tooltip
            v-if="p.sourceConvId"
            content="查看来源对话"
            placement="top"
          >
            <span
              class="source-info"
              :title="'来源对话: ' + p.sourceConvId"
              :aria-label="`查看${p.attributeName}的来源对话`"
            >
              <el-icon><ChatLineRound /></el-icon>
            </span>
          </el-tooltip>
        </footer>
      </article>

      <div v-if="profiles.length === 0" class="empty-state">
        暂无画像数据，开始对话后将自动提取
      </div>
    </div>

    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="440px"
      @close="closeModal"
      destroy-on-close
    >
      <el-form label-width="80px" @submit.prevent="handleSubmit">
        <el-form-item label="类别">
          <el-select v-model="form.category" style="width: 100%">
            <el-option
              v-for="(label, key) in categoryMap"
              :key="key"
              :label="label"
              :value="key"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="属性名">
          <el-input
            v-model="form.attributeName"
            placeholder="如：姓名、爱好、职业"
          />
        </el-form-item>
        <el-form-item label="属性值">
          <el-input
            v-model="form.attributeValue"
            placeholder="如：张三、喜欢摄影"
          />
        </el-form-item>
        <el-form-item label="置信度">
          <el-input-number
            v-model="form.confidence"
            :min="0"
            :max="100"
            style="width: 100%"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="closeModal">取消</el-button>
        <el-button type="primary" @click="handleSubmit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from "vue";
import { ElMessageBox } from "element-plus";
import {
  ChatLineRound,
  Delete,
  EditPen,
} from "@element-plus/icons-vue";
import { useProfile, type UserProfile } from "@/composables/useProfile";

const {
  profiles,
  loading,
  fetchProfiles,
  createProfile,
  updateProfile,
  deleteProfile,
  categoryLabel,
} = useProfile();

const categoryMap: Record<string, string> = {
  personal_info: "个人信息",
  preference: "偏好",
  habit: "习惯",
  fear: "恐惧",
  relationship: "关系",
  health: "健康",
  plan: "计划",
};

const filterCategory = ref("");
const dialogVisible = ref(false);
const editingProfile = ref<UserProfile | null>(null);
const dialogTitle = ref("新增画像");

const form = reactive({
  category: "personal_info",
  attributeName: "",
  attributeValue: "",
  confidence: 50,
});

onMounted(() => {
  fetchProfiles();
});

function onFilterChange() {
  fetchProfiles({ category: filterCategory.value || undefined });
}

function openCreate() {
  editingProfile.value = null;
  form.category = "personal_info";
  form.attributeName = "";
  form.attributeValue = "";
  form.confidence = 50;
  dialogTitle.value = "新增画像";
  dialogVisible.value = true;
}

function editProfile(p: UserProfile) {
  editingProfile.value = p;
  form.category = p.category;
  form.attributeName = p.attributeName;
  form.attributeValue = p.attributeValue;
  form.confidence = p.confidence;
  dialogTitle.value = "编辑画像";
  dialogVisible.value = true;
}

function closeModal() {
  dialogVisible.value = false;
  editingProfile.value = null;
}

async function handleSubmit() {
  if (editingProfile.value) {
    await updateProfile(editingProfile.value.id, {
      attributeValue: form.attributeValue,
      confidence: form.confidence,
    });
  } else {
    await createProfile({
      category: form.category,
      attributeName: form.attributeName,
      attributeValue: form.attributeValue,
      confidence: form.confidence,
    });
  }
  closeModal();
}

async function handleDelete(id: string) {
  try {
    await ElMessageBox.confirm("确定删除这条画像？", "删除确认", {
      confirmButtonText: "确定",
      cancelButtonText: "取消",
      type: "warning",
    });
    await deleteProfile(id);
  } catch {}
}
</script>

<style scoped>
.profile-page {
  max-width: 100%;
}
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}
.page-header h2 {
  margin: 0;
  font-size: 24px;
  line-height: 1.25;
  color: var(--ac-color-text-primary);
}
.header-actions {
  display: flex;
  gap: 12px;
}

.profile-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 12px;
}
.profile-card {
  display: flex;
  flex-direction: column;
  min-height: 156px;
  padding: 16px;
  border: 1px solid var(--ac-color-border-light);
  border-radius: var(--radius-sm);
  background: var(--ac-color-surface);
  transition:
    border-color var(--ac-transition-fast),
    background-color var(--ac-transition-fast);
}
.profile-card:hover {
  border-color: var(--ac-color-border-strong);
}
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  margin-bottom: 14px;
}
.category-badge {
  color: var(--ac-color-text-muted);
  font-size: 12px;
  font-weight: 500;
}
.card-actions {
  display: flex;
  flex: 0 0 auto;
  gap: 0;
  margin-right: -6px;
}
.card-action-button {
  width: 28px;
  height: 28px;
  padding: 0;
  border-radius: 6px;
  color: var(--ac-color-text-muted);
  transition:
    background-color var(--ac-transition-fast),
    color var(--ac-transition-fast);
}
.card-action-button:hover,
.card-action-button:focus-visible {
  background: var(--ac-color-bg-hover);
  color: var(--ac-color-primary);
}
.card-action-button--danger:hover,
.card-action-button--danger:focus-visible {
  background: var(--ac-color-danger-bg);
  color: var(--ac-color-danger);
}

.card-body {
  flex: 1;
  min-width: 0;
  margin-bottom: 14px;
}
.attr-name {
  margin: 0 0 5px;
  color: var(--ac-color-text-muted);
  font-size: 12px;
  font-weight: 500;
  line-height: 1.4;
}
.attr-value {
  margin: 0;
  color: var(--ac-color-text-primary);
  font-size: 15px;
  font-weight: 500;
  line-height: 1.6;
  overflow-wrap: anywhere;
}
.card-footer {
  display: flex;
  align-items: center;
  gap: 10px;
  padding-top: 12px;
  border-top: 1px solid var(--ac-color-border-light);
}
.confidence-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
}
.confidence-track {
  flex: 1;
  height: 4px;
  overflow: hidden;
  border-radius: 2px;
  background: var(--ac-color-border-light);
}
.confidence-fill {
  height: 100%;
  border-radius: inherit;
  background: var(--ac-color-primary);
  transition: width var(--ac-transition-normal);
}
.confidence-text {
  min-width: 32px;
  color: var(--ac-color-text-muted);
  font-family: var(--ac-font-family-mono);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  text-align: right;
}
.source-info {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
  width: 24px;
  height: 24px;
  border-radius: 6px;
  color: var(--ac-color-text-muted);
  cursor: help;
  transition:
    background-color var(--ac-transition-fast),
    color var(--ac-transition-fast);
}
.source-info .el-icon {
  font-size: 14px;
}
.source-info:hover {
  background: var(--ac-color-bg-hover);
  color: var(--ac-color-primary);
}
.empty-state {
  grid-column: 1 / -1;
  text-align: center;
  padding: 48px;
  color: var(--ac-color-text-muted);
}
.loading {
  text-align: center;
  padding: 48px;
  color: var(--ac-color-text-muted);
}
.btn {
  padding: 8px 16px;
  border: 1px solid var(--ac-color-border);
  border-radius: 6px;
  background: var(--ac-color-surface);
  color: var(--ac-color-text);
  cursor: pointer;
  font-size: 14px;
}
.btn-primary {
  background: var(--ac-color-primary);
  color: var(--ac-color-text-on-primary);
  border-color: var(--ac-color-primary);
}

@media (max-width: 768px) {
  .page-header {
    align-items: flex-start;
    gap: 14px;
  }
  .header-actions {
    flex-wrap: wrap;
    justify-content: flex-end;
  }
  .profile-grid {
    grid-template-columns: 1fr;
  }
  .profile-card {
    min-height: 148px;
  }
}
</style>
