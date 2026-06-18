<template>
  <div class="profile-page">
    <div class="page-header">
      <h2>用户画像</h2>
      <div class="header-actions">
        <select v-model="filterCategory" @change="onFilterChange">
          <option value="">全部类别</option>
          <option v-for="(label, key) in categoryMap" :key="key" :value="key">{{ label }}</option>
        </select>
        <button class="btn btn-primary" @click="showCreate = true">+ 新增画像</button>
      </div>
    </div>

    <div v-if="loading" class="loading">加载中...</div>

    <div v-else class="profile-grid">
      <div v-for="p in profiles" :key="p.id" class="profile-card" :class="'confidence-' + confidenceColor(p.confidence)">
        <div class="card-header">
          <span class="category-badge">{{ categoryLabel(p.category) }}</span>
          <div class="card-actions">
            <button class="btn-icon" @click="editProfile(p)" title="编辑">✏️</button>
            <button class="btn-icon" @click="handleDelete(p.id)" title="删除">🗑️</button>
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

    <div v-if="showCreate || editingProfile" class="modal-overlay" @click.self="closeModal">
      <div class="modal">
        <h3>{{ editingProfile ? '编辑画像' : '新增画像' }}</h3>
        <form @submit.prevent="handleSubmit">
          <div class="form-group">
            <label>类别</label>
            <select v-model="form.category" required>
              <option v-for="(label, key) in categoryMap" :key="key" :value="key">{{ label }}</option>
            </select>
          </div>
          <div class="form-group">
            <label>属性名</label>
            <input v-model="form.attributeName" required placeholder="如：姓名、爱好、职业" />
          </div>
          <div class="form-group">
            <label>属性值</label>
            <input v-model="form.attributeValue" required placeholder="如：张三、喜欢摄影" />
          </div>
          <div class="form-group">
            <label>置信度 (0-100)</label>
            <input type="number" v-model.number="form.confidence" min="0" max="100" />
          </div>
          <div class="form-actions">
            <button type="button" class="btn" @click="closeModal">取消</button>
            <button type="submit" class="btn btn-primary">保存</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from "vue"
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
const showCreate = ref(false)
const editingProfile = ref<UserProfile | null>(null)

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

function editProfile(p: UserProfile) {
  editingProfile.value = p
  form.category = p.category
  form.attributeName = p.attributeName
  form.attributeValue = p.attributeValue
  form.confidence = p.confidence
  showCreate.value = true
}

function closeModal() {
  showCreate.value = false
  editingProfile.value = null
  form.category = "personal_info"
  form.attributeName = ""
  form.attributeValue = ""
  form.confidence = 50
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
  if (confirm("确定删除这条画像？")) {
    await deleteProfile(id)
  }
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
.header-actions select {
  padding: 8px 12px;
  border: 1px solid #ddd;
  border-radius: 6px;
  background: #fff;
}
.profile-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}
.profile-card {
  background: #fff;
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
.btn-icon {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 14px;
  padding: 2px 4px;
}
.card-body { margin-bottom: 12px; }
.attr-name { font-size: 13px; color: #888; margin-bottom: 4px; }
.attr-value { font-size: 16px; font-weight: 500; color: #333; }
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
.modal-overlay {
  position: fixed; top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,0.4);
  display: flex; justify-content: center; align-items: center;
  z-index: 1000;
}
.modal {
  background: #fff;
  border-radius: 12px;
  padding: 24px;
  width: 400px;
  max-width: 90vw;
}
.modal h3 { margin: 0 0 16px; }
.form-group { margin-bottom: 12px; }
.form-group label { display: block; margin-bottom: 4px; font-size: 13px; color: #666; }
.form-group input, .form-group select {
  width: 100%; padding: 8px 12px; border: 1px solid #ddd; border-radius: 6px;
  box-sizing: border-box;
}
.form-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 16px; }
</style>