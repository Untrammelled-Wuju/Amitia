<template>
  <div class="cleanup-view">
    <div class="page-header">
      <h2>聊天记录清理</h2>
      <p class="page-desc">管理聊天数据的存储空间，清理旧数据以释放数据库空间</p>
    </div>

    <!-- Stat Cards -->
    <div class="stat-grid">
      <div class="stat-card">
        <div class="stat-label">总会话数</div>
        <div class="stat-value">{{ stats.totalConversations }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">总消息数</div>
        <div class="stat-value">{{ stats.totalMessages }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">数据库大小</div>
        <div class="stat-value">{{ stats.dbSize }}</div>
      </div>
    </div>

    <!-- Step 1: Select Cleanup Conditions -->
    <el-card class="section-card">
      <template #header>
        <span class="card-title">第一步：选择清理条件</span>
      </template>
      <el-form :model="form" label-width="120px" label-position="top" class="cleanup-form">
        <el-row :gutter="20">
          <el-col :xs="24" :sm="12">
            <el-form-item label="清理此日期之前的数据">
              <el-date-picker
                v-model="form.beforeDate"
                type="date"
                placeholder="选择日期"
                value-format="YYYY-MM-DD"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12">
            <el-form-item label="或清理超过 N 天的数据">
              <el-input-number
                v-model="form.olderThanDays"
                :min="1"
                :max="3650"
                placeholder="如 90"
                style="width: 100%"
              />
              <span class="form-hint">留空则不按天数过滤</span>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="20">
          <el-col :xs="24" :sm="12">
            <el-form-item label="按渠道筛选">
              <el-select
                v-model="form.channels"
                multiple
                placeholder="不选择则清理所有渠道"
                style="width: 100%"
              >
                <el-option label="Web" value="web" />
                <el-option label="微信" value="wechat" />
                <el-option label="桌面端" value="desktop" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12">
            <el-form-item label="按来源筛选">
              <el-select
                v-model="form.sources"
                multiple
                placeholder="不选择则清理所有来源"
                style="width: 100%"
              >
                <el-option label="手动" value="manual" />
                <el-option label="导入" value="import" />
                <el-option label="微信" value="wechat" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="20">
          <el-col :xs="24" :sm="12">
            <el-form-item>
              <el-checkbox v-model="form.includeMemories">同时清理关联的记忆数据</el-checkbox>
              <div class="form-hint">默认不清理记忆，除非你明确需要</div>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item>
          <el-button
            type="primary"
            :loading="previewLoading"
            @click="previewCleanup"
          >
            预览清理结果
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- Step 2: Preview Results -->
    <el-card v-if="previewResult" class="section-card preview-card">
      <template #header>
        <span class="card-title">第二步：确认清理范围</span>
      </template>
      <div class="preview-stats">
        <div class="preview-stat">
          <div class="preview-stat-label">待清理会话</div>
          <div class="preview-stat-value">{{ previewResult.conversationCount }}</div>
        </div>
        <div class="preview-stat">
          <div class="preview-stat-label">待清理消息</div>
          <div class="preview-stat-value">{{ previewResult.messageCount }}</div>
        </div>
        <div class="preview-stat">
          <div class="preview-stat-label">估算释放空间</div>
          <div class="preview-stat-value">{{ previewResult.estimatedSize }}</div>
        </div>
        <div v-if="previewResult.memoryCount > 0" class="preview-stat warn">
          <div class="preview-stat-label">关联记忆</div>
          <div class="preview-stat-value">{{ previewResult.memoryCount }}</div>
        </div>
      </div>
      <el-alert
        type="warning"
        title="清理前将自动备份数据库"
        :closable="false"
        show-icon
        style="margin-bottom: 16px"
      />
      <div class="confirm-section">
        <div class="confirm-row">
          <span class="confirm-label">输入「确认清理」以继续：</span>
          <el-input
            v-model="confirmText"
            placeholder='输入"确认清理"'
            style="width: 200px"
            class="confirm-input"
          />
        </div>
        <el-button
          type="danger"
          :disabled="confirmText !== '确认清理'"
          :loading="confirmLoading"
          @click="executeCleanup"
          class="confirm-btn"
        >
          确认清理
        </el-button>
      </div>
    </el-card>

    <!-- Step 3: Cleanup Result -->
    <el-card v-if="cleanupResult" class="section-card result-card">
      <template #header>
        <span class="card-title">清理完成</span>
      </template>
      <div class="cleanup-report">
        <div class="report-item">
          <span class="report-label">已删除会话：</span>
          <span class="report-value">{{ cleanupResult.deletedConversations }}</span>
        </div>
        <div class="report-item">
          <span class="report-label">已删除消息：</span>
          <span class="report-value">{{ cleanupResult.deletedMessages }}</span>
        </div>
        <div v-if="cleanupResult.deletedMemories > 0" class="report-item">
          <span class="report-label">已删除记忆：</span>
          <span class="report-value">{{ cleanupResult.deletedMemories }}</span>
        </div>
        <div class="report-item">
          <span class="report-label">备份名称：</span>
          <span class="report-value mono">{{ cleanupResult.backupName }}</span>
        </div>
      </div>
      <el-alert
        type="success"
        title="数据已备份至 data/backups/ 目录"
        :closable="false"
        show-icon
        style="margin-top: 12px"
      />
      <div style="margin-top: 16px">
        <el-button
          :loading="vacuumLoading"
          @click="runVacuum"
        >
          优化数据库空间
        </el-button>
        <span class="form-hint">清理后建议执行 VACUUM 回收磁盘空间</span>
      </div>
    </el-card>

    <!-- Vacuum Result -->
    <el-card v-if="vacuumResult" class="section-card vacuum-card">
      <template #header>
        <span class="card-title">数据库优化结果</span>
      </template>
      <div class="cleanup-report">
        <div class="report-item">
          <span class="report-label">优化前：</span>
          <span class="report-value">{{ vacuumResult.sizeBeforeFormatted }}</span>
        </div>
        <div class="report-item">
          <span class="report-label">优化后：</span>
          <span class="report-value">{{ vacuumResult.sizeAfterFormatted }}</span>
        </div>
        <div class="report-item highlight">
          <span class="report-label">释放空间：</span>
          <span class="report-value">{{ vacuumResult.freedFormatted }}</span>
        </div>
      </div>
    </el-card>
  
    <!-- Database Migration Status -->
    <el-card class="section-card migration-card">
      <template #header>
        <div class="migration-header">
          <span class="card-title">数据库版本</span>
          <el-tag v-if="migrationStatus" :type="migrationStatus.safeMode ? 'danger' : 'success'" size="small">
            {{ migrationStatus.safeMode ? '安全模式' : '正常' }}
          </el-tag>
        </div>
      </template>

      <div v-if="!migrationStatus" class="migration-loading">
        加载中...
      </div>

      <div v-else>
        <div class="migration-stats">
          <div class="migration-stat-item">
            <span class="ms-label">当前版本</span>
            <span class="ms-value">v{{ migrationStatus.currentVersion }}</span>
          </div>
          <div class="migration-stat-item">
            <span class="ms-label">迁移总数</span>
            <span class="ms-value">{{ migrationStatus.totalMigrations }}</span>
          </div>
          <div class="migration-stat-item">
            <span class="ms-label">已应用</span>
            <span class="ms-value">{{ migrationStatus.appliedCount }}</span>
          </div>
          <div class="migration-stat-item" v-if="migrationStatus.pendingCount > 0">
            <span class="ms-label">待应用</span>
            <span class="ms-value pending">{{ migrationStatus.pendingCount }}</span>
          </div>
        </div>

        <!-- Safe mode warning -->
        <el-alert
          v-if="migrationStatus.safeMode"
          type="error"
          title="数据库处于安全模式"
          :closable="false"
          show-icon
          style="margin-top: 12px"
        >
          <template #default>
            <p style="margin: 0 0 8px">部分迁移执行失败，系统以降级模式运行。建议从备份恢复数据库。</p>
            <p style="margin: 0; font-size: 12px; color: var(--el-text-color-secondary)">
              备份位置：data/backups/ 目录
            </p>
          </template>
        </el-alert>

        <!-- Last migration info -->
        <div v-if="migrationStatus.lastMigration" class="migration-last" style="margin-top: 12px">
          <span class="report-label">最近迁移：</span>
          <span class="report-value">{{ migrationStatus.lastMigration.name }}</span>
          <span class="form-hint">
            ({{ migrationStatus.lastMigration.success ? '成功' : '失败' }}，
            {{ migrationStatus.lastMigration.executed_at || '未知' }})
          </span>
        </div>

        <!-- Migration history -->
        <div v-if="migrationStatus.history && migrationStatus.history.length > 0" style="margin-top: 16px">
          <div class="migration-history-title">迁移历史</div>
          <el-table :data="migrationStatus.history" size="small" style="width: 100%">
            <el-table-column prop="version" label="版本" width="70" />
            <el-table-column prop="name" label="名称" min-width="140" />
            <el-table-column label="状态" width="70">
              <template #default="{ row }">
                <span :style="{ color: row.success ? 'var(--el-color-success)' : 'var(--el-color-danger)', fontSize: '12px' }">
                  {{ row.success ? '成功' : '失败' }}
                </span>
              </template>
            </el-table-column>
            <el-table-column prop="executed_at" label="执行时间" width="160">
              <template #default="{ row }">
                <span style="font-size: 12px; color: var(--el-text-color-secondary)">
                  {{ row.executed_at || '—' }}
                </span>
              </template>
            </el-table-column>
            <el-table-column prop="error_message" label="错误" min-width="120" show-overflow-tooltip>
              <template #default="{ row }">
                <span v-if="row.error_message" style="font-size: 11px; color: var(--el-color-danger)">
                  {{ row.error_message }}
                </span>
                <span v-else style="color: var(--el-text-color-placeholder)">—</span>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <!-- Manual check button -->
        <div class="migration-actions" v-if="migrationStatus.needsMigration">
          <el-alert
            type="warning"
            title="有未执行的迁移，启动时将自动应用"
            :closable="false"
            show-icon
            style="margin-bottom: 8px"
          />
          <div class="confirm-row">
            <el-input
              v-model="migCheckConfirm"
              placeholder='输入"确认检查"以手动触发'
              style="width: 220px"
              size="small"
            />
            <el-button
              size="small"
              :disabled="migCheckConfirm !== '确认检查'"
              :loading="migChecking"
              @click="checkMigrations"
            >
              手动检查迁移
            </el-button>
          </div>
        </div>
      </div>
    </el-card>

    <!-- Encrypted Backup Section -->
    <el-card class="section-card backup-card">
      <template #header>
        <span class="card-title">加密备份</span>
      </template>

      <el-alert
        type="warning"
        title="忘记密码无法恢复"
        :closable="false"
        show-icon
        style="margin-bottom: 16px"
      >
        <template #default>
          <p style="margin: 0; font-size: 13px">加密备份使用 AES-256-GCM 加密。密码不会存储在系统中，遗忘后将永久无法解密备份数据。</p>
        </template>
      </el-alert>

      <!-- Create Encrypted Backup -->
      <div class="backup-create">
        <div class="backup-create-row">
          <el-input
            v-model="backupPassword"
            type="password"
            show-password
            placeholder="设置备份密码（至少4位）"
            style="width: 240px"
          />
          <el-button
            type="primary"
            :disabled="!backupPassword || backupPassword.length < 4"
            :loading="backupCreating"
            @click="createEncryptedBackup"
          >
            创建加密备份
          </el-button>
        </div>
      </div>

      <!-- Backup List -->
      <div v-if="backupList.length > 0" style="margin-top: 20px">
        <div class="migration-history-title">备份列表</div>
        <el-table :data="backupList" size="small" style="width: 100%">
          <el-table-column prop="name" label="名称" min-width="160" />
          <el-table-column label="加密" width="70">
            <template #default="{ row }">
              <span :style="{ color: row.encrypted ? 'var(--el-color-success)' : 'var(--el-text-color-placeholder)', fontSize: '12px' }">
                {{ row.encrypted ? '是' : '否' }}
              </span>
            </template>
          </el-table-column>
          <el-table-column prop="sizeFormatted" label="大小" width="80" />
          <el-table-column label="时间" width="160">
            <template #default="{ row }">
              <span style="font-size: 12px; color: var(--el-text-color-secondary)">
                {{ row.createdAt?.slice(0, 19) || '—' }}
              </span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="140" fixed="right">
            <template #default="{ row }">
              <el-button
                v-if="row.encrypted"
                size="small"
                @click="openRestoreDialog(row)"
              >
                恢复
              </el-button>
              <span v-else style="font-size: 12px; color: var(--el-text-color-placeholder)">—</span>
            </template>
          </el-table-column>
        </el-table>
      </div>
      <div v-else-if="backupListLoaded" style="margin-top: 12px; font-size: 13px; color: var(--el-text-color-placeholder)">
        暂无备份
      </div>
    </el-card>

    <!-- Restore Dialog -->
    <el-dialog
      v-model="restoreDialogVisible"
      title="恢复加密备份"
      width="500px"
      :close-on-click-modal="false"
    >
      <template v-if="restoreTarget">
        <div class="restore-info">
          <div class="report-row"><span>备份名称：</span><strong>{{ restoreTarget.name }}</strong></div>
          <div class="report-row"><span>创建时间：</span><span>{{ restoreTarget.createdAt?.slice(0, 19) || '—' }}</span></div>
          <div class="report-row"><span>大小：</span><span>{{ restoreTarget.sizeFormatted }}</span></div>
        </div>

        <el-alert
          type="warning"
          title="恢复将覆盖当前数据库"
          :closable="false"
          show-icon
          style="margin: 12px 0"
        >
          <template #default>
            <p style="margin: 0; font-size: 12px">恢复前将自动备份当前数据至 data/backups/ 目录。恢复完成后需要重启 Core 服务。</p>
          </template>
        </el-alert>

        <el-form label-position="top" style="margin-top: 12px">
          <el-form-item label="备份密码">
            <el-input
              v-model="restorePassword"
              type="password"
              show-password
              placeholder="输入备份时设置的密码"
            />
          </el-form-item>
          <el-form-item>
            <el-button
              type="primary"
              :disabled="!restorePassword"
              :loading="restoreVerifying"
              @click="verifyRestore"
            >
              验证备份
            </el-button>
          </el-form-item>
        </el-form>

        <!-- Verify Result -->
        <div v-if="restoreVerifyResult" style="margin-top: 12px">
          <el-alert
            v-if="restoreVerifyResult.valid"
            type="success"
            title="备份有效且兼容"
            :closable="false"
            show-icon
          />
          <el-alert
            v-else
            type="error"
            title="备份验证未通过"
            :closable="false"
            show-icon
          >
            <template #default>
              <ul style="margin: 4px 0; padding-left: 16px; font-size: 13px">
                <li v-for="e in restoreVerifyResult.errors" :key="e">{{ e }}</li>
              </ul>
            </template>
          </el-alert>
          <div v-if="restoreVerifyResult.warnings?.length" style="margin-top: 8px">
            <el-alert
              type="warning"
              :title="restoreVerifyResult.warnings.join('; ')"
              :closable="false"
              show-icon
            />
          </div>
        </div>

        <!-- Execute Restore -->
        <div v-if="restoreVerifyResult?.valid" style="margin-top: 16px">
          <el-divider />
          <div class="confirm-row" style="margin-bottom: 8px">
            <span style="font-size: 13px">输入「确认恢复」以执行：</span>
            <el-input
              v-model="restoreConfirmText"
              placeholder='输入"确认恢复"'
              style="width: 160px"
              size="small"
            />
          </div>
          <el-button
            type="danger"
            :disabled="restoreConfirmText !== '确认恢复'"
            :loading="restoreExecuting"
            @click="executeRestore"
          >
            确认恢复
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from "vue"
import { ElMessage } from "element-plus"
import { apiClient } from "../../composables/useApi"

// ---- Stats ----
const stats = reactive({
  totalConversations: 0,
  totalMessages: 0,
  dbSize: "计算中...",
})

// ---- Form ----
const form = reactive({
  beforeDate: "" as string,
  olderThanDays: null as number | null,
  channels: [] as string[],
  sources: [] as string[],
  includeMemories: false,
})

// ---- Preview ----
const previewLoading = ref(false)
const previewResult = ref<any>(null)

// ---- Confirm ----
const confirmText = ref("")
const confirmLoading = ref(false)
const cleanupResult = ref<any>(null)

// ---- Vacuum ----
const vacuumLoading = ref(false)
const vacuumResult = ref<any>(null)

// ---- Migration ----
const migrationStatus = ref<any>(null)
const migChecking = ref(false)
const migCheckConfirm = ref("")

// ---- Encrypted Backup ----
const backupPassword = ref("")
const backupCreating = ref(false)
const backupList = ref<any[]>([])
const backupListLoaded = ref(false)
const restoreDialogVisible = ref(false)
const restoreTarget = ref<any>(null)
const restorePassword = ref("")
const restoreVerifyResult = ref<any>(null)
const restoreVerifying = ref(false)
const restoreExecuting = ref(false)
const restoreConfirmText = ref("")

onMounted(async () => {
  await loadStats()
  await loadMigrations()
  await loadBackups()
})

async function loadStats() {
  try {
    const res = await apiClient.get("/api/chats/stats")
    const d = res.data?.data || res.data
    stats.totalConversations = d?.totalConversations ?? 0

    // Get total messages and db size from cleanup/preview with empty filters
    const emptyRes = await apiClient.post("/api/chats/cleanup/preview", {
      channels: [],
      sources: [],
    })
    const ed = emptyRes.data?.data || emptyRes.data
    stats.totalMessages = ed?.messageCount ?? 0

    // Try to get db size via vacuum endpoint
    try {
      const vRes = await apiClient.post("/api/chats/cleanup/vacuum")
      const vd = vRes.data?.data || vRes.data
      if (vd?.sizeAfterFormatted) {
        stats.dbSize = vd.sizeBeforeFormatted || vd.sizeAfterFormatted
      }
    } catch {
      stats.dbSize = "--"
    }
  } catch {
    stats.totalConversations = 0
    stats.totalMessages = 0
    stats.dbSize = "--"
  }
}

async function loadMigrations() {
  try {
    const res = await apiClient.get("/api/storage/migrations")
    const d = res.data?.data || res.data
    migrationStatus.value = d
    if (d?.currentVersion) {
      (stats as any).dbVersion = "v" + d.currentVersion
    }
  } catch {
    migrationStatus.value = null
  }
}

async function checkMigrations() {
  if (migCheckConfirm.value !== "确认检查") return
  migChecking.value = true
  try {
    const res = await apiClient.post("/api/storage/migrations/check", {
      confirmText: "确认检查",
    })
    const d = res.data?.data || res.data
    migrationStatus.value = d
    migCheckConfirm.value = ""
    ElMessage.success(d.message || "检查完成")
  } catch (err: any) {
    ElMessage.error("检查失败: " + (err.response?.data?.message || err.message))
  } finally {
    migChecking.value = false
  }
}

async function loadBackups() {
  try {
    const res = await apiClient.get("/api/storage/backups")
    backupList.value = res.data?.data || res.data || []
    backupListLoaded.value = true
  } catch {
    backupList.value = []
    backupListLoaded.value = true
  }
}

async function createEncryptedBackup() {
  if (!backupPassword.value || backupPassword.value.length < 4) return
  backupCreating.value = true
  try {
    const res = await apiClient.post("/api/storage/backup/encrypted", {
      password: backupPassword.value,
    })
    const d = res.data?.data || res.data
    backupPassword.value = ""
    ElMessage.success("加密备份创建成功: " + d.backupName)
    await loadBackups()
  } catch (err: any) {
    ElMessage.error("创建失败: " + (err.response?.data?.message || err.message))
  } finally {
    backupCreating.value = false
  }
}

function openRestoreDialog(backup: any) {
  restoreTarget.value = backup
  restorePassword.value = ""
  restoreVerifyResult.value = null
  restoreConfirmText.value = ""
  restoreDialogVisible.value = true
}

async function verifyRestore() {
  if (!restorePassword.value || !restoreTarget.value) return
  restoreVerifying.value = true
  restoreVerifyResult.value = null
  try {
    const res = await apiClient.post("/api/storage/restore/verify", {
      backupName: restoreTarget.value.name,
      password: restorePassword.value,
    })
    const d = res.data?.data || res.data
    restoreVerifyResult.value = d
    if (d.valid) {
      ElMessage.success("验证通过")
    }
  } catch (err: any) {
    const msg = err.response?.data?.detail || err.response?.data?.message || err.message
    restoreVerifyResult.value = { valid: false, errors: [msg], warnings: [] }
    ElMessage.error("验证失败: " + msg)
  } finally {
    restoreVerifying.value = false
  }
}

async function executeRestore() {
  if (restoreConfirmText.value !== "确认恢复" || !restoreTarget.value || !restorePassword.value) return
  restoreExecuting.value = true
  try {
    const res = await apiClient.post("/api/storage/restore/encrypted", {
      backupName: restoreTarget.value.name,
      password: restorePassword.value,
      confirmText: "确认恢复",
    })
    const d = res.data?.data || res.data
    ElMessage.success(d.message || "恢复完成")
    restoreDialogVisible.value = false
  } catch (err: any) {
    ElMessage.error("恢复失败: " + (err.response?.data?.message || err.message))
  } finally {
    restoreExecuting.value = false
  }
}

async function previewCleanup() {
  previewLoading.value = true
  previewResult.value = null
  cleanupResult.value = null
  vacuumResult.value = null
  confirmText.value = ""
  try {
    const payload: any = {}
    if (form.beforeDate) payload.beforeDate = form.beforeDate
    if (form.olderThanDays) payload.olderThanDays = form.olderThanDays
    if (form.channels.length > 0) payload.channels = form.channels
    if (form.sources.length > 0) payload.sources = form.sources
    payload.includeMemories = form.includeMemories

    const res = await apiClient.post("/api/chats/cleanup/preview", payload)
    const d = res.data?.data || res.data
    previewResult.value = d
    ElMessage.success(`预览完成：${d.conversationCount} 个会话，${d.messageCount} 条消息`)
  } catch (err: any) {
    ElMessage.error("预览失败: " + (err.response?.data?.message || err.message))
  } finally {
    previewLoading.value = false
  }
}

async function executeCleanup() {
  if (confirmText.value !== "确认清理" || !previewResult.value?.previewId) return
  confirmLoading.value = true
  try {
    const res = await apiClient.post("/api/chats/cleanup/confirm", {
      previewId: previewResult.value.previewId,
      confirmText: "确认清理",
    })
    const d = res.data?.data || res.data
    cleanupResult.value = d
    ElMessage.success("清理完成")
  } catch (err: any) {
    ElMessage.error("清理失败: " + (err.response?.data?.message || err.message))
  } finally {
    confirmLoading.value = false
  }
}

async function runVacuum() {
  vacuumLoading.value = true
  try {
    const res = await apiClient.post("/api/chats/cleanup/vacuum")
    const d = res.data?.data || res.data
    vacuumResult.value = d
    ElMessage.success(`优化完成，释放 ${d.freedFormatted}`)
  } catch (err: any) {
    ElMessage.error("优化失败: " + (err.response?.data?.message || err.message))
  } finally {
    vacuumLoading.value = false
  }
}
</script>

<style scoped>
.cleanup-view {
  padding: 20px;
  max-width: 800px;
}

.page-header {
  margin-bottom: 20px;
}
.page-header h2 {
  font-size: 20px;
  font-weight: 600;
  margin: 0 0 4px 0;
  color: var(--el-text-color-primary);
}
.page-desc {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  margin: 0;
}

/* Stat Cards */
.stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 12px;
  margin-bottom: 20px;
}
.stat-card {
  background: var(--el-fill-color-lighter);
  border: 1px solid var(--el-border-color-light);
  border-radius: 8px;
  padding: 16px;
}
.stat-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-bottom: 6px;
}
.stat-value {
  font-size: 22px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

/* Section Cards */
.section-card {
  margin-bottom: 16px;
  border: 1px solid var(--el-border-color-light);
}
.card-title {
  font-size: 15px;
  font-weight: 600;
}
.cleanup-form {
  margin-top: 0;
}
.form-hint {
  font-size: 12px;
  color: var(--el-text-color-placeholder);
  margin-left: 8px;
}

/* Preview Stats */
.preview-stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}
.preview-stat {
  background: var(--el-fill-color);
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
  padding: 14px;
  text-align: center;
}
.preview-stat.warn {
  border-color: var(--el-color-warning-light-5);
  background: var(--el-color-warning-light-9);
}
.preview-stat-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-bottom: 4px;
}
.preview-stat-value {
  font-size: 20px;
  font-weight: 700;
  color: var(--el-text-color-primary);
}
.preview-stat.warn .preview-stat-value {
  color: var(--el-color-warning);
}

