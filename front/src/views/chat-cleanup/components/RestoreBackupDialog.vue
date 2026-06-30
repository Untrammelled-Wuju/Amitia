<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-dialog
    :model-value="visible"
    title="恢复加密备份"
    width="500px"
    :close-on-click-modal="false"
    @update:model-value="emit('update:visible', $event)"
    @closed="handleClosed"
  >
    <template v-if="backup">
      <div class="restore-info">
        <div class="report-row"><span>备份名称：</span><strong>{{ backup.name }}</strong></div>
        <div class="report-row"><span>创建时间：</span><span>{{ backup.createdAt?.slice(0, 19) || '—' }}</span></div>
        <div class="report-row"><span>大小：</span><span>{{ backup.sizeFormatted }}</span></div>
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
</template>

<script setup lang="ts">
import { ref } from "vue"
import { ElMessage } from "element-plus"
import { apiClient } from "../../../composables/useApi"

const props = defineProps<{
  visible: boolean
  backup: any | null
}>()

const emit = defineEmits<{
  (e: "update:visible", value: boolean): void
}>()

const restorePassword = ref("")
const restoreVerifyResult = ref<any>(null)
const restoreVerifying = ref(false)
const restoreExecuting = ref(false)
const restoreConfirmText = ref("")

function handleClosed() {
  restorePassword.value = ""
  restoreVerifyResult.value = null
  restoreConfirmText.value = ""
}

async function verifyRestore() {
  if (!restorePassword.value || !props.backup) return
  restoreVerifying.value = true
  restoreVerifyResult.value = null
  try {
    const res = await apiClient.post("/api/storage/restore/verify", {
      backupName: props.backup.name,
      password: restorePassword.value,
    })
    const d = res.data?.data || res.data
    restoreVerifyResult.value = d
    if (d.valid) {
      ElMessage.success("验证通过")
    }
  } catch (err: any) {
    const msg = err.response?.data?.message || err.message
    restoreVerifyResult.value = { valid: false, errors: [msg] }
    ElMessage.error("验证失败: " + msg)
  } finally {
    restoreVerifying.value = false
  }
}

async function executeRestore() {
  if (restoreConfirmText.value !== "确认恢复" || !props.backup || !restorePassword.value) return
  restoreExecuting.value = true
  try {
    const res = await apiClient.post("/api/storage/restore/encrypted", {
      backupName: props.backup.name,
      password: restorePassword.value,
      confirmText: "确认恢复",
    })
    const d = res.data?.data || res.data
    ElMessage.success(d.message || "恢复完成")
    emit("update:visible", false)
  } catch (err: any) {
    ElMessage.error("恢复失败: " + (err.response?.data?.message || err.message))
  } finally {
    restoreExecuting.value = false
  }
}
</script>

<style scoped>
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
.confirm-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
