<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <main class="pet-installations">
    <ExtensionPageHeader
      title="安装管理"
      description="管理已安装的桌宠、启用停用、运行配置与动作"
      grandparent-title="创意工坊"
      grandparent-path="/creative-workshop"
      parent-title="桌宠"
      parent-path="/creative-workshop/pet"
    >
      <template #actions>
        <el-button :icon="Back" @click="goBack">返回任务列表</el-button>
        <el-button type="success" :icon="Plus" @click="openInstallDialog">安装桌宠</el-button>
        <el-button
          v-if="hasActiveInstallation"
          type="warning"
          plain
          :loading="disablingAll"
          @click="confirmDisableAll"
          >停用当前桌宠</el-button
        >
        <el-button :icon="Refresh" :loading="loading" @click="loadList"
          >刷新</el-button
        >
      </template>
    </ExtensionPageHeader>

    <el-card shadow="never" class="summary-card">
      <div class="summary-grid">
        <div class="summary-item">
          <div class="summary-label">安装总数</div>
          <div class="summary-value">{{ installations.length }}</div>
        </div>
        <div class="summary-item">
          <div class="summary-label">当前启用</div>
          <div class="summary-value">
            <span :class="activeInstallation ? 'stat-success' : 'stat-muted'">
              {{ activeInstallation ? activeInstallation.name : "未启用" }}
            </span>
          </div>
        </div>
        <div class="summary-item">
          <div class="summary-label">已停用</div>
          <div class="summary-value">{{ disabledCount }}</div>
        </div>
        <div class="summary-item">
          <div class="summary-label">资源损坏</div>
          <div class="summary-value">
            <span :class="invalidCount > 0 ? 'stat-danger' : 'stat-muted'">
              {{ invalidCount }}
            </span>
          </div>
        </div>
      </div>
    </el-card>

    <el-card shadow="never" class="table-card">
      <el-empty
        v-if="!loading && !installations.length"
        description="暂无安装的桌宠，前往处理结果审核页安装资源包"
        :image-size="80"
      >
        <el-button type="primary" @click="goToTasks">前往任务列表</el-button>
      </el-empty>

      <div v-else class="installation-grid" v-loading="loading">
        <div
          v-for="item in installations"
          :key="item.id"
          class="installation-card"
          :class="{
            active: runtimeStatusOf(item) === 'running',
            invalid: runtimeStatusOf(item) === 'corrupted',
            uninstalled: isUninstalled(item),
          }"
        >
          <div class="card-head">
            <div class="card-title">
              <strong>{{ item.name || "未命名桌宠" }}</strong>
              <el-tag
                size="small"
                :type="runtimeStatusTagType(item)"
              >{{ runtimeStatusLabel(item) }}</el-tag>
            </div>
            <div class="card-version">v{{ item.packageVersion || "—" }}</div>
          </div>

          <div class="card-body">
            <div class="preview-box">
              <img
                v-if="previewUrlOf(item)"
                :src="previewUrlOf(item)"
                :alt="item.name"
                @error="onPreviewError(item)"
              />
              <div v-else class="preview-placeholder">
                <el-icon><Picture /></el-icon>
                <span>暂无预览</span>
              </div>
            </div>
            <div class="meta">
              <div class="meta-row">
                <span class="meta-label">绑定角色</span>
                <span class="meta-value">{{ characterLabelOf(item) }}</span>
              </div>
              <div class="meta-row">
                <span class="meta-label">默认动作</span>
                <span class="meta-value">{{ item.defaultActionKey || "—" }}</span>
              </div>
              <div class="meta-row">
                <span class="meta-label">画布尺寸</span>
                <span class="meta-value">
                  {{ item.canvasWidth || 0 }} × {{ item.canvasHeight || 0 }}
                </span>
              </div>
              <div class="meta-row">
                <span class="meta-label">安装时间</span>
                <span class="meta-value">{{ formatTime(item.installedAt) }}</span>
              </div>
              <div v-if="item.lastEnabledAt" class="meta-row">
                <span class="meta-label">最近启用</span>
                <span class="meta-value">{{ formatTime(item.lastEnabledAt) }}</span>
              </div>
              <div v-if="contentHashOf(item)" class="meta-row">
                <span class="meta-label">内容哈希</span>
                <span class="meta-value hash-value" :title="contentHashOf(item)">
                  {{ shortHash(contentHashOf(item)) }}
                </span>
              </div>
            </div>
          </div>

          <div v-if="runtimeStatusOf(item) === 'corrupted'" class="invalid-banner">
            <el-alert
              title="资源校验失败，桌宠已自动停用"
              type="error"
              :closable="false"
              show-icon
            />
          </div>

          <div class="card-foot">
            <el-button
              v-if="canEnable(item)"
              type="primary"
              size="small"
              :loading="actionTargetId === item.id"
              @click="onEnable(item)"
            >启用</el-button>
            <el-button
              v-if="canDisable(item)"
              type="warning"
              size="small"
              plain
              :loading="actionTargetId === item.id"
              @click="onDisable(item)"
            >停用</el-button>
            <el-button
              v-if="canRecenter(item)"
              size="small"
              :loading="actionTargetId === item.id"
              @click="onRecenter(item)"
            >重新定位</el-button>
            <el-button
              v-if="canResize(item)"
              size="small"
              @click="openResizeDialog(item)"
            >调整大小</el-button>
            <el-button
              v-if="canViewActions(item)"
              size="small"
              @click="openActionsDialog(item)"
            >查看动作</el-button>
            <el-button
              v-if="canChangeDefaultAction(item)"
              size="small"
              @click="openDefaultActionDialog(item)"
            >更换默认待机</el-button>
            <el-button
              v-if="canUpgrade(item)"
              size="small"
              type="primary"
              plain
              :loading="actionTargetId === item.id"
              @click="openUpgradeDialog(item)"
            >升级</el-button>
            <el-button
              v-if="canSwitch(item)"
              size="small"
              :loading="actionTargetId === item.id"
              @click="onSwitch(item)"
            >切换</el-button>
            <el-button
              v-if="canRepair(item)"
              size="small"
              type="warning"
              :loading="actionTargetId === item.id"
              @click="onRepair(item)"
            >修复</el-button>
            <el-button
              v-if="canReinstall(item)"
              size="small"
              type="warning"
              plain
              :loading="actionTargetId === item.id"
              @click="onReinstall(item)"
            >重新安装</el-button>
            <el-button
              v-if="canUninstall(item)"
              size="small"
              type="danger"
              plain
              :loading="actionTargetId === item.id"
              @click="confirmUninstall(item)"
            >卸载</el-button>
          </div>
        </div>
      </div>
    </el-card>

    <el-dialog
      v-model="resizeDialogVisible"
      title="调整桌宠大小"
      width="420px"
      destroy-on-close
    >
      <el-form label-width="120px">
        <el-form-item label="桌宠">
          <span>{{ resizeForm.name }}</span>
        </el-form-item>
        <el-form-item label="缩放比例">
          <el-input-number
            v-model="resizeForm.scale"
            :min="0.25"
            :max="4"
            :step="0.05"
            :precision="2"
            controls-position="right"
          />
          <span class="resize-hint">范围 0.25 ~ 4.00</span>
        </el-form-item>
        <el-form-item label="置顶">
          <el-switch v-model="resizeForm.alwaysOnTop" />
        </el-form-item>
        <el-form-item label="Amitia 启动时恢复">
          <el-switch v-model="resizeForm.restoreOnAppStart" />
        </el-form-item>
        <el-form-item label="待机启用">
          <el-switch v-model="resizeForm.idleEnabled" />
        </el-form-item>
        <el-form-item label="声音">
          <el-switch v-model="resizeForm.soundEnabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="resizeDialogVisible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="actionTargetId === resizeForm.id"
          @click="submitResize"
          >保存</el-button
        >
      </template>
    </el-dialog>

    <el-dialog
      v-model="actionsDialogVisible"
      title="动作列表"
      width="560px"
      destroy-on-close
    >
      <div v-loading="actionsLoading" class="actions-dialog-body">
        <el-empty
          v-if="!actionsLoading && !actionsList.length"
          description="暂无动作数据"
          :image-size="80"
        />
        <div v-else class="action-list">
          <div
            v-for="action in actionsList"
            :key="action.key"
            class="action-row"
          >
            <div class="action-row-head">
              <strong>{{ action.name || action.key }}</strong>
              <el-tag
                v-if="action.key === actionsDefaultAction"
                size="small"
                type="success"
              >当前默认</el-tag>
              <el-tag
                v-if="action.supportsDefaultIdle"
                size="small"
                type="info"
              >支持默认待机</el-tag>
            </div>
            <div class="action-row-meta">
              <span class="meta-label">Key</span>
              <span class="meta-value">{{ action.key }}</span>
            </div>
            <div class="action-row-foot">
              <el-button
                v-if="actionsInstallationStatus === 'enabled'"
                size="small"
                :loading="playActionLoadingKey === action.key"
                @click="onPlayAction(action.key)"
                >播放动作</el-button
              >
              <el-button
                v-if="
                  action.supportsDefaultIdle &&
                  action.key !== actionsDefaultAction
                "
                size="small"
                @click="onChangeDefaultAction(action.key)"
                >设为默认待机</el-button
              >
            </div>
          </div>
        </div>
      </div>
    </el-dialog>

    <el-dialog
      v-model="defaultActionDialogVisible"
      title="更换默认待机动作"
      width="420px"
      destroy-on-close
    >
      <el-form label-width="120px">
        <el-form-item label="桌宠">
          <span>{{ defaultActionForm.name }}</span>
        </el-form-item>
        <el-form-item label="默认待机">
          <el-select
            v-model="defaultActionForm.actionKey"
            placeholder="请选择支持默认待机的动作"
            style="width: 100%"
          >
            <el-option
              v-for="action in defaultIdleCandidates"
              :key="action.key"
              :label="action.name || action.key"
              :value="action.key"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="defaultActionDialogVisible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="actionTargetId === defaultActionForm.id"
          @click="submitDefaultAction"
          >保存</el-button
        >
      </template>
    </el-dialog>
  </main>

    <el-dialog
      v-model="upgradeDialogVisible"
      title="升级桌宠"
      width="420px"
      destroy-on-close
    >
      <el-form label-width="100px">
        <el-form-item label="桌宠">
          <span>{{ upgradeForm.name }}</span>
        </el-form-item>
        <el-form-item label="目标版本">
          <el-select
            v-model="upgradeForm.targetReleaseId"
            placeholder="请选择目标资源包"
            style="width: 100%"
            filterable
          >
            <el-option
              v-for="pkg in upgradeCandidates"
              :key="pkg.id"
              :label="`Release v${pkg.version}`"
              :value="pkg.id"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="upgradeDialogVisible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="actionTargetId === upgradeForm.id"
          :disabled="!upgradeForm.targetReleaseId"
          @click="submitUpgrade"
          >升级</el-button
        >
      </template>
    </el-dialog>

    <el-dialog
      v-model="installDialogVisible"
      title="安装桌宠"
      width="480px"
      destroy-on-close
    >
      <el-form label-width="100px">
        <el-form-item label="资源包">
          <el-select
            v-model="installForm.releaseId"
            placeholder="请选择资源包"
            style="width: 100%"
            filterable
            @change="onInstallPackageChange"
          >
            <el-option
              v-for="pkg in availablePackages"
              :key="pkg.id"
              :label="`Release v${pkg.version}`"
              :value="pkg.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="绑定角色">
          <el-select
            v-model="installForm.characterId"
            placeholder="请选择角色"
            style="width: 100%"
            filterable
          >
            <el-option
              v-for="char in characters"
              :key="char.id"
              :label="char.name"
              :value="String(char.id)"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="installDialogVisible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="installSubmitting"
          :disabled="!installForm.releaseId || !installForm.characterId"
          @click="submitInstall"
          >安装</el-button
        >
      </template>
    </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from "vue";
import { useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import { Back, Refresh, Picture, Plus } from "@element-plus/icons-vue";
import ExtensionPageHeader from "../extensions/components/ExtensionPageHeader.vue";
import { useApi } from "../../composables/useApi";
import {
  useDesktopPetInstallations,
  type DesktopPetInstallation,
  type ManifestActionInfo,
  type InstallationDetail,
  type InstallationRuntimeStatus,
} from "../../composables/useDesktopPetInstallations";
import { useAssetUrl } from "../../composables/useAssetUrl";

interface CharacterOption {
  id: string | number;
  name: string;
  status?: string;
  isActive?: number | boolean;
  isDefault?: number | boolean;
}

interface PackageOption {
  id: string;
  petId: string;
  version: string;
  status: string;
  sourceGenerationTask: string;
}

const router = useRouter();
const { get } = useApi();
const { assetUrl } = useAssetUrl();

const {
  loading,
  installations,
  install,
  listInstallations,
  getInstallation,
  enable,
  disable,
  updateDefaultAction,
  updateSettings,
  recenter,
  playAction,
  uninstall,
  upgrade,
  switchInstallation,
  repair,
  refresh,
} = useDesktopPetInstallations();

const actionTargetId = ref<string | null>(null);
const disablingAll = ref(false);
const characters = ref<CharacterOption[]>([]);
const characterMap = reactive<Record<string, CharacterOption>>({});
const previewFailedSet = reactive<Set<string>>(new Set());
const playActionLoadingKey = ref<string>("");

const resizeDialogVisible = ref(false);
const resizeForm = reactive({
  id: "",
  name: "",
  scale: 1,
  alwaysOnTop: false,
  restoreOnAppStart: true,
  idleEnabled: false,
  soundEnabled: false,
});

const actionsDialogVisible = ref(false);
const actionsLoading = ref(false);
const actionsList = ref<ManifestActionInfo[]>([]);
const actionsDefaultAction = ref<string>("");
const actionsInstallationId = ref<string>("");
const actionsInstallationStatus = ref<string>("");

const defaultActionDialogVisible = ref(false);
const defaultActionForm = reactive({
  id: "",
  name: "",
  actionKey: "",
});

const installDialogVisible = ref(false);
const installSubmitting = ref(false);
const availablePackages = ref<PackageOption[]>([]);
const installForm = reactive({ petId: "", releaseId: "", characterId: "" });

const upgradeDialogVisible = ref(false);
const upgradeForm = reactive({
  id: "",
  name: "",
  currentReleaseId: "",
  targetReleaseId: "",
});

const statusMeta: Record<string, { label: string; type: string }> = {
  installing: { label: "安装中", type: "warning" },
  installed: { label: "已安装", type: "info" },
  enabled: { label: "已启用", type: "success" },
  disabled: { label: "已停用", type: "info" },
  invalid: { label: "资源损坏", type: "danger" },
  uninstalling: { label: "卸载中", type: "warning" },
  uninstalled: { label: "已卸载", type: "info" },
};

const runtimeStatusMeta: Record<string, { label: string; type: string }> = {
  installed: { label: "已安装", type: "info" },
  pending_runtime: { label: "等待运行端", type: "warning" },
  enabled: { label: "已启用", type: "success" },
  running: { label: "运行中", type: "success" },
  offline: { label: "离线", type: "info" },
  corrupted: { label: "损坏", type: "danger" },
  recovery_required: { label: "待修复", type: "warning" },
};

function statusLabel(status?: string): string {
  if (!status) return "—";
  return statusMeta[status]?.label || status;
}

function statusTagType(status?: string): any {
  const t = status ? statusMeta[status]?.type : "";
  if (t === "success") return "success";
  if (t === "warning") return "warning";
  if (t === "danger") return "danger";
  return "info";
}

function runtimeStatusOf(
  item: DesktopPetInstallation,
): InstallationRuntimeStatus {
  if (item.integrityStatus === "corrupted" || item.status === "invalid") {
    return "corrupted";
  }
  if (item.integrityStatus === "recovery_required") {
    return "recovery_required";
  }
  if (item.lifecycleState === "running") {
    return "running";
  }
  if (item.lifecycleState === "offline") {
    return "offline";
  }
  if (item.lifecycleState === "pending_runtime") {
    return "pending_runtime";
  }
  if (item.lifecycleState === "enabled" || item.status === "enabled") {
    return "enabled";
  }
  return "installed";
}

function runtimeStatusLabel(item: DesktopPetInstallation): string {
  const rs = runtimeStatusOf(item);
  return runtimeStatusMeta[rs]?.label || String(rs);
}

function runtimeStatusTagType(item: DesktopPetInstallation): any {
  const rs = runtimeStatusOf(item);
  const t = runtimeStatusMeta[rs]?.type;
  if (t === "success") return "success";
  if (t === "warning") return "warning";
  if (t === "danger") return "danger";
  return "info";
}

function isActive(item: DesktopPetInstallation): boolean {
  return Number(item.isActive) === 1 && item.status === "enabled";
}

function isUninstalled(item: DesktopPetInstallation): boolean {
  return (
    item.status === "uninstalled" || item.status === "uninstalling"
  );
}

function canEnable(item: DesktopPetInstallation): boolean {
  if (isUninstalled(item)) return false;
  if (item.status === "invalid") return false;
  if (isActive(item)) return false;
  return (
    item.status === "installed" ||
    item.status === "disabled" ||
    item.status === "enabled"
  );
}

function canDisable(item: DesktopPetInstallation): boolean {
  if (isUninstalled(item)) return false;
  return item.status === "enabled" || Number(item.isActive) === 1;
}

function canRecenter(item: DesktopPetInstallation): boolean {
  return item.status === "enabled";
}

function canResize(item: DesktopPetInstallation): boolean {
  if (isUninstalled(item)) return false;
  return (
    item.status === "enabled" ||
    item.status === "disabled" ||
    item.status === "installed"
  );
}

function canViewActions(item: DesktopPetInstallation): boolean {
  if (isUninstalled(item)) return false;
  return (
    item.status === "enabled" ||
    item.status === "disabled" ||
    item.status === "installed"
  );
}

function canChangeDefaultAction(item: DesktopPetInstallation): boolean {
  return canViewActions(item);
}

function canReinstall(item: DesktopPetInstallation): boolean {
  return item.status === "invalid";
}

function canUninstall(item: DesktopPetInstallation): boolean {
  if (isUninstalled(item)) return false;
  return (
    item.status === "installed" ||
    item.status === "enabled" ||
    item.status === "disabled" ||
    item.status === "invalid"
  );
}

function canUpgrade(item: DesktopPetInstallation): boolean {
  if (isUninstalled(item)) return false;
  const rs = runtimeStatusOf(item);
  if (rs === "corrupted" || rs === "recovery_required") return false;
  return !!item.currentReleaseId || !!item.packageId;
}

function canSwitch(item: DesktopPetInstallation): boolean {
  if (isUninstalled(item)) return false;
  const rs = runtimeStatusOf(item);
  return (
    rs === "installed" ||
    rs === "enabled" ||
    rs === "offline" ||
    rs === "pending_runtime"
  );
}

function canRepair(item: DesktopPetInstallation): boolean {
  const rs = runtimeStatusOf(item);
  return rs === "corrupted" || rs === "recovery_required";
}

const activeInstallation = computed(() =>
  installations.value.find((i) => isActive(i)) || null,
);

const hasActiveInstallation = computed(
  () => !!activeInstallation.value,
);

const disabledCount = computed(
  () =>
    installations.value.filter(
      (i) => i.status === "disabled" || i.status === "installed",
    ).length,
);

const invalidCount = computed(
  () => installations.value.filter((i) => i.status === "invalid").length,
);

const defaultIdleCandidates = computed(() =>
  actionsList.value.filter((a) => a.supportsDefaultIdle),
);

const upgradeCandidates = computed(() =>
  availablePackages.value.filter(
    (pkg) => pkg.id !== upgradeForm.currentReleaseId,
  ),
);

function previewUrlOf(item: DesktopPetInstallation): string {
  if (previewFailedSet.has(item.id)) return "";
  if (!item.previewPath) return "";
  return assetUrl(item.previewPath);
}

function onPreviewError(item: DesktopPetInstallation) {
  previewFailedSet.add(item.id);
}

function characterLabelOf(item: DesktopPetInstallation): string {
  if (!item.characterId) return "—";
  const c = characterMap[String(item.characterId)];
  return c?.name || String(item.characterId);
}

function contentHashOf(item: DesktopPetInstallation): string {
  return item.installedContentHash || item.packageHash || "";
}

function shortHash(hash?: string): string {
  if (!hash) return "—";
  if (hash.length <= 16) return hash;
  return `${hash.slice(0, 8)}…${hash.slice(-6)}`;
}

function formatTime(value?: string): string {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(
    date.getDate(),
  )} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

async function loadCharacters() {
  try {
    const list = (await get<CharacterOption[]>("/api/characters")) || [];
    characters.value = list;
    for (const c of list) {
      characterMap[String(c.id)] = c;
    }
  } catch {
    characters.value = [];
  }
}

async function loadList() {
  await listInstallations();
}

async function refreshAll() {
  await Promise.all([loadCharacters(), loadList()]);
}

async function onEnable(item: DesktopPetInstallation) {
  actionTargetId.value = item.id;
  try {
    await enable(item.id);
    await refresh();
  } catch (err: any) {
    ElMessage.error(err?.message || "启用失败");
  } finally {
    actionTargetId.value = null;
  }
}

async function onDisable(item: DesktopPetInstallation) {
  actionTargetId.value = item.id;
  try {
    await disable(item.id);
    await refresh();
  } catch (err: any) {
    ElMessage.error(err?.message || "停用失败");
  } finally {
    actionTargetId.value = null;
  }
}

function confirmDisableAll() {
  if (!activeInstallation.value) return;
  ElMessageBox.confirm(
    `将停用当前正在运行的桌宠「${activeInstallation.value.name}」,窗口会关闭,可以随时再启用。是否继续?`,
    "确认停用桌宠",
    {
      confirmButtonText: "确认停用",
      cancelButtonText: "再想想",
      type: "warning",
    },
  )
    .then(async () => {
      disablingAll.value = true;
      try {
        await disable(activeInstallation.value!.id);
        await refresh();
      } catch (err: any) {
        ElMessage.error(err?.message || "停用失败");
      } finally {
        disablingAll.value = false;
      }
    })
    .catch(() => {});
}

async function onRecenter(item: DesktopPetInstallation) {
  actionTargetId.value = item.id;
  try {
    await recenter(item.id);
    await refresh();
  } catch (err: any) {
    ElMessage.error(err?.message || "重置位置失败");
  } finally {
    actionTargetId.value = null;
  }
}

function openResizeDialog(item: DesktopPetInstallation) {
  resizeForm.id = item.id;
  resizeForm.name = item.name || "未命名桌宠";
  resizeForm.scale = 1;
  resizeForm.alwaysOnTop = true;
  resizeForm.restoreOnAppStart = true;
  resizeForm.idleEnabled = true;
  resizeForm.soundEnabled = false;
  resizeDialogVisible.value = true;
  void loadRuntimeSettings(item);
}

async function loadRuntimeSettings(item: DesktopPetInstallation) {
  try {
    const detail = await getInstallation(item.id);
    const settings = (detail as InstallationDetail).settings;
    if (settings) {
      resizeForm.scale = Number(settings.scale) || 1;
      resizeForm.alwaysOnTop = Number(settings.alwaysOnTop) === 1;
      resizeForm.restoreOnAppStart = Number(settings.restoreOnAppStart) !== 0;
      resizeForm.idleEnabled = Number(settings.idleEnabled) === 1;
      resizeForm.soundEnabled = Number(settings.soundEnabled) === 1;
    }
  } catch {
    // ignore
  }
}

async function submitResize() {
  if (!resizeForm.id) return;
  actionTargetId.value = resizeForm.id;
  try {
    await updateSettings(resizeForm.id, {
      scale: Number(resizeForm.scale.toFixed(2)),
      alwaysOnTop: resizeForm.alwaysOnTop ? 1 : 0,
      restoreOnAppStart: resizeForm.restoreOnAppStart ? 1 : 0,
      idleEnabled: resizeForm.idleEnabled ? 1 : 0,
      soundEnabled: resizeForm.soundEnabled ? 1 : 0,
    });
    resizeDialogVisible.value = false;
    await refresh();
  } catch (err: any) {
    ElMessage.error(err?.message || "保存配置失败");
  } finally {
    actionTargetId.value = null;
  }
}

async function openActionsDialog(item: DesktopPetInstallation) {
  actionsDialogVisible.value = true;
  actionsLoading.value = true;
  actionsList.value = [];
  actionsDefaultAction.value = item.defaultActionKey || "";
  actionsInstallationId.value = item.id;
  actionsInstallationStatus.value = item.status;
  playActionLoadingKey.value = "";
  try {
    const detail = await getInstallation(item.id);
    const actions = (detail as InstallationDetail).manifest?.actions || [];
    actionsList.value = actions.map((a) => ({
      key: a.key,
      name: a.name,
      config: a.config,
      supportsDefaultIdle: a.supportsDefaultIdle,
    }));
    if (detail.defaultActionKey) {
      actionsDefaultAction.value = detail.defaultActionKey;
    }
  } catch (err: any) {
    ElMessage.error(err?.message || "加载动作失败");
  } finally {
    actionsLoading.value = false;
  }
}

async function onPlayAction(actionKey: string) {
  if (!actionsInstallationId.value) return;
  playActionLoadingKey.value = actionKey;
  try {
    await playAction(actionsInstallationId.value, actionKey);
  } catch (err: any) {
    ElMessage.error(err?.message || "播放动作失败");
  } finally {
    playActionLoadingKey.value = "";
  }
}

function openDefaultActionDialog(item: DesktopPetInstallation) {
  defaultActionForm.id = item.id;
  defaultActionForm.name = item.name || "未命名桌宠";
  defaultActionForm.actionKey = item.defaultActionKey || "";
  actionsList.value = [];
  actionsInstallationId.value = item.id;
  void loadActionsForDefault(item);
  defaultActionDialogVisible.value = true;
}

async function loadActionsForDefault(item: DesktopPetInstallation) {
  try {
    const detail = await getInstallation(item.id);
    const actions = (detail as InstallationDetail).manifest?.actions || [];
    actionsList.value = actions.map((a) => ({
      key: a.key,
      name: a.name,
      config: a.config,
      supportsDefaultIdle: a.supportsDefaultIdle,
    }));
    if (detail.defaultActionKey) {
      defaultActionForm.actionKey = detail.defaultActionKey;
    }
  } catch (err: any) {
    ElMessage.error(err?.message || "加载动作失败");
  }
}

async function onChangeDefaultAction(actionKey: string) {
  if (!actionsInstallationId.value) return;
  actionTargetId.value = actionKey;
  try {
    await updateDefaultAction(actionsInstallationId.value, actionKey);
    actionsDefaultAction.value = actionKey;
    defaultActionForm.actionKey = actionKey;
    await refresh();
  } catch (err: any) {
    ElMessage.error(err?.message || "更换默认动作失败");
  } finally {
    actionTargetId.value = null;
  }
}

async function submitDefaultAction() {
  if (!defaultActionForm.actionKey) {
    ElMessage.warning("请选择默认待机动作");
    return;
  }
  actionTargetId.value = defaultActionForm.id;
  try {
    await updateDefaultAction(
      defaultActionForm.id,
      defaultActionForm.actionKey,
    );
    defaultActionDialogVisible.value = false;
    await refresh();
  } catch (err: any) {
    ElMessage.error(err?.message || "更换默认动作失败");
  } finally {
    actionTargetId.value = null;
  }
}

async function openUpgradeDialog(item: DesktopPetInstallation) {
  upgradeForm.id = item.id;
  upgradeForm.name = item.name || "未命名桌宠";
  upgradeForm.currentReleaseId = item.currentReleaseId || "";
  upgradeForm.targetReleaseId = "";
  await loadAvailablePackages();
  upgradeDialogVisible.value = true;
}

async function submitUpgrade() {
  if (!upgradeForm.id || !upgradeForm.targetReleaseId) {
    ElMessage.warning("请选择目标资源包");
    return;
  }
  actionTargetId.value = upgradeForm.id;
  try {
    await upgrade(upgradeForm.id, upgradeForm.targetReleaseId);
    upgradeDialogVisible.value = false;
    await refresh();
  } catch (err: any) {
    ElMessage.error(err?.message || "升级失败");
  } finally {
    actionTargetId.value = null;
  }
}

function onSwitch(item: DesktopPetInstallation) {
  ElMessageBox.confirm(
    `将切换为桌宠「${item.name}」为当前运行实例,是否继续?`,
    "确认切换桌宠",
    {
      confirmButtonText: "确认切换",
      cancelButtonText: "取消",
      type: "info",
    },
  )
    .then(async () => {
      actionTargetId.value = item.id;
      try {
        await switchInstallation(item.id);
        await refresh();
      } catch (err: any) {
        ElMessage.error(err?.message || "切换失败");
      } finally {
        actionTargetId.value = null;
      }
    })
    .catch(() => {});
}

function onRepair(item: DesktopPetInstallation) {
  ElMessageBox.confirm(
    `将尝试修复桌宠「${item.name}」的安装完整性,是否继续?`,
    "确认修复桌宠",
    {
      confirmButtonText: "确认修复",
      cancelButtonText: "取消",
      type: "warning",
    },
  )
    .then(async () => {
      actionTargetId.value = item.id;
      try {
        await repair(item.id);
        await refresh();
      } catch (err: any) {
        ElMessage.error(err?.message || "修复失败");
      } finally {
        actionTargetId.value = null;
      }
    })
    .catch(() => {});
}

async function onReinstall(item: DesktopPetInstallation) {
  actionTargetId.value = item.id;
  try {
    const releaseId =
      item.currentReleaseId || item.legacyPackageId || item.packageId;
    if (!releaseId) {
      ElMessage.warning("缺少资源版本信息,无法重新安装");
      return;
    }
    let petId = "";
    try {
      const data = await get<{ items: PackageOption[]; total: number }>(
        "/api/desktop-pets/releases",
      );
      const rel = (data?.items || []).find((r) => r.id === releaseId);
      petId = rel?.petId || "";
    } catch {
      // ignore
    }
    if (!petId) {
      ElMessage.warning("无法确定桌宠信息,请使用修复功能");
      return;
    }
    await install(petId, releaseId, item.characterId);
    await refresh();
  } catch (err: any) {
    ElMessage.error(err?.message || "重新安装失败");
  } finally {
    actionTargetId.value = null;
  }
}

function confirmUninstall(item: DesktopPetInstallation) {
  ElMessageBox.confirm(
    `卸载桌宠「${item.name}」会删除安装目录与安装记录,生成历史与原始素材将保留。是否继续?`,
    "确认卸载桌宠",
    {
      confirmButtonText: "确认卸载",
      cancelButtonText: "取消",
      type: "warning",
    },
  )
    .then(async () => {
      actionTargetId.value = item.id;
      try {
        await uninstall(item.id);
        await refresh();
      } catch (err: any) {
        ElMessage.error(err?.message || "卸载失败");
      } finally {
        actionTargetId.value = null;
      }
    })
    .catch(() => {});
}

function goBack() {
  router.push("/creative-workshop/pet/tasks");
}

function goToTasks() {
  router.push("/creative-workshop/pet/tasks");
}

async function openInstallDialog() {
  installForm.petId = "";
  installForm.releaseId = "";
  installForm.characterId = "";
  await loadCharacters();
  await loadAvailablePackages();
  installDialogVisible.value = true;
}

async function loadAvailablePackages() {
  try {
    const data = await get<{ items: PackageOption[]; total: number }>("/api/desktop-pets/releases");
    availablePackages.value = data?.items || [];
  } catch {
    availablePackages.value = [];
  }
}

function onInstallPackageChange() {
  const pkg = availablePackages.value.find((p) => p.id === installForm.releaseId);
  if (pkg) {
    installForm.petId = pkg.petId;
  }
}

async function submitInstall() {
  if (!installForm.petId || !installForm.releaseId || !installForm.characterId) {
    ElMessage.warning("请选择资源包和角色");
    return;
  }
  installSubmitting.value = true;
  try {
    await install(installForm.petId, installForm.releaseId, installForm.characterId);
    installDialogVisible.value = false;
    await refresh();
  } catch (err: any) {
    ElMessage.error(err?.message || "安装失败");
  } finally {
    installSubmitting.value = false;
  }
}

onMounted(() => {
  void refreshAll();
});

onUnmounted(() => {
  previewFailedSet.clear();
});
</script>

<style scoped>
.pet-installations {
  height: 100%;
  overflow: auto;
  padding: 0;
}
.summary-card {
  margin-bottom: 12px;
  border: 1px solid var(--el-border-color-light);
  background: var(--ac-color-surface);
}
.summary-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 16px;
}
.summary-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.summary-label {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.summary-value {
  color: var(--console-text);
  font-size: 16px;
  font-weight: 500;
}
.stat-success {
  color: var(--el-color-success);
}
.stat-danger {
  color: var(--el-color-danger);
}
.stat-muted {
  color: var(--el-text-color-secondary);
}
.table-card {
  border: 1px solid var(--el-border-color-light);
  background: var(--ac-color-surface);
}
.installation-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
  gap: 12px;
}
.installation-card {
  border: 1px solid var(--el-border-color-light);
  border-radius: 8px;
  padding: 12px;
  background: var(--ac-color-surface);
  display: flex;
  flex-direction: column;
  gap: 10px;
  transition: border-color 180ms ease;
}
.installation-card.active {
  border-color: var(--el-color-success);
  box-shadow: 0 0 0 1px var(--el-color-success-light-7) inset;
}
.installation-card.invalid {
  border-color: var(--el-color-danger-light-5);
}
.installation-card.uninstalled {
  opacity: 0.6;
}
.card-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.card-title {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.card-version {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.card-body {
  display: grid;
  grid-template-columns: 120px 1fr;
  gap: 12px;
}
.preview-box {
  width: 120px;
  height: 120px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
  overflow: hidden;
  background: var(--ac-color-bg-secondary, #f5f7fa);
  display: flex;
  align-items: center;
  justify-content: center;
}
.preview-box img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}
.preview-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.meta {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}
.meta-row {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 12px;
}
.meta-label {
  color: var(--el-text-color-secondary);
}
.meta-value {
  color: var(--console-text);
  word-break: break-all;
}
.hash-value {
  font-family: var(--el-font-family-mono, monospace);
  font-size: 12px;
}
.invalid-banner {
  margin-top: 4px;
}
.card-foot {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  justify-content: flex-end;
}
.resize-hint {
  margin-left: 8px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.actions-dialog-body {
  min-height: 200px;
}
.action-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.action-row {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 6px;
  padding: 10px 12px;
  background: var(--ac-color-surface);
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.action-row-head {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.action-row-meta {
  display: flex;
  gap: 8px;
  font-size: 12px;
}
.action-row-foot {
  display: flex;
  justify-content: flex-end;
  gap: 6px;
}
</style>
