<template>
  <div class="logs-page">
    <h2 class="page-title">系统日志</h2>

    <!-- Tab: DB Logs / Log Files -->
    <el-tabs v-model="activeTab" @tab-change="onTabChange">
      <el-tab-pane label="操作日志" name="db">
        <div class="log-toolbar">
          <el-button size="small" @click="fetchDbLogs">刷新</el-button>
          <el-button size="small" type="danger" plain @click="clearDbLogs" :disabled="dbLogs.length===0">清除</el-button>
          <span class="log-count" v-if="dbLogs.length">{{ dbLogs.length }}条</span>
        </div>
        <el-table :data="dbLogs" stripe size="small" max-height="500">
          <el-table-column prop="action" label="操作" width="150" show-overflow-tooltip />
          <el-table-column prop="targetType" label="目标" width="100" />
          <el-table-column prop="details" label="详情" show-overflow-tooltip />
          <el-table-column label="时间" width="160">
            <template #default="{row}">{{ fmtDate(row.createdAt) }}</template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="日志文件" name="files">
        <div class="files-layout">
          <div class="file-list">
            <div class="file-toolbar">
              <el-button size="small" @click="fetchLogFiles">刷新</el-button>
              <el-button size="small" type="danger" plain @click="clearLogFiles" :disabled="logFiles.length===0">清除所有</el-button>
            </div>
            <div
              v-for="f in logFiles"
              :key="f.name"
              class="file-item"
              :class="{ active: selectedFile === f.name }"
              @click="viewFile(f.name)"
            >
              <span class="fi-name">{{ f.name }}</span>
              <span class="fi-size">{{ formatSize(f.size) }}</span>
            </div>
            <el-empty v-if="logFiles.length===0" description="暂无日志文件" :image-size="40" />
          </div>
          <div class="file-content" v-if="selectedFile">
            <div class="fc-header">
              <span>{{ selectedFile }}</span>
              <span class="fc-lines">{{ fileLines }}行</span>
            </div>
            <pre class="fc-body">{{ fileContent }}</pre>
          </div>
          <div class="file-content empty" v-else>
            <el-empty description="选择左侧文件查看" :image-size="50" />
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { useApi } from "../../composables/useApi"

const { get, del } = useApi()

const activeTab = ref("db")

// DB Logs
const dbLogs = ref<any[]>([])

// Log Files
const logFiles = ref<any[]>([])
const selectedFile = ref("")
const fileContent = ref("")
const fileLines = ref(0)

async function fetchDbLogs() {
  try {
    const r = await get<any>("/api/logs/recent", { limit: 100 })
    dbLogs.value = r?.items || []
  } catch {}
}

async function clearDbLogs() {
  await ElMessageBox.confirm("确定清除所有操作日志？","提示",{type:"warning"})
  try { await del("/api/logs"); dbLogs.value = []; ElMessage.success("已清除") } catch {}
}

async function fetchLogFiles() {
  try { logFiles.value = await get<any[]>("/api/logs/files") || [] } catch {}
}

async function clearLogFiles() {
  await ElMessageBox.confirm("确定删除所有日志文件？","提示",{type:"warning"})
  try { await del("/api/logs"); logFiles.value = []; selectedFile.value = ""; fileContent.value = ""; ElMessage.success("已清除") } catch {}
}

async function viewFile(name: string) {
  selectedFile.value = name
  try {
    const r = await get<any>(`/api/logs/files/${encodeURIComponent(name)}`)
    fileContent.value = r?.content || ""
    fileLines.value = r?.lines || 0
  } catch {}
}

function formatSize(bytes: number): string {
  if (!bytes) return "0 B"
  if (bytes < 1024) return bytes + " B"
  return (bytes / 1024).toFixed(1) + " KB"
}

function fmtDate(d: string) { if(!d)return""; try{return new Date(d).toLocaleString("zh-CN")}catch{return d} }

function onTabChange() {
  if (activeTab.value === "db") fetchDbLogs()
  else fetchLogFiles()
}

onMounted(fetchDbLogs)
</script>

<style scoped>
.logs-page { }
.page-title { font-size:var(--ac-font-size-lg); font-weight:600; margin-bottom:14px; }

.log-toolbar { display:flex; align-items:center; gap:8px; margin-bottom:8px; }
.log-count { font-size:var(--ac-font-size-xs); color:var(--ac-color-text-muted); margin-left:auto; }

/* File layout */
.files-layout { display:flex; gap:14px; min-height:400px; }
.file-list { width:240px; flex-shrink:0; overflow-y:auto; border-right:1px solid var(--ac-color-border-light); padding-right:10px; }
.file-toolbar { display:flex; gap:6px; margin-bottom:8px; }

.file-item { display:flex; justify-content:space-between; padding:6px 8px; border-radius:var(--ac-radius-sm); cursor:pointer; font-size:var(--ac-font-size-sm); transition:background var(--ac-transition-fast); }
.file-item:hover { background:var(--ac-color-surface-hover); }
.file-item.active { background:var(--ac-color-primary-bg); color:var(--ac-color-primary); }
.fi-name { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; flex:1; }
.fi-size { font-size:var(--ac-font-size-xs); color:var(--ac-color-text-muted); margin-left:6px; }

.file-content { flex:1; overflow:hidden; display:flex; flex-direction:column; }
.file-content.empty { align-items:center; justify-content:center; }
.fc-header { display:flex; justify-content:space-between; padding:4px 0 6px; border-bottom:1px solid var(--ac-color-border-light); font-size:var(--ac-font-size-sm); font-weight:500; flex-shrink:0; }
.fc-lines { font-weight:400; color:var(--ac-color-text-muted); }
.fc-body { flex:1; overflow:auto; padding:8px 0; font-family:monospace; font-size:12px; line-height:1.5; white-space:pre-wrap; word-break:break-all; color:var(--ac-color-text-secondary); }

@media (max-width:768px) {
  .files-layout { flex-direction:column; }
  .file-list { width:100%; max-height:150px; border-right:none; border-bottom:1px solid var(--ac-color-border-light); padding-right:0; padding-bottom:8px; }
}
</style>
