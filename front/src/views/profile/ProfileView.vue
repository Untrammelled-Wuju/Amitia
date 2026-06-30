<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="profile-page">
    <div class="page-header">
      <h2>用户画像</h2>
      <div class="header-actions">
        <el-select v-model="filterCategory" placeholder="全部类别" clearable size="small" style="width:140px" @change="onFilterChange">
          <el-option label="全部类别" value="" />
          <el-option v-for="(label, key) in categoryMap" :key="key" :label="label" :value="key" />
        </el-select>
        <el-button type="primary" size="small" @click="openCreate">+ 新增画像</el-button>
      </div>
    </div>

    <div v-if="loading" class="loading">加载中...</div>

    <div v-else class="profile-grid">
      <div v-for="p in profiles" :key="p.id" class="profile-card" :class="'confidence-' + confidenceColor(p.confidence)">
        <div class="card-header">
          <span class="category-badge">{{ categoryLabel(p.category) }}</span>
          <div class="card-actions">
            <el-button size="small" text @click="editProfile(p)">✏️</el-button>
            <el-button size="small" text @click="handleDelete(p.id)">🗑️</el-button>
          </div>
        </div>
        <div class="card-body">
          <div class="attr-name">{{ p.attributeName }}</div>
          <div class="attr-value">{{ p.attributeValue }}</div>
        </div>
        <div class="card-footer">
          <div class="confidence-bar">
            <div class="confidence-fill" :style="{ width: p.confidence + '%' }"></div>
            <span class="confidence-text">{{ p.confidence }}%</span>
          </div>
          <div v-if="p.sourceConvId" class="source-info" :title="'来源对话: ' + p.sourceConvId">
            📎 对话追溯
          </div>
        </div>
      </div>

      <div v-if="profiles.length === 0" class="empty-state">
        暂无画像数据，开始对话后将自动提取
      </div>
    </div>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="440px" @close="closeModal" destroy-on-close>
      <el-form label-width="80px" @submit.prevent="handleSubmit">
        <el-form-item label="类别">
          <el-select v-model="form.category" style="width:100%">
            <el-option v-for="(label, key) in categoryMap" :key="key" :label="label" :value="key" />
          </el-select>
        </el-form-item>
        <el-form-item label="属性名">
          <el-input v-model="form.attributeName" placeholder="如：姓名、爱好、职业" />
        </el-form-item>
        <el-form-item label="属性值">
          <el-input v-model="form.attributeValue" placeholder="如：张三、喜欢摄影" />
        </el-form-item>
        <el-form-item label="置信度">
          <el-input-number v-model="form.confidence" :min="0" :max="100" style="width:100%" />
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
import { ref, reactive, onMounted } from "vue"
import { ElMessageBox } from "element-plus"
import { useProfile, type UserProfile } from "@/composables/useProfile"

const {
  profiles,
  loading,
  fetchProfiles,
  createProfile,
  updateProfile,
  deleteProfile,
  categoryLabel,
  confidenceColor,
} = useProfile()

const categoryMap: Record<string, string> = {
  personal_info: "个人信息",
  preference: "偏好",
  habit: "习惯",
  fear: "恐惧",
  relationship: "关系",
  health: "健康",
  plan: "计划",
}

const filterCategory = ref("")
const dialogVisible = ref(false)
const editingProfile = ref<UserProfile | null>(null)
const dialogTitle = ref("新增画像")

const form = reactive({
  category: "personal_info",
  attributeName: "",
  attributeValue: "",
  confidence: 50,
})

onMounted(() => {
  fetchProfiles()
})

function onFilterChange() {
  fetchProfiles({ category: filterCategory.value || undefined })
}

function openCreate() {
  editingProfile.value = null
  form.category = "personal_info"
  form.attributeName = ""
  form.attributeValue = ""
  form.confidence = 50
  dialogTitle.value = "新增画像"
  dialogVisible.value = true
}

function editProfile(p: UserProfile) {
  editingProfile.value = p
  form.category = p.category
  form.attributeName = p.attributeName
  form.attributeValue = p.attributeValue
  form.confidence = p.confidence
  dialogTitle.value = "编辑画像"
  dialogVisible.value = true
}

function closeModal() {
  dialogVisible.value = false
  editingProfile.value = null
}

async function handleSubmit() {
  if (editingProfile.value) {
    await updateProfile(editingProfile.value.id, {
      attributeValue: form.attributeValue,
      confidence: form.confidence,
    })
  } else {
    await createProfile({
      category: form.category,
      attributeName: form.attributeName,
      attributeValue: form.attributeValue,
      confidence: form.confidence,
    })
  }
  closeModal()
}

async function handleDelete(id: string) {
  try {
    await ElMessageBox.confirm("确定删除这条画像？", "删除确认", {
      confirmButtonText: "确定",
      cancelButtonText: "取消",
      type: "warning",
    })
    await deleteProfile(id)
  } catch {}
}
</script>

<style scoped>
.profile-page {
  padding: 24px;
  max-width: 1200px;
  margin: 0 auto;
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
}
.header-actions {
  display: flex;
  gap: 12px;
}

.profile-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}
.profile-card {
  background: var(--ac-color-bg-secondary);
  border-radius: 12px;
  padding: 16px;
  box-shadow: 0 2px 8px rgba(0,0,0,0.08);
  border-left: 4px solid #e0e0e0;
}
.profile-card.confidence-success { border-left-color: #4caf50; }
.profile-card.confidence-warning { border-left-color: #ff9800; }
.profile-card.confidence-danger { border-left-color: #f44336; }
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}
.category-badge {
  font-size: 12px;
  padding: 2px 8px;
  background: #f0f0f0;
  border-radius: 4px;
  color: #666;
}
.card-actions { display: flex; gap: 4px; }

.card-body { margin-bottom: 12px; }
.attr-name { font-size: 13px; color: var(--ac-color-text-secondary); margin-bottom: 4px; }
.attr-value { font-size: 16px; font-weight: 500; color: var(--ac-color-text-primary); }
.card-footer { display: flex; align-items: center; gap: 12px; }
.confidence-bar {
  flex: 1;
  height: 6px;
  background: #eee;
  border-radius: 3px;
  position: relative;
  overflow: hidden;
}
.confidence-fill {
  height: 100%;
  background: #4caf50;
  border-radius: 3px;
  transition: width 0.3s;
}
.confidence-text { font-size: 11px; color: #999; min-width: 36px; text-align: right; }
.source-info { font-size: 11px; color: #999; cursor: help; }
.empty-state { grid-column: 1 / -1; text-align: center; padding: 48px; color: #999; }
.loading { text-align: center; padding: 48px; color: #999; }
.btn { padding: 8px 16px; border: 1px solid #ddd; border-radius: 6px; background: #fff; cursor: pointer; font-size: 14px; }
.btn-primary { background: #1976d2; color: #fff; border-color: #1976d2; }

</style>