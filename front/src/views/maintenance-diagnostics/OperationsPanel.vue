<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card shadow="never" class="section-card">
    <template #header>
      <span class="section-title">
        <el-icon><Setting /></el-icon> 维护操作
      </span>
    </template>
    <div class="ops-grid">
      <div class="op-card">
        <div class="op-info">
          <div class="op-title">重启 Wechat Bridge</div>
          <div class="op-desc">仅重启Wechat Bridge 进程，不影响 Core 服务。适用于 Bridge 断连或不响应的情况。</div>
          <div class="op-risk high">高风险操作</div>
        </div>
        <div class="op-action">
          <el-popconfirm
            title="确定要重启 Bridge 吗？正在进行的微信对话可能会中断。"
            confirm-button-text="确认重启"
            cancel-button-text="取消"
            @confirm="handleRestartBridge"
          >
            <template #reference>
              <el-button type="warning" size="small" :loading="bridgeRestartLoading">
                {{ bridgeRestartLoading ? '重启中...' : '重启 Wechat Bridge' }}
              </el-button>
            </template>
          </el-popconfirm>
        </div>
      </div>
      <div class="op-card">
        <div class="op-info">
          <div class="op-title">重启 QQ Bridge</div>
          <div class="op-desc">仅重启 QQ Bridge 进程，不影响 Core 服务。适用于 QQ Bridge 断连或不响应的情况。</div>
          <div class="op-risk high">高风险操作</div>
        </div>
        <div class="op-action">
          <el-popconfirm
            title="确定要重启 QQ Bridge 吗？正在进行的 QQ 对话可能会中断。"
            confirm-button-text="确认重启"
            cancel-button-text="取消"
            @confirm="handleRestartQQBridge"
          >
            <template #reference>
              <el-button type="warning" size="small" :loading="qqBridgeRestartLoading">
                {{ qqBridgeRestartLoading ? '重启中...' : '重启 QQ Bridge' }}
              </el-button>
            </template>
          </el-popconfirm>
        </div>
      </div>
      <div class="op-card">
        <div class="op-info">
          <div class="op-title">重新加载配置</div>
          <div class="op-desc">重新读取 config.yaml 配置文件。部分配置需要重启 Core 服务才能完全生效。</div>
          <div class="op-risk high">高风险操作</div>
        </div>
        <div class="op-action">
          <el-popconfirm
            title="确定要重新加载配置吗？部分更改可能需要重启服务。"
            confirm-button-text="确认重载"
            cancel-button-text="取消"
            @confirm="handleReloadConfig"
          >
            <template #reference>
              <el-button type="warning" size="small" :loading="configReloadLoading">
                {{ configReloadLoading ? '重载中...' : '重载配置' }}
              </el-button>
            </template>
          </el-popconfirm>
        </div>
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { ref } from "vue"
import { ElMessage } from "element-plus"
import { Setting } from "@element-plus/icons-vue"
import { restartBridgeApi, restartQQBridgeApi, reloadConfigApi } from "./api"

const bridgeRestartLoading = ref(false)
const qqBridgeRestartLoading = ref(false)
const configReloadLoading = ref(false)

async function handleRestartBridge() {
  bridgeRestartLoading.value = true
  try {
    await restartBridgeApi()
    ElMessage.success("Bridge 重启指令已发送")
  } catch (e: any) {
    ElMessage.error("重启失败: " + (e.response?.data?.message || e.message))
  } finally {
    bridgeRestartLoading.value = false
  }
}

async function handleRestartQQBridge() {
  qqBridgeRestartLoading.value = true
  try {
    await restartQQBridgeApi()
    ElMessage.success("QQ Bridge 重启指令已发送")
  } catch (e: any) {
    ElMessage.error("QQ Bridge 重启失败: " + (e.response?.data?.message || e.message))
  } finally {
    qqBridgeRestartLoading.value = false
  }
}

async function handleReloadConfig() {
  configReloadLoading.value = true
  try {
    await reloadConfigApi()
    ElMessage.success("配置已重载")
  } catch (e: any) {
    ElMessage.error("重载失败: " + (e.response?.data?.message || e.message))
  } finally {
    configReloadLoading.value = false
  }
}
</script>

<style scoped>
.section-card {
  margin-bottom: 12px;
}
.section-title {
  font-weight: 600;
  font-size: var(--ac-font-size-sm);
  display: flex;
  align-items: center;
  gap: 6px;
}
.ops-grid {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.op-card {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px;
  border-radius: var(--ac-radius-md);
  background: var(--ac-color-bg-secondary);
}
.op-info {
  flex: 1;
  min-width: 0;
}
.op-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--ac-color-text);
}
.op-desc {
  font-size: 12px;
  color: var(--ac-color-text-muted);
  margin-top: 4px;
}
.op-risk {
  font-size: 11px;
  margin-top: 4px;
  padding: 1px 6px;
  border-radius: 3px;
  display: inline-block;
}
.op-risk.high {
  color: #f56c6c;
  background: #fef0f0;
  border: 1px solid #fde2e2;
}
.op-action {
  flex-shrink: 0;
  margin-left: 16px;
}
@media (max-width: 600px) {
  .op-card {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }
  .op-action {
    margin-left: 0;
  }
}
</style>