/* Confirm */
.confirm-section {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}
.confirm-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.confirm-label {
  font-size: 14px;
  color: var(--el-text-color-regular);
}

/* Result Report */
.cleanup-report {
  font-size: 14px;
}
.report-item {
  padding: 6px 0;
  border-bottom: 1px solid var(--el-border-color-extra-light);
}
.report-item.highlight {
  font-weight: 600;
}
.report-label {
  color: var(--el-text-color-secondary);
}
.report-value {
  color: var(--el-text-color-primary);
  font-weight: 500;
}
.report-value.mono {
  font-family: monospace;
  font-size: 12px;
}

.result-card {
  border-color: var(--el-color-success-light-5);
}
.vacuum-card {
  border-color: var(--el-color-info-light-5);
}

/* Mobile responsive */
@media (max-width: 600px) {
  .cleanup-view {
    padding: 12px;
    max-width: 100%;
  }
  .stat-grid {
    grid-template-columns: 1fr 1fr;
  }
  .preview-stats {
    grid-template-columns: 1fr 1fr;
  }
  .confirm-section {
    flex-direction: column;
    align-items: flex-start;
  }
  .confirm-btn {
    width: 100%;
  }
}

/* Migration Section */
.migration-card {
  border-color: var(--el-border-color-light);
}
.migration-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.migration-stats {
  display: flex;
  gap: 24px;
}
.migration-stat-item {
  text-align: center;
}
.ms-label {
  display: block;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.ms-value {
  font-size: 18px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}
.ms-value.pending {
  color: var(--el-color-warning);
}
.migration-loading {
  font-size: 13px;
  color: var(--el-text-color-placeholder);
}
.migration-history-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--el-text-color-secondary);
  margin-bottom: 8px;
}
.migration-actions {
  margin-top: 16px;
}
@media (max-width: 600px) {
  .migration-stats {
    flex-wrap: wrap;
    gap: 12px;
  }
}

/* Encrypted Backup */
.backup-card {
  border-color: var(--el-border-color-light);
}
.backup-create-row {
  display: flex;
  align-items: center;
  gap: 12px;
}
.restore-info {
  font-size: 14px;
}
.restore-info .report-row {
  padding: 4px 0;
  border-bottom: 1px solid var(--el-border-color-extra-light);
}
.restore-info .report-row strong {
  color: var(--el-text-color-primary);
}
@media (max-width: 600px) {
  .backup-create-row {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
