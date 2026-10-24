<template>
  <div class="bound-device-management">
    <DeviceCapabilityGrants v-if="grantDevice" v-model:visible="grantsOpen" :device-id="grantDevice.deviceId" :label="grantDevice.label" :devices="devices" @changed="refresh" />
    <DeviceCoreCall v-if="callDevice" v-model:visible="callOpen" :device-id="callDevice.deviceId" :label="callDevice.label" />

    <el-alert v-if="localError" :title="localError" type="warning" show-icon :closable="false" />
    <el-alert v-if="error" :title="error" type="error" show-icon :closable="false" />
    <el-card v-if="approvals.length" shadow="never">
      <template #header><strong>待批准的配对请求</strong></template>
      <div v-for="request in approvals" :key="request.requestId" class="detail-row">
        <span>{{ request.label || request.deviceId }} · {{ request.platform }}</span>
        <div>
          <el-button :loading="busy === request.requestId" @click="decidePairing(request, false)">拒绝</el-button>
          <el-button type="primary" :loading="busy === request.requestId" @click="decidePairing(request, true)">批准配对</el-button>
        </div>
      </div>
    </el-card>
    <section id="bound-devices" class="bound-devices-panel">
      <div class="panel-heading">
        <div>
          <h3>已绑定设备 <span class="device-count">{{ devices.length }}</span></h3>
          <p>{{ onlineDeviceCount }} 台设备在线，展开后可管理权限和同步。</p>
        </div>
        <el-button :loading="loading" @click="refreshAll">刷新状态</el-button>
      </div>
      <el-empty v-if="!loading && devices.length === 0" description="暂无已绑定设备" :image-size="70" />
      <div v-else class="device-list" v-loading="loading">
        <el-card v-for="device in devices" :key="device.deviceId" shadow="never" class="device-card">
          <div class="device-head">
            <div class="device-title">
              <strong>{{ device.label || device.deviceId }}</strong>
              <div class="muted">{{ device.platform }} · {{ formatTime(device.lastHeartbeat) }}</div>
            </div>
            <el-tag size="small" :type="device.trustState === 'revoked' ? 'danger' : device.presence === 'online' ? 'success' : 'info'" effect="plain">{{ device.trustState === 'revoked' ? "已撤销" : device.presence === "online" ? "在线" : "离线" }}</el-tag>
          </div>
          <details class="device-detail-toggle">
            <summary>查看详情与管理</summary>
            <div class="device-detail-body">
              <div class="detail-row"><span>Device ID</span><code>{{ device.deviceId }}</code></div>
              <div class="detail-row"><span>信任状态</span><span>{{ device.trustState || "未知" }}</span></div>
              <div class="detail-row"><span>Runtime</span><span>{{ device.runtimes?.length || 0 }}</span></div>
              <div class="detail-row"><span>同步</span><span>{{ syncLabel(device.deviceId) }}</span></div>
              <div v-if="device.coordination" class="detail-row"><span>运行模式</span><span>云端统筹</span></div>
              <div v-if="device.coordination" class="detail-row"><span>管理员</span><span>{{ device.coordination.administrator ? '已授权' : '未授权' }}</span></div>

              <div v-if="device.runtimes?.length" class="runtime-list">
                <div v-for="runtime in device.runtimes" :key="runtime.runtimeId" class="runtime-row">
                  <div>
                    <div>{{ runtime.runtimeId }}</div>
                    <small>{{ runtime.presence }}</small>
                  </div>
                  <el-button size="small" @click="probe(device.deviceId, runtime.runtimeId)">探测</el-button>
                </div>
              </div>

              <div class="card-actions">
                <el-button v-if="device.trustState === 'trusted'" size="small" :disabled="!coordinationAvailable" @click="callDevice = device; callOpen = true">通过 Core 调用</el-button>
                <el-button v-if="device.trustState === 'trusted' && (canAdminister || device.deviceId === localIdentity?.deviceId)" size="small" @click="grantDevice = device; grantsOpen = true">能力授权</el-button>
                <el-button v-if="canAdminister && device.coordination" size="small" :disabled="device.trustState !== 'trusted'" :loading="busy === device.deviceId" @click="changeAdministrator(device)">{{ device.coordination.administrator ? '撤销管理员' : '授予管理员' }}</el-button>
                <el-button size="small" :disabled="device.trustState !== 'trusted' || (!canAdminister && device.deviceId !== currentPolicy?.deviceId)" @click="loadSync(device.deviceId)">刷新同步状态</el-button>
                <el-button v-if="(canAdminister || device.deviceId === localIdentity?.deviceId) && device.deviceId !== coreConsoleDeviceId" size="small" type="danger" plain :loading="busy === device.deviceId" @click="revoke(device)">移除设备</el-button>
              </div>
            </div>
          </details>
        </el-card>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import DeviceCapabilityGrants from "@/components/DeviceCapabilityGrants.vue";
