<template>
  <div class="memory-view">
    <div class="page-header">
      <h2>记忆管理</h2>
      <el-select v-model="filterCharacterId" placeholder="筛选角色" clearable @change="fetchMemories" style="width: 200px">
        <el-option v-for="c in characters" :key="c.id" :label="c.name" :value="c.id" />
      </el-select>
      <el-button type="primary" @click="openCreate">添加记忆</el-button>
    </div>
    <el-table :data="memories" stripe style="width: 100%">
      <el-table-column prop="key" label="记忆键" width="180" />
      <el-table-column prop="value" label="记忆值" />
      <el-table-column prop="importance" label="重要性" width="100" />
      <el-table-column label="操作" width="200">
        <template #default="{ row }">
          <el-button size="small" @click="editMemory(row)">编辑</el-button>
          <el-button size="small" type="danger" @click="deleteMemory(row.id)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="showDialog" :title="editingId ? '编辑记忆' : '添加记忆'" width="500px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="角色"><el-select v-model="form.characterId" placeholder="选择角色"><el-option v-for="c in characters" :key="c.id" :label="c.name" :value="c.id" /></el-select></el-form-item>
        <el-form-item label="键"><el-input v-model="form.key" placeholder="记忆标识" /></el-form-item>
        <el-form-item label="值"><el-input v-model="form.value" type="textarea" :rows="3" placeholder="记忆内容" /></el-form-item>
        <el-form-item label="重要性"><el-slider v-model="form.importance" :max="10" show-input /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showDialog = false">取消</el-button>
        <el-button type="primary" @click="saveMemory">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue"
import axios from "axios"
import type { Memory, Character, ApiResponse } from "@/types"
import { ElMessage, ElMessageBox } from "element-plus"

const API = "http://127.0.0.1:8899"
const memories = ref<Memory[]>([])
const characters = ref<Character[]>([])
const filterCharacterId = ref("")
const showDialog = ref(false)
const editingId = ref("")
const form = ref({ characterId: "", key: "", value: "", importance: 0 })

onMounted(async () => {
  await fetchMemories()
  const { data } = await axios.get<ApiResponse<Character[]>>(API + "/api/characters")
  if (data.success && data.data) characters.value = data.data
})

async function fetchMemories() {
  const params = filterCharacterId.value ? "?characterId=" + filterCharacterId.value : ""
  const { data } = await axios.get<ApiResponse<Memory[]>>(API + "/api/memories" + params)
  if (data.success && data.data) memories.value = data.data
}

function openCreate() {
  editingId.value = ""
  form.value = { characterId: "", key: "", value: "", importance: 0 }
  showDialog.value = true
}

function editMemory(row: Memory) {
  editingId.value = row.id
  form.value = { characterId: row.characterId, key: row.key, value: row.value, importance: row.importance }
  showDialog.value = true
}

async function saveMemory() {
  if (!form.value.characterId || !form.value.key) { ElMessage.warning("请填写必填项"); return }
  if (editingId.value) {
    await axios.put(API + "/api/memories/" + editingId.value, form.value)
  } else {
    await axios.post(API + "/api/memories", form.value)
  }
  showDialog.value = false
  editingId.value = ""
  form.value = { characterId: "", key: "", value: "", importance: 0 }
  await fetchMemories()
  ElMessage.success("记忆已保存")
}

async function deleteMemory(id: string) {
  await ElMessageBox.confirm("确定删除该记忆?", "确认", { type: "warning" })
  await axios.delete(API + "/api/memories/" + id)
  await fetchMemories()
  ElMessage.success("记忆已删除")
}
</script>

<style scoped>
.memory-view { padding: 20px; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; gap: 12px; }
.page-header h2 { font-size: 18px; font-weight: 600; margin-right: auto; }
</style>
