<template>
  <div class="hook-center">
    <div class="hook-header">
      <div class="header-left">
        <h2>Hook 管理中心</h2>
        <p class="subtitle">Third-Party Hook — 查看钩子点、管理贡献、监控熔断状态</p>
      </div>
      <div class="header-right">
        <el-tag :type="ready ? 'success' : 'danger'" size="large">
          {{ ready ? '服务在线' : '服务离线' }}
        </el-tag>
      </div>
    </div>

    <el-tabs v-model="activeTab" class="hook-tabs">
      <el-tab-pane label="钩子点" name="points">
        <div class="tab-toolbar">
          <el-input
            v-model="pointSearch"
            placeholder="搜索钩子点 ID 或描述"
            clearable
            style="width: 300px"
            :prefix-icon="Search"
          />
          <el-button @click="loadPoints" :icon="Refresh">刷新</el-button>
        </div>
        <el-table :data="filteredPoints" v-loading="pointsLoading" stripe>
          <el-table-column prop="id" label="钩子点 ID" width="280" />
          <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
          <el-table-column label="风险等级" width="100">
            <template #default="{ row }">
              <el-tag :type="riskTagType(row.riskLevel)" size="small">
                {{ riskLabel(row.riskLevel) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="第三方" width="80">
            <template #default="{ row }">
              <el-tag :type="row.thirdPartyAllowed ? 'success' : 'danger'" size="small">
                {{ row.thirdPartyAllowed ? '允许' : '禁止' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="支持阶段" width="200">
            <template #default="{ row }">
              <el-tag v-for="phase in row.supportedPhases" :key="phase" size="small" class="phase-tag">
                {{ phase }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="maxHandlers" label="最大处理数" width="110" />
          <el-table-column label="操作" width="140">
            <template #default="{ row }">
              <el-button size="small" @click="viewPointContributions(row.id)">
                查看贡献
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="贡献列表" name="contributions">
        <div class="tab-toolbar">
          <el-input
            v-model="contribSearch"
            placeholder="搜索贡献 ID 或扩展 ID"
            clearable
            style="width: 300px"
            :prefix-icon="Search"
          />
          <el-select v-model="contribFilter" placeholder="状态筛选" clearable style="width: 160px">
            <el-option label="全部" value="" />
            <el-option label="活跃" value="active" />
            <el-option label="已禁用" value="disabled" />
            <el-option label="熔断开启" value="circuit_open" />
            <el-option label="半开" value="half_open" />
          </el-select>
          <el-button @click="loadContributions" :icon="Refresh">刷新</el-button>
        </div>
        <el-table :data="filteredContributions" v-loading="contribLoading" stripe>
          <el-table-column prop="contributionId" label="贡献 ID" width="200" show-overflow-tooltip />
          <el-table-column prop="extensionId" label="扩展 ID" width="180" show-overflow-tooltip />
          <el-table-column prop="hookPointId" label="钩子点" width="200" show-overflow-tooltip />
          <el-table-column prop="phase" label="阶段" width="90" />
          <el-table-column prop="priority" label="优先级" width="80" />
          <el-table-column label="状态" width="120">
            <template #default="{ row }">
              <el-tag :type="stateTagType(row.effectiveState)" size="small">
                {{ stateLabel(row.effectiveState) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="熔断" width="100">
            <template #default="{ row }">
              <el-tag :type="circuitTagType(row.circuitState)" size="small">
                {{ circuitLabel(row.circuitState) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="240">
            <template #default="{ row }">
              <el-button
                size="small"
                :type="row.enabled ? 'warning' : 'success'"
                @click="toggleContribution(row)"
              >
                {{ row.enabled ? '禁用' : '启用' }}
              </el-button>
              <el-button
                size="small"
                @click="viewCircuitStats(row.contributionId)"
              >
                熔断详情
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="pointContribDialog" title="钩子点贡献" width="800px">
      <el-table :data="pointContribs" v-loading="pointContribLoading" stripe>
        <el-table-column prop="contributionId" label="贡献 ID" width="200" show-overflow-tooltip />
        <el-table-column prop="extensionId" label="扩展 ID" width="180" show-overflow-tooltip />
        <el-table-column prop="phase" label="阶段" width="90" />
        <el-table-column prop="priority" label="优先级" width="80" />
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="stateTagType(row.effectiveState)" size="small">
              {{ stateLabel(row.effectiveState) }}
            </el-tag>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <el-dialog v-model="circuitDialog" title="熔断器详情" width="500px">
      <el-descriptions v-if="circuitStats" :column="2" border>
        <el-descriptions-item label="贡献 ID">{{ circuitStats.contributionId }}</el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="circuitTagType(circuitStats.state)" size="small">
            {{ circuitLabel(circuitStats.state) }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="连续失败">{{ circuitStats.consecutiveFails }}</el-descriptions-item>
        <el-descriptions-item label="总失败">{{ circuitStats.totalFails }}</el-descriptions-item>
        <el-descriptions-item label="总成功">{{ circuitStats.totalSuccess }}</el-descriptions-item>
        <el-descriptions-item label="最后错误码">{{ circuitStats.lastFailCode || '-' }}</el-descriptions-item>
        <el-descriptions-item label="开启时间" :span="2">
          {{ circuitStats.openedAt || '-' }}
        </el-descriptions-item>
      </el-descriptions>
      <div class="dialog-footer">
        <el-button type="warning" @click="doResetCircuit">重置熔断器</el-button>
        <el-button @click="circuitDialog = false">关闭</el-button>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { Search, Refresh } from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  listHookPoints,
  listContributions,
  listContributionsByPoint,
  enableContribution,
  disableContribution,
  getCircuitStats,
  resetCircuit,
  type HookPointSummary,
  type HookContributionSummary,
  type HookCircuitSummary,
} from "./hook-api";

const ready = ref(false);
const activeTab = ref("points");

const points = ref<HookPointSummary[]>([]);
const pointsLoading = ref(false);
const pointSearch = ref("");

const contributions = ref<HookContributionSummary[]>([]);
const contribLoading = ref(false);
const contribSearch = ref("");
const contribFilter = ref("");

const pointContribDialog = ref(false);
const pointContribs = ref<HookContributionSummary[]>([]);
const pointContribLoading = ref(false);

const circuitDialog = ref(false);
const circuitStats = ref<HookCircuitSummary | null>(null);

const filteredPoints = computed(() => {
  if (!pointSearch.value) return points.value;
  const q = pointSearch.value.toLowerCase();
  return points.value.filter(
    (p) =>
      p.id.toLowerCase().includes(q) ||
      p.description.toLowerCase().includes(q),
  );
});

const filteredContributions = computed(() => {
  let list = contributions.value;
  if (contribSearch.value) {
    const q = contribSearch.value.toLowerCase();
    list = list.filter(
      (c) =>
        c.contributionId.toLowerCase().includes(q) ||
        c.extensionId.toLowerCase().includes(q),
    );
  }
  if (contribFilter.value) {
    list = list.filter((c) => c.effectiveState === contribFilter.value);
  }
  return list;
});

function riskTagType(risk: string) {
  switch (risk) {
    case "critical":
      return "danger";
    case "high":
      return "danger";
    case "medium":
      return "warning";
    case "low":
      return "success";
    default:
      return "info";
  }
}

function riskLabel(risk: string) {
  const map: Record<string, string> = {
    critical: "严重",
    high: "高",
    medium: "中",
    low: "低",
  };
  return map[risk] || risk;
}

function stateTagType(state: string) {
  switch (state) {
    case "active":
      return "success";
    case "disabled":
      return "info";
    case "circuit_open":
      return "danger";
    case "half_open":
      return "warning";
    default:
      return "info";
  }
}

function stateLabel(state: string) {
  const map: Record<string, string> = {
    active: "活跃",
    disabled: "已禁用",
    circuit_open: "熔断开启",
    half_open: "半开",
  };
  return map[state] || state;
}

function circuitTagType(state: string) {
  switch (state) {
    case "closed":
      return "success";
    case "open":
      return "danger";
    case "half_open":
      return "warning";
    default:
      return "info";
  }
}

function circuitLabel(state: string) {
  const map: Record<string, string> = {
    closed: "正常",
    open: "熔断",
    half_open: "半开",
  };
  return map[state] || state;
}

async function loadPoints() {
  pointsLoading.value = true;
  try {
    const res = await listHookPoints();
    points.value = res.points || [];
    ready.value = true;
  } catch (e: any) {
    ready.value = false;
    ElMessage.error("加载钩子点失败: " + (e?.message || e));
  } finally {
    pointsLoading.value = false;
  }
}

async function loadContributions() {
  contribLoading.value = true;
  try {
    const res = await listContributions();
    contributions.value = res.contributions || [];
    ready.value = true;
  } catch (e: any) {
    ready.value = false;
    ElMessage.error("加载贡献列表失败: " + (e?.message || e));
  } finally {
    contribLoading.value = false;
  }
}

async function viewPointContributions(pointId: string) {
  pointContribDialog.value = true;
  pointContribLoading.value = true;
  try {
    const res = await listContributionsByPoint(pointId);
    pointContribs.value = res.contributions || [];
  } catch (e: any) {
    ElMessage.error("加载钩子点贡献失败: " + (e?.message || e));
    pointContribs.value = [];
  } finally {
    pointContribLoading.value = false;
  }
}

async function toggleContribution(row: HookContributionSummary) {
  const action = row.enabled ? "禁用" : "启用";
  try {
    await ElMessageBox.confirm(
      `确认${action}贡献 ${row.contributionId}?`,
      "提示",
      { type: "warning" },
    );
    if (row.enabled) {
      await disableContribution(row.contributionId);
    } else {
      await enableContribution(row.contributionId);
    }
    ElMessage.success(`${action}成功`);
    await loadContributions();
  } catch (e: any) {
    if (e !== "cancel") {
      ElMessage.error(`${action}失败: ` + (e?.message || e));
    }
  }
}

async function viewCircuitStats(id: string) {
  circuitDialog.value = true;
  try {
    circuitStats.value = await getCircuitStats(id);
  } catch (e: any) {
    ElMessage.error("加载熔断器状态失败: " + (e?.message || e));
    circuitStats.value = null;
  }
}

async function doResetCircuit() {
  if (!circuitStats.value) return;
  try {
    await resetCircuit(circuitStats.value.contributionId);
    ElMessage.success("熔断器已重置");
    circuitStats.value = await getCircuitStats(
      circuitStats.value.contributionId,
    );
    await loadContributions();
  } catch (e: any) {
    ElMessage.error("重置失败: " + (e?.message || e));
  }
}

onMounted(() => {
  loadPoints();
  loadContributions();
});
</script>

<style scoped>
.hook-center {
  padding: 20px;
}

.hook-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 24px;
}

.header-left h2 {
  margin: 0 0 4px 0;
  font-size: 22px;
}

.subtitle {
  margin: 0;
  color: var(--el-text-color-secondary);
  font-size: 14px;
}

.header-right {
  display: flex;
  gap: 8px;
}

.hook-tabs {
  margin-top: 8px;
}

.tab-toolbar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
  align-items: center;
}

.phase-tag {
  margin-right: 4px;
  margin-bottom: 4px;
}

.dialog-footer {
  margin-top: 16px;
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}
</style>
