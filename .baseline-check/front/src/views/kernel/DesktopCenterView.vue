<template>
  <div class="desktop-center">
    <ExtensionPageHeader
      title="桌面贡献中心"
      description="Desktop Contribution — 菜单、托盘、快捷键贡献管理与冲突解决"
      parent-title="扩展内核中心"
      parent-path="/kernel"
    >
      <template #actions>
        <el-button @click="refreshActive" :icon="Refresh" :loading="loading">刷新</el-button>
      </template>
    </ExtensionPageHeader>

    <el-tabs v-model="activeTab" class="desktop-tabs" @tab-change="onTabChange">
      <el-tab-pane label="贡献列表" name="contributions">
        <div class="tab-toolbar">
          <el-input
            v-model="extensionFilter"
            placeholder="输入扩展ID查询桌面贡献..."
            clearable
            style="width: 320px"
            :prefix-icon="Search"
            @keyup.enter="loadContributions"
            @clear="contributions = []"
          />
          <el-button type="primary" @click="loadContributions">查询</el-button>
        </div>
        <el-table :data="contributions" border v-loading="loading" style="width: 100%">
          <el-table-column label="贡献ID" min-width="200" show-overflow-tooltip>
            <template #default="{ row }">{{ row.definition.contributionId }}</template>
          </el-table-column>
          <el-table-column label="扩展ID" min-width="160" show-overflow-tooltip>
            <template #default="{ row }">{{ row.definition.extensionId }}</template>
          </el-table-column>
          <el-table-column label="桌面类型" width="120">
            <template #default="{ row }">{{ row.definition.desktopType }}</template>
          </el-table-column>
          <el-table-column label="目标" width="140" show-overflow-tooltip>
            <template #default="{ row }">{{ row.definition.target }}</template>
          </el-table-column>
          <el-table-column label="状态" width="110">
            <template #default="{ row }">
              <el-tag :type="statusTagType(row.status)" size="small">{{ row.status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="标签" min-width="140" show-overflow-tooltip>
            <template #default="{ row }">{{ row.effectiveLabel }}</template>
          </el-table-column>
          <el-table-column label="快捷键" width="140">
            <template #default="{ row }">{{ row.definition.shortcut?.accelerator || '-' }}</template>
          </el-table-column>
          <el-table-column label="操作" width="280" fixed="right">
            <template #default="{ row }">
              <el-button
                v-if="row.status !== 'enabled'"
                size="small"
                type="success"
                @click="doEnable(row)"
              >启用</el-button>
              <el-button
                v-else
                size="small"
                type="warning"
                @click="doDisable(row)"
              >禁用</el-button>
              <el-button
                v-if="row.definition.shortcut"
                size="small"
                type="primary"
                @click="doRebind(row)"
              >重绑快捷键</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loading && contributions.length === 0" description="请输入扩展ID查询桌面贡献" />
      </el-tab-pane>

      <el-tab-pane label="冲突管理" name="conflicts">
        <div class="tab-toolbar">
          <el-button size="small" @click="loadConflicts">刷新</el-button>
        </div>
        <el-table :data="conflicts" border v-loading="loading" style="width: 100%">
          <el-table-column prop="conflictId" label="冲突ID" min-width="200" show-overflow-tooltip />
          <el-table-column prop="type" label="类型" width="120" />
          <el-table-column label="严重程度" width="110">
            <template #default="{ row }">
              <el-tag :type="severityTagType(row.severity)" size="small">{{ row.severity }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="target" label="目标" width="140" show-overflow-tooltip />
          <el-table-column prop="existingContribId" label="现有贡献" min-width="180" show-overflow-tooltip />
          <el-table-column prop="newContribId" label="新贡献" min-width="180" show-overflow-tooltip />
          <el-table-column prop="accelerator" label="快捷键" width="140" />
          <el-table-column label="已解决" width="90">
            <template #default="{ row }">
              <el-tag :type="row.resolved ? 'success' : 'danger'" size="small">{{ row.resolved ? '是' : '否' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="120" fixed="right">
            <template #default="{ row }">
              <el-button
                v-if="!row.resolved"
                size="small"
                type="primary"
                @click="doResolve(row)"
              >解决</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loading && conflicts.length === 0" description="暂无冲突记录" />
      </el-tab-pane>

      <el-tab-pane label="契约与权限" name="contracts">
        <div class="tab-toolbar">
          <el-button size="small" @click="loadContractsAndPermissions">刷新</el-button>
        </div>
        <h4 class="section-title">契约列表</h4>
        <el-table :data="contracts" border size="small" v-loading="loading" style="width: 100%">
          <el-table-column prop="contractId" label="契约ID" min-width="180" show-overflow-tooltip />
          <el-table-column prop="version" label="版本" width="80" />
          <el-table-column prop="desktopType" label="桌面类型" width="120" />
          <el-table-column label="允许目标" min-width="200" show-overflow-tooltip>
            <template #default="{ row }">{{ row.allowedTargets?.join(', ') }}</template>
          </el-table-column>
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="row.status === 'active' ? 'success' : 'info'" size="small">{{ row.status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="maxItemsPerExt" label="每扩展上限" width="110" />
          <el-table-column label="需要权限" width="90">
            <template #default="{ row }">
              <el-tag :type="row.requiresPermission ? 'warning' : 'info'" size="small">{{ row.requiresPermission ? '是' : '否' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="description" label="描述" min-width="160" show-overflow-tooltip />
        </el-table>

        <h4 class="section-title">权限列表</h4>
        <el-table :data="permissions" border size="small" v-loading="loading" style="width: 100%">
          <el-table-column prop="id" label="权限ID" min-width="180" show-overflow-tooltip />
          <el-table-column prop="name" label="名称" min-width="140" />
          <el-table-column prop="category" label="类别" width="120" />
          <el-table-column label="风险等级" width="110">
            <template #default="{ row }">
              <el-tag :type="riskTagType(row.riskLevel)" size="small">{{ row.riskLevel }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="资源归属" name="resources">
        <div class="tab-toolbar">
          <el-button size="small" @click="loadResources">刷新</el-button>
        </div>
        <el-table :data="resources" border v-loading="loading" style="width: 100%">
          <el-table-column prop="extensionId" label="扩展ID" min-width="180" show-overflow-tooltip />
          <el-table-column prop="contributionId" label="贡献ID" min-width="200" show-overflow-tooltip />
          <el-table-column prop="resourceType" label="资源类型" width="140" />
          <el-table-column prop="resourceHandle" label="资源句柄" min-width="180" show-overflow-tooltip />
          <el-table-column label="获取时间" width="180">
            <template #default="{ row }">{{ formatDate(row.acquiredAt) }}</template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loading && resources.length === 0" description="暂无资源归属记录" />
      </el-tab-pane>

      <el-tab-pane label="快照" name="snapshot">
        <div class="tab-toolbar">
          <el-button size="small" @click="loadSnapshot">刷新</el-button>
        </div>
        <div v-loading="loading">
          <el-descriptions v-if="snapshot" :column="3" border>
            <el-descriptions-item label="Generation">{{ snapshot.generation }}</el-descriptions-item>
            <el-descriptions-item label="哈希">{{ snapshot.hash }}</el-descriptions-item>
            <el-descriptions-item label="创建时间">{{ formatDate(snapshot.createdAt) }}</el-descriptions-item>
          </el-descriptions>

          <h4 class="section-title">菜单树</h4>
          <el-empty v-if="!snapshot || menuTreeKeys.length === 0" description="暂无菜单项" />
          <el-collapse v-else>
            <el-collapse-item
              v-for="key in menuTreeKeys"
              :key="`menu-${key}`"
              :title="`${key} (${snapshot.menuTree[key].length})`"
              :name="`menu-${key}`"
            >
              <el-table :data="snapshot.menuTree[key]" size="small" border>
                <el-table-column label="贡献ID" min-width="180" show-overflow-tooltip>
                  <template #default="{ row }">{{ row.definition.contributionId }}</template>
                </el-table-column>
                <el-table-column label="扩展ID" min-width="140" show-overflow-tooltip>
                  <template #default="{ row }">{{ row.definition.extensionId }}</template>
                </el-table-column>
                <el-table-column label="标签" min-width="120">
                  <template #default="{ row }">{{ row.effectiveLabel }}</template>
                </el-table-column>
                <el-table-column label="状态" width="100">
                  <template #default="{ row }">
                    <el-tag :type="statusTagType(row.status)" size="small">{{ row.status }}</el-tag>
                  </template>
                </el-table-column>
              </el-table>
            </el-collapse-item>
          </el-collapse>

          <h4 class="section-title">托盘树</h4>
          <el-empty v-if="!snapshot || trayTreeKeys.length === 0" description="暂无托盘项" />
          <el-collapse v-else>
            <el-collapse-item
              v-for="key in trayTreeKeys"
              :key="`tray-${key}`"
              :title="`${key} (${snapshot.trayTree[key].length})`"
              :name="`tray-${key}`"
            >
              <el-table :data="snapshot.trayTree[key]" size="small" border>
                <el-table-column label="贡献ID" min-width="180" show-overflow-tooltip>
                  <template #default="{ row }">{{ row.definition.contributionId }}</template>
                </el-table-column>
                <el-table-column label="扩展ID" min-width="140" show-overflow-tooltip>
                  <template #default="{ row }">{{ row.definition.extensionId }}</template>
                </el-table-column>
                <el-table-column label="标签" min-width="120">
                  <template #default="{ row }">{{ row.effectiveLabel }}</template>
                </el-table-column>
                <el-table-column label="状态" width="100">
                  <template #default="{ row }">
                    <el-tag :type="statusTagType(row.status)" size="small">{{ row.status }}</el-tag>
                  </template>
                </el-table-column>
              </el-table>
            </el-collapse-item>
          </el-collapse>

          <h4 class="section-title">快捷键列表</h4>
          <el-empty v-if="!snapshot || snapshot.shortcuts.length === 0" description="暂无快捷键" />
          <el-table v-else :data="snapshot.shortcuts" border size="small">
            <el-table-column label="贡献ID" min-width="180" show-overflow-tooltip>
              <template #default="{ row }">{{ row.definition.contributionId }}</template>
            </el-table-column>
            <el-table-column label="扩展ID" min-width="140" show-overflow-tooltip>
              <template #default="{ row }">{{ row.definition.extensionId }}</template>
            </el-table-column>
            <el-table-column label="快捷键" width="160">
              <template #default="{ row }">{{ row.definition.shortcut?.accelerator || '-' }}</template>
            </el-table-column>
            <el-table-column label="全局" width="80">
              <template #default="{ row }">{{ row.definition.shortcut?.global ? '是' : '否' }}</template>
            </el-table-column>
            <el-table-column label="标签" min-width="120">
              <template #default="{ row }">{{ row.effectiveLabel }}</template>
            </el-table-column>
            <el-table-column label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="statusTagType(row.status)" size="small">{{ row.status }}</el-tag>
              </template>
            </el-table-column>
          </el-table>

          <h4 class="section-title">快照内冲突</h4>
          <el-empty v-if="!snapshot || snapshot.conflicts.length === 0" description="暂无冲突" />
          <el-table v-else :data="snapshot.conflicts" border size="small">
            <el-table-column prop="conflictId" label="冲突ID" min-width="180" show-overflow-tooltip />
            <el-table-column prop="type" label="类型" width="120" />
            <el-table-column label="严重程度" width="110">
              <template #default="{ row }">
                <el-tag :type="severityTagType(row.severity)" size="small">{{ row.severity }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="target" label="目标" width="140" show-overflow-tooltip />
            <el-table-column label="已解决" width="90">
              <template #default="{ row }">
                <el-tag :type="row.resolved ? 'success' : 'danger'" size="small">{{ row.resolved ? '是' : '否' }}</el-tag>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Refresh, Search } from "@element-plus/icons-vue";
import ExtensionPageHeader from "../extensions/components/ExtensionPageHeader.vue";
import {
  listDesktopContributions,
  enableDesktopContribution,
  disableDesktopContribution,
  rebindShortcut,
  listConflicts,
  resolveConflict,
  getDesktopSnapshot,
  listContracts,
  listDesktopPermissions,
  listDesktopResources,
} from "@/api/desktop";
import type {
  ResolvedContribution,
  ConflictRecord,
  DesktopSnapshot,
  DesktopContract,
  DesktopPermissionDef,
  ResourceOwner,
} from "@/api/desktop";

const activeTab = ref("contributions");
const loading = ref(false);
const extensionFilter = ref("");

const contributions = ref<ResolvedContribution[]>([]);
const conflicts = ref<ConflictRecord[]>([]);
const contracts = ref<DesktopContract[]>([]);
const permissions = ref<DesktopPermissionDef[]>([]);
const resources = ref<ResourceOwner[]>([]);
const snapshot = ref<DesktopSnapshot | null>(null);

const menuTreeKeys = computed<string[]>(() => {
  if (!snapshot.value) return [];
  return Object.keys(snapshot.value.menuTree);
});

const trayTreeKeys = computed<string[]>(() => {
  if (!snapshot.value) return [];
  return Object.keys(snapshot.value.trayTree);
});

function statusTagType(status: string): "success" | "info" | "warning" | "danger" {
  if (status === "enabled" || status === "active") return "success";
  if (status === "disabled" || status === "inactive" || status === "created") return "info";
  if (status === "paused" || status === "pending") return "warning";
  return "danger";
}

function severityTagType(severity: string): "success" | "info" | "warning" | "danger" {
  if (severity === "critical" || severity === "high") return "danger";
  if (severity === "medium") return "warning";
  return "info";
}

function riskTagType(risk: string): "success" | "info" | "warning" | "danger" {
  if (risk === "high" || risk === "critical") return "danger";
  if (risk === "medium") return "warning";
  return "info";
}

function formatDate(s?: string): string {
  if (!s) return "";
  try {
    return new Date(s).toLocaleString("zh-CN");
  } catch {
    return s;
  }
}

async function loadContributions() {
  if (!extensionFilter.value) {
    ElMessage.warning("请输入扩展ID");
    return;
  }
  loading.value = true;
  try {
    const data = await listDesktopContributions(extensionFilter.value);
    contributions.value = Array.isArray(data) ? data : [];
  } catch (e: any) {
    ElMessage.error("加载贡献列表失败: " + (e?.message || e));
  } finally {
    loading.value = false;
  }
}

async function loadConflicts() {
  loading.value = true;
  try {
    const data = await listConflicts();
    conflicts.value = Array.isArray(data) ? data : [];
  } catch (e: any) {
    ElMessage.error("加载冲突列表失败: " + (e?.message || e));
  } finally {
    loading.value = false;
  }
}

async function loadContractsAndPermissions() {
  loading.value = true;
  try {
    const [c, p] = await Promise.all([listContracts(), listDesktopPermissions()]);
    contracts.value = Array.isArray(c) ? c : [];
    permissions.value = Array.isArray(p) ? p : [];
  } catch (e: any) {
    ElMessage.error("加载契约与权限失败: " + (e?.message || e));
  } finally {
    loading.value = false;
  }
}

async function loadResources() {
  loading.value = true;
  try {
    const data = await listDesktopResources();
    resources.value = Array.isArray(data) ? data : [];
  } catch (e: any) {
    ElMessage.error("加载资源归属失败: " + (e?.message || e));
  } finally {
    loading.value = false;
  }
}

async function loadSnapshot() {
  loading.value = true;
  try {
    snapshot.value = await getDesktopSnapshot();
  } catch (e: any) {
    ElMessage.error("加载快照失败: " + (e?.message || e));
  } finally {
    loading.value = false;
  }
}

async function refreshActive() {
  if (activeTab.value === "contributions") {
    if (extensionFilter.value) await loadContributions();
  } else if (activeTab.value === "conflicts") await loadConflicts();
  else if (activeTab.value === "contracts") await loadContractsAndPermissions();
  else if (activeTab.value === "resources") await loadResources();
  else if (activeTab.value === "snapshot") await loadSnapshot();
}

async function onTabChange(name: string | number) {
  const tab = String(name);
  if (tab === "conflicts" && conflicts.value.length === 0) await loadConflicts();
  else if (tab === "contracts" && contracts.value.length === 0) await loadContractsAndPermissions();
  else if (tab === "resources" && resources.value.length === 0) await loadResources();
  else if (tab === "snapshot" && !snapshot.value) await loadSnapshot();
}

async function doEnable(row: ResolvedContribution) {
  try {
    await enableDesktopContribution(row.definition.contributionId);
    ElMessage.success("已启用: " + row.effectiveLabel);
    await loadContributions();
  } catch (e: any) {
    ElMessage.error("启用失败: " + (e?.message || e));
  }
}

async function doDisable(row: ResolvedContribution) {
  try {
    await disableDesktopContribution(row.definition.contributionId);
    ElMessage.success("已禁用: " + row.effectiveLabel);
    await loadContributions();
  } catch (e: any) {
    ElMessage.error("禁用失败: " + (e?.message || e));
  }
}

async function doRebind(row: ResolvedContribution) {
  try {
    const result = await ElMessageBox.prompt("请输入新的快捷键加速器", "重绑快捷键", {
      inputValue: row.definition.shortcut?.accelerator || "",
    });
    await rebindShortcut(row.definition.contributionId, result.value);
    ElMessage.success("快捷键已重绑");
    await loadContributions();
  } catch (e: any) {
    if (e !== "cancel" && e?.message !== "cancel") {
      ElMessage.error("重绑失败: " + (e?.message || e));
    }
  }
}

async function doResolve(row: ConflictRecord) {
  try {
    const result = await ElMessageBox.prompt("请输入解决方案", "解决冲突", {});
    await resolveConflict(row.conflictId, result.value);
    ElMessage.success("冲突已解决");
    await loadConflicts();
  } catch (e: any) {
    if (e !== "cancel" && e?.message !== "cancel") {
      ElMessage.error("解决失败: " + (e?.message || e));
    }
  }
}

onMounted(async () => {
  await loadSnapshot();
});
</script>

<style scoped>
.desktop-center {
  padding: 20px;
}

.desktop-tabs {
  margin-top: 20px;
}

.tab-toolbar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
  align-items: center;
}

.section-title {
  margin: 20px 0 8px 0;
  font-size: 14px;
}
</style>
