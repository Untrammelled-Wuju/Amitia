<template>
  <div class="bound-device-management">
    <DeviceCapabilityGrants v-if="grantDevice" v-model:visible="grantsOpen" :device-id="grantDevice.deviceId" :label="grantDevice.label" :devices="boundDevices" @changed="refresh" />

    <el-alert v-if="localError" :title="localError" type="warning" show-icon :closable="false" />
    <el-alert v-if="error" :title="error" type="error" show-icon :closable="false" />
    <el-dialog v-model="approvalDialogVisible" title="子设备配对请求" width="min(620px, 92vw)" append-to-body>
      <div class="approval-request-list">
        <div class="approval-request-header">
          <strong>待批准的配对请求</strong>
          <span>{{ approvals.length }} 台设备</span>
        </div>
        <p class="approval-request-hint">以下子设备正在请求加入当前统筹中心，请逐台确认。</p>
        <div v-for="request in approvals" :key="request.requestId" class="approval-request-row">
          <span>
            <strong>{{ request.label || '未命名设备' }}</strong>
            <small>{{ request.platform }} · {{ request.deviceId }}</small>
          </span>
          <div>
            <el-button :loading="busy === request.requestId" @click="decidePairing(request, false)">拒绝</el-button>
            <el-button type="primary" :loading="busy === request.requestId" @click="decidePairing(request, true)">批准配对</el-button>
          </div>
        </div>
      </div>
    </el-dialog>
    <el-dialog v-if="roleDomainOpen" v-model="roleDomainOpen" title="默认角色" width="min(520px, 92vw)" append-to-body>
      <div class="role-domain-body">
        <p class="role-domain-hint">数据归属跟随角色来源：使用统筹中心角色时数据保存在统筹中心，使用本机角色时数据保存在本机，历史数据不会迁移。</p>
        <el-radio-group v-model="roleDomainDraft" :disabled="roleDomainSaving" @change="switchRoleDomain(roleDomainDraft)">
          <el-radio value="local">本机角色</el-radio>
          <el-radio value="center">统筹中心角色</el-radio>
        </el-radio-group>
        <p v-if="roleDomainDraft === 'local'" class="role-domain-warning">切换为本机角色会同时关闭这台设备的中心管理员授权。</p>
        <el-select v-model="roleDomainRole" :loading="roleDomainLoading" :disabled="roleDomainSaving" clearable placeholder="不指定（调用时再选择）" style="width: 100%">
          <el-option v-for="role in roleDomainRoles" :key="role.id" :label="role.name || role.id" :value="role.id" />
        </el-select>
        <el-alert v-if="roleDomainError" :title="roleDomainError" type="error" show-icon :closable="false" />
      </div>
      <template #footer>
        <el-button @click="roleDomainOpen = false">取消</el-button>
        <el-button type="primary" :loading="roleDomainSaving" @click="saveRoleDomain">保存</el-button>
      </template>
    </el-dialog>
    <section v-if="currentPolicy" class="current-device-panel">
      <div class="panel-heading">
        <div>
          <h3>当前设备</h3>
          <p>{{ currentRoleDomain() === "center" ? "使用统筹中心角色" : "使用本机角色" }} · 默认角色 {{ roleDomainLabel() }}</p>
        </div>
        <el-button size="small" @click="openRoleDomain">更改默认角色</el-button>
      </div>
    </section>
    <section id="bound-devices" class="bound-devices-panel">
      <div class="panel-heading">
        <div>
          <h3>已绑定设备 <span class="device-count">{{ boundDevices.length }}</span></h3>
          <p>{{ onlineDeviceCount }} 台设备在线，展开后可管理权限和同步。</p>
        </div>
        <el-button :loading="loading" @click="refreshAll">刷新状态</el-button>
      </div>
      <el-empty v-if="!loading && boundDevices.length === 0" description="暂无已绑定设备" :image-size="70" />
      <div v-else class="device-list" v-loading="loading">
        <el-card v-for="device in boundDevices" :key="device.deviceId" shadow="never" class="device-card">
          <div class="device-head">
            <div class="device-title">
              <strong>{{ displayName(device) }}</strong>
              <div class="muted">{{ device.platform }} · {{ formatTime(device.lastHeartbeat) }}</div>
              <div v-if="device.pendingName" class="device-rename-hint">
                此设备已更名为：{{ device.pendingName }}
                <el-button size="small" type="primary" link :loading="busy === device.deviceId" @click="confirmDeviceName(device)">变更</el-button>
              </div>
            </div>
            <el-tag size="small" :type="device.trustState === 'revoked' ? 'danger' : isDeviceOnline(device) ? 'success' : 'info'" effect="plain">{{ device.trustState === 'revoked' ? "已撤销" : isDeviceOnline(device) ? "在线" : "离线" }}</el-tag>
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
                  <el-button v-if="canProbeRuntime(runtime)" size="small" @click="probe(device.deviceId, runtime.runtimeId)">探测</el-button>
                  <small v-else>本地运行或无活动 Mesh 连接</small>
                </div>
              </div>

              <div class="card-actions">
                <el-button v-if="device.trustState === 'trusted' && canAdminister" size="small" @click="grantDevice = device; grantsOpen = true">能力授权</el-button>
                <el-button v-if="canAdminister && device.coordination" size="small" :loading="busy === device.deviceId" @click="device.trustState !== 'trusted' ? ElMessage.warning('该设备未受信任，无法更改管理员权限。') : changeAdministrator(device)">{{ device.coordination.administrator ? '撤销管理员' : '授予管理员' }}</el-button>
                <el-button size="small" @click="device.trustState !== 'trusted' ? ElMessage.warning('该设备未受信任，无法查看同步状态。') : (!canAdminister && device.deviceId !== currentPolicy?.deviceId) ? ElMessage.warning('当前设备没有查看此设备同步状态的权限。') : loadSync(device.deviceId)">刷新同步状态</el-button>
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
import { computed, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import DeviceCapabilityGrants from "@/components/DeviceCapabilityGrants.vue";
import { useDeviceManagement } from "@/composables/useDeviceManagement";
import { canProbeRuntime, isDeviceOnline } from "@/runtime/device-presence";
import { resolveDeviceDisplayNames } from "@/runtime/device-display-name";

const {
  loading, busy, error, localError, boundDevices, grantDevice, grantsOpen,
  approvals, localIdentity, currentPolicy, canAdminister,
  coreConsoleDeviceId, coordinationAvailable, onlineDeviceCount, refresh,
  refreshAll, decidePairing, loadSync, syncLabel, probe, revoke, formatTime,
  changeAdministrator, confirmDeviceName,
  roleDomainOpen, roleDomainDraft, roleDomainRole, roleDomainRoles,
  roleDomainLoading, roleDomainSaving, roleDomainError,
  currentRoleDomain, currentRoleDomainRole, openRoleDomain, switchRoleDomain, saveRoleDomain,
} = useDeviceManagement();

function roleDomainLabel() {
  const roleId = currentRoleDomainRole();
  if (!roleId) return "未指定";
  return roleDomainRoles.value.find((role) => role.id === roleId)?.name || roleId;
}

const displayNames = computed(() => resolveDeviceDisplayNames(boundDevices.value));
function displayName(device: { deviceId: string }) {
  return displayNames.value.get(device.deviceId) || "未命名设备";
}

const approvalDialogVisible = ref(false);

watch(
  () => approvals.value.length,
  (count, previous) => {
    if (count === 0) approvalDialogVisible.value = false;
    else if (previous === 0) approvalDialogVisible.value = true;
  },
  { immediate: true },
);
</script>

<style scoped>
.bound-device-management { min-width: 0; display: grid; gap: 18px; margin-top: 24px; }
.bound-device-management :deep(.el-card) { border: 0; background: var(--surface-bg-elevated); box-shadow: none; }
.bound-device-management :deep(.el-card__header) { border-bottom: 0; }
.device-head, .detail-row, .runtime-row, .card-actions, .head-actions, .device-head-actions { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.device-card, .current-device-card { min-width: 0; }
.device-rename-hint { margin-top: 6px; display: flex; align-items: center; gap: 4px; font-size: var(--ac-font-size-xs); color: var(--el-color-primary); }
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
.role-domain-body { display: grid; gap: 12px; }
.role-domain-hint, .role-domain-warning { margin: 0; font-size: var(--ac-font-size-xs); line-height: 1.6; color: var(--el-text-color-secondary); }
.role-domain-warning { color: var(--el-color-warning); }
.current-device-panel { margin-bottom: 6px; }
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
.approval-request-list { display: grid; gap: 10px; }
.approval-request-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; font-size: 13px; }
.approval-request-header span { color: var(--text-secondary); font-size: 12px; }
.approval-request-hint { margin: 0 0 4px; color: var(--text-secondary); font-size: 12px; line-height: 1.5; }
.approval-request-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 12px 0; border-top: 1px solid var(--el-border-color-lighter); }
.approval-request-row > span { min-width: 0; overflow-wrap: anywhere; display: grid; gap: 2px; }
.approval-request-row > div { display: flex; flex-shrink: 0; gap: 8px; }
</style>