import DeviceCoreCall from "@/components/DeviceCoreCall.vue";
import { useDeviceManagement } from "@/composables/useDeviceManagement";

const {
  loading, busy, error, localError, devices, grantDevice, grantsOpen, callDevice,
  callOpen, approvals, localIdentity, currentPolicy, canAdminister,
  coreConsoleDeviceId, coordinationAvailable, onlineDeviceCount, refresh,
  refreshAll, decidePairing, loadSync, syncLabel, probe, revoke, formatTime,
  changeAdministrator,
} = useDeviceManagement();
</script>

<style scoped>
.bound-device-management { min-width: 0; display: grid; gap: 18px; margin-top: 24px; }
.bound-device-management :deep(.el-card) { border: 0; background: var(--surface-bg-elevated); box-shadow: none; }
.bound-device-management :deep(.el-card__header) { border-bottom: 0; }
.device-head, .detail-row, .runtime-row, .card-actions, .head-actions, .device-head-actions { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.device-card, .current-device-card { min-width: 0; }
.muted, small { color: var(--el-text-color-secondary); font-size: var(--ac-font-size-xs); margin-top: 4px; }
.detail-row { padding: 7px 0; font-size: var(--ac-font-size-sm); }
.detail-row > span:first-child { flex-shrink: 0; color: var(--el-text-color-secondary); }
code { min-width: 0; overflow-wrap: anywhere; max-width: 65%; text-align: right; }
.runtime-list { margin-top: 14px; display: grid; gap: 8px; }
.runtime-row { padding: 10px 12px; border-radius: 10px; background: var(--control-hover-bg); font-size: var(--ac-font-size-xs); }
.runtime-row > div { min-width: 0; overflow-wrap: anywhere; }
.card-actions, .head-actions, .device-head-actions { flex-wrap: wrap; }
.card-actions { justify-content: flex-end; margin-top: 14px; }
.bound-devices-panel { min-width: 0; }
.panel-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 16px; }
.panel-heading h3 { margin: 0; font-size: 16px; font-weight: 600; }
.panel-heading p { margin: 6px 0 0; font-size: 12px; color: var(--text-secondary); line-height: 1.5; }
.device-detail-toggle { margin-top: 14px; }
.device-detail-toggle > summary { padding: 14px 0 2px; cursor: pointer; color: var(--text-secondary); font-size: 13px; }
.device-detail-body { padding-top: 14px; }
.device-count { display: inline-flex; margin-left: 6px; font-size: 12px; font-weight: 500; color: var(--text-secondary); }
.device-list { display: grid; gap: 12px; }
.device-card { border: 0; border-radius: 14px; background: var(--workbench-bg); box-shadow: none; }
.device-card :deep(.el-card__body) { padding: 16px 18px; }
.device-title { min-width: 0; }
.device-title strong { font-size: 14px; overflow-wrap: anywhere; }
.device-head { align-items: flex-start; }
.detail-row { align-items: flex-start; }
.detail-row code { font-size: 12px; overflow-wrap: anywhere; }
.card-actions { padding-top: 16px; }
</style>
