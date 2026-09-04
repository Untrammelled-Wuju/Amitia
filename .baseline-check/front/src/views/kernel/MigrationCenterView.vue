<template>
  <div class="migration-center">
    <div class="migration-header">
      <div class="header-left">
        <h2>迁移中心</h2>
        <p class="subtitle">数据迁移 / 灰度发布 / 回滚管理 — 全生命周期可视化</p>
      </div>
      <div class="header-right">
        <el-button @click="refreshAll" :icon="Refresh" :loading="loading">刷新</el-button>
      </div>
    </div>

    <el-tabs v-model="activeTab">
      <el-tab-pane label="数据迁移" name="migration">
        <div class="tab-toolbar">
          <el-input
            v-model="migrationFilter"
            placeholder="按扩展ID过滤..."
            clearable
            style="width: 280px"
            :prefix-icon="Search"
            @keyup.enter="loadMigrations"
            @clear="loadMigrations"
          />
          <el-button type="primary" @click="loadMigrations">查询</el-button>
          <el-button type="success" @click="openPlanDialog">规划迁移</el-button>
        </div>

        <el-table :data="migrations" border v-loading="loading" style="width: 100%">
          <el-table-column prop="migration_id" label="迁移ID" min-width="180" show-overflow-tooltip />
          <el-table-column prop="extension_id" label="扩展ID" min-width="160" show-overflow-tooltip />
          <el-table-column prop="from_version_range" label="起始版本范围" min-width="140" show-overflow-tooltip />
          <el-table-column prop="to_version" label="目标版本" width="120" />
          <el-table-column label="方向" width="100">
            <template #default="{ row }">
              <el-tag :type="directionTagType(row.direction)" size="small">{{ row.direction }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="可逆性" width="120">
            <template #default="{ row }">
              <el-tag :type="row.reversibility === 'reversible' ? 'success' : 'warning'" size="small">
                {{ row.reversibility }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="definition_hash" label="定义哈希" min-width="180" show-overflow-tooltip />
          <el-table-column label="创建时间" width="180">
            <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="120" fixed="right">
            <template #default="{ row }">
              <el-button size="small" link @click="openPlanDialogFromRow(row)">规划</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loading && migrations.length === 0" description="暂无迁移定义" />
      </el-tab-pane>

      <el-tab-pane label="灰度发布" name="canary">
        <div class="tab-toolbar">
          <el-input
            v-model="canaryFilter"
            placeholder="按扩展ID过滤..."
            clearable
            style="width: 280px"
            :prefix-icon="Search"
            @keyup.enter="loadCanaries"
            @clear="loadCanaries"
          />
          <el-button type="primary" @click="loadCanaries">查询</el-button>
          <el-button type="success" @click="openCanaryCreate">创建灰度</el-button>
        </div>

        <el-table :data="canaries" border v-loading="loading" style="width: 100%">
          <el-table-column prop="canary_id" label="灰度ID" min-width="180" show-overflow-tooltip />
          <el-table-column prop="extension_id" label="扩展ID" min-width="160" show-overflow-tooltip />
          <el-table-column label="当前阶段" width="120">
            <template #default="{ row }">
              <el-tag size="small">{{ row.current_stage }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="120">
            <template #default="{ row }">
              <el-tag :type="statusTagType(row.status)" size="small">{{ row.status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="generation_from" label="From Gen" width="100" />
          <el-table-column prop="generation_to" label="To Gen" width="100" />
          <el-table-column label="开始时间" width="180">
            <template #default="{ row }">{{ formatDate(row.started_at) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="100" fixed="right">
            <template #default="{ row }">
              <el-button size="small" link @click="showCanaryDetail(row)">详情</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loading && canaries.length === 0" description="暂无灰度发布" />
      </el-tab-pane>

      <el-tab-pane label="回滚管理" name="rollback">
        <div class="tab-toolbar">
          <el-input
            v-model="rollbackFilter"
            placeholder="按扩展ID过滤..."
            clearable
            style="width: 280px"
            :prefix-icon="Search"
            @keyup.enter="loadRollbacks"
            @clear="loadRollbacks"
          />
          <el-button type="primary" @click="loadRollbacks">查询</el-button>
          <el-button type="warning" @click="doScanRecovery">恢复扫描</el-button>
        </div>

        <el-table :data="rollbacks" border v-loading="loading" style="width: 100%">
          <el-table-column prop="rollback_id" label="回滚ID" min-width="180" show-overflow-tooltip />
          <el-table-column prop="extension_id" label="扩展ID" min-width="160" show-overflow-tooltip />
          <el-table-column label="级别" width="120">
            <template #default="{ row }">
              <el-tag :type="row.level === 'hard' ? 'danger' : 'info'" size="small">{{ row.level }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="120">
            <template #default="{ row }">
              <el-tag :type="statusTagType(row.status)" size="small">{{ row.status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="from_generation" label="From Gen" width="100" />
          <el-table-column prop="to_generation" label="To Gen" width="100" />
          <el-table-column label="开始时间" width="180">
            <template #default="{ row }">{{ formatDate(row.started_at) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="100" fixed="right">
            <template #default="{ row }">
              <el-button size="small" link @click="showRollbackDetail(row)">详情</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loading && rollbacks.length === 0" description="暂无回滚计划" />
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="planVisible" title="规划迁移" width="640px" top="8vh">
      <el-form label-width="120px" v-loading="subLoading">
        <el-form-item label="扩展ID">
          <el-input v-model="planForm.extension_id" placeholder="例如: ext.example" />
        </el-form-item>
        <el-form-item label="起始版本">
          <el-input v-model="planForm.from_version" placeholder="例如: 1.0.0" />
        </el-form-item>
        <el-form-item label="目标版本">
          <el-input v-model="planForm.to_version" placeholder="例如: 2.0.0" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="doPlanMigration">规划迁移</el-button>
        </el-form-item>
      </el-form>

      <div v-if="planResult" class="plan-result">
        <el-descriptions title="规划结果" :column="1" border>
          <el-descriptions-item label="迁移路径">
            <div class="path-tags">
              <el-tag v-for="(p, i) in planResult.path" :key="i" size="small" class="path-tag">{{ p }}</el-tag>
            </div>
          </el-descriptions-item>
          <el-descriptions-item label="风险等级">
            <el-tag :type="riskTagType(planResult.estimated_risk)" size="small">{{ planResult.estimated_risk }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="是否可逆">
            <el-tag :type="planResult.has_irreversible ? 'danger' : 'success'" size="small">
              {{ planResult.has_irreversible ? '包含不可逆步骤' : '全部可逆' }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="可逆性">{{ planResult.reversibility }}</el-descriptions-item>
          <el-descriptions-item label="需要用户确认">{{ planResult.requires_user_confirm ? '是' : '否' }}</el-descriptions-item>
        </el-descriptions>
        <div class="dialog-footer">
          <el-button type="danger" @click="doExecuteMigration">执行迁移</el-button>
        </div>
      </div>
    </el-dialog>

    <el-dialog v-model="canaryCreateVisible" title="创建灰度发布" width="560px">
      <el-form label-width="120px" v-loading="subLoading">
        <el-form-item label="扩展ID">
          <el-input v-model="canaryCreateForm.extension_id" placeholder="例如: ext.example" />
        </el-form-item>
        <el-form-item label="策略ID">
          <el-input v-model="canaryCreateForm.policy_id" placeholder="例如: policy.001" />
        </el-form-item>
        <el-form-item label="起始Generation">
          <el-input v-model="canaryCreateForm.generation_from" placeholder="例如: 1" />
        </el-form-item>
        <el-form-item label="目标Generation">
          <el-input v-model="canaryCreateForm.generation_to" placeholder="例如: 2" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="doCreateCanary">创建</el-button>
          <el-button @click="canaryCreateVisible = false">取消</el-button>
        </el-form-item>
      </el-form>
    </el-dialog>

    <el-dialog v-model="canaryDetailVisible" title="灰度详情" width="640px" top="8vh">
      <template v-if="canaryDetail">
        <el-descriptions :column="2" border v-loading="subLoading">
          <el-descriptions-item label="灰度ID">{{ canaryDetail.canary_id }}</el-descriptions-item>
          <el-descriptions-item label="扩展ID">{{ canaryDetail.extension_id }}</el-descriptions-item>
          <el-descriptions-item label="策略ID">{{ canaryDetail.policy_id }}</el-descriptions-item>
          <el-descriptions-item label="当前阶段">
            <el-tag size="small">{{ canaryDetail.current_stage }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="statusTagType(canaryDetail.status)" size="small">{{ canaryDetail.status }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="From Gen">{{ canaryDetail.generation_from }}</el-descriptions-item>
          <el-descriptions-item label="To Gen">{{ canaryDetail.generation_to }}</el-descriptions-item>
          <el-descriptions-item label="开始时间">{{ formatDate(canaryDetail.started_at) }}</el-descriptions-item>
          <el-descriptions-item label="完成时间">{{ formatDate(canaryDetail.finished_at) }}</el-descriptions-item>
          <el-descriptions-item label="中止原因" :span="2">{{ canaryDetail.abort_reason || '无' }}</el-descriptions-item>
        </el-descriptions>

        <div class="detail-actions">
          <el-button
            type="primary"
            @click="doAdvanceCanary"
            :disabled="isTerminalCanary(canaryDetail.status)"
          >推进阶段</el-button>
          <el-button
            type="warning"
            @click="doPauseCanary"
            :disabled="canaryDetail.status === 'paused' || isTerminalCanary(canaryDetail.status)"
          >暂停</el-button>
          <el-button
            type="success"
            @click="doResumeCanary"
            :disabled="canaryDetail.status !== 'paused'"
          >恢复</el-button>
          <el-button
            type="danger"
            @click="doAbortCanary"
            :disabled="isTerminalCanary(canaryDetail.status)"
          >中止</el-button>
          <el-button
            type="success"
            @click="doCommitCanary"
            :disabled="isTerminalCanary(canaryDetail.status)"
          >提交</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="rollbackDetailVisible" title="回滚详情" width="920px" top="5vh">
      <template v-if="rollbackDetail">
        <el-descriptions :column="2" border v-loading="subLoading">
          <el-descriptions-item label="回滚ID">{{ rollbackDetail.rollback_id }}</el-descriptions-item>
          <el-descriptions-item label="操作ID">{{ rollbackDetail.operation_id }}</el-descriptions-item>
          <el-descriptions-item label="扩展ID">{{ rollbackDetail.extension_id }}</el-descriptions-item>
          <el-descriptions-item label="级别">
            <el-tag :type="rollbackDetail.level === 'hard' ? 'danger' : 'info'" size="small">{{ rollbackDetail.level }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="statusTagType(rollbackDetail.status)" size="small">{{ rollbackDetail.status }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="自动">{{ rollbackDetail.automatic ? '是' : '否' }}</el-descriptions-item>
          <el-descriptions-item label="From Gen">{{ rollbackDetail.from_generation }}</el-descriptions-item>
          <el-descriptions-item label="To Gen">{{ rollbackDetail.to_generation }}</el-descriptions-item>
          <el-descriptions-item label="需要用户操作">{{ rollbackDetail.requires_user_action ? '是' : '否' }}</el-descriptions-item>
          <el-descriptions-item label="开始时间">{{ formatDate(rollbackDetail.started_at) }}</el-descriptions-item>
          <el-descriptions-item label="完成时间">{{ formatDate(rollbackDetail.finished_at) }}</el-descriptions-item>
          <el-descriptions-item label="错误码">{{ rollbackDetail.error_code || '无' }}</el-descriptions-item>
          <el-descriptions-item label="错误信息" :span="2">{{ rollbackDetail.error_message || '无' }}</el-descriptions-item>
        </el-descriptions>

        <div class="detail-actions">
          <el-button type="danger" @click="doExecuteRollback">执行回滚</el-button>
          <el-button type="warning" @click="doRecoverRollback">恢复回滚</el-button>
          <el-button @click="loadJournal(rollbackDetail.operation_id)">查看日志</el-button>
          <el-button size="small" @click="loadRollbackSteps(rollbackDetail.rollback_id)">刷新步骤</el-button>
        </div>

        <h4 class="section-title">回滚步骤</h4>
        <el-table :data="rollbackSteps" border size="small" v-loading="subLoading">
          <el-table-column prop="step_id" label="步骤ID" min-width="180" show-overflow-tooltip />
          <el-table-column prop="step_type" label="类型" width="140" />
          <el-table-column label="状态" width="110">
            <template #default="{ row }">
              <el-tag :type="statusTagType(row.status)" size="small">{{ row.status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="开始时间" width="180">
            <template #default="{ row }">{{ formatDate(row.started_at) }}</template>
          </el-table-column>
          <el-table-column label="完成时间" width="180">
            <template #default="{ row }">{{ formatDate(row.finished_at) }}</template>
          </el-table-column>
          <el-table-column prop="error_code" label="错误码" width="120" show-overflow-tooltip />
          <el-table-column prop="error_message" label="错误信息" min-width="160" show-overflow-tooltip />
        </el-table>
        <el-empty v-if="!subLoading && rollbackSteps.length === 0" description="暂无步骤" />

        <div v-if="journalLoaded" class="detail-section">
          <h4 class="section-title">日志条目</h4>
          <el-table :data="journalEntries" border size="small" v-loading="subLoading">
            <el-table-column prop="entry_id" label="条目ID" min-width="180" show-overflow-tooltip />
            <el-table-column prop="step_id" label="步骤ID" min-width="160" show-overflow-tooltip />
            <el-table-column prop="step_type" label="类型" width="120" />
            <el-table-column label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="statusTagType(row.status)" size="small">{{ row.status }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="开始时间" width="180">
              <template #default="{ row }">{{ formatDate(row.started_at) }}</template>
            </el-table-column>
            <el-table-column label="完成时间" width="180">
              <template #default="{ row }">{{ formatDate(row.finished_at) }}</template>
            </el-table-column>
            <el-table-column prop="error_code" label="错误码" width="100" show-overflow-tooltip />
          </el-table>
          <el-empty v-if="!subLoading && journalEntries.length === 0" description="暂无日志" />
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="recoveryVisible" title="恢复扫描" width="760px">
      <el-table :data="recoveryActions" border size="small" v-loading="subLoading">
        <el-table-column prop="operation_id" label="操作ID" min-width="180" show-overflow-tooltip />
        <el-table-column prop="strategy" label="策略" width="140" />
        <el-table-column prop="detail" label="详情" min-width="220" show-overflow-tooltip />
        <el-table-column label="操作" width="100" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="primary" @click="doExecuteRecovery(row)">执行</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!subLoading && recoveryActions.length === 0" description="无需恢复的操作" />
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Refresh, Search } from "@element-plus/icons-vue";
import {
  listMigrations,
  planMigration,
  executeMigration,
  listCanaryStates,
  createCanaryState,
  advanceCanary,
  pauseCanary,
  resumeCanary,
  abortCanary,
  commitCanary,
  listRollbacks,
  listRollbackSteps,
  executeRollback,
  recoverRollback,
  scanRecovery,
  executeRecovery,
  getJournalEntries,
} from "./migration-api";
import type {
  MigrationDefinition,
  MigrationPlanInput,
  MigrationPlanOutput,
  CanaryState,
  CanaryPolicy,
  RollbackPlan,
  RollbackStepRecord,
  RecoveryAction,
  LifecycleJournalEntry,
} from "./migration-types";

const loading = ref(false);
const subLoading = ref(false);
const activeTab = ref("migration");

const migrationFilter = ref("");
const canaryFilter = ref("");
const rollbackFilter = ref("");

const migrations = ref<MigrationDefinition[]>([]);
const canaries = ref<CanaryState[]>([]);
const rollbacks = ref<RollbackPlan[]>([]);

const planVisible = ref(false);
const planForm = ref({ extension_id: "", from_version: "", to_version: "" });
const planResult = ref<MigrationPlanOutput | null>(null);

const canaryCreateVisible = ref(false);
const canaryCreateForm = ref({ extension_id: "", policy_id: "", generation_from: "", generation_to: "" });

const canaryDetailVisible = ref(false);
const canaryDetail = ref<CanaryState | null>(null);

const rollbackDetailVisible = ref(false);
const rollbackDetail = ref<RollbackPlan | null>(null);
const rollbackSteps = ref<RollbackStepRecord[]>([]);
const journalEntries = ref<LifecycleJournalEntry[]>([]);
const journalLoaded = ref(false);

const recoveryVisible = ref(false);
const recoveryActions = ref<RecoveryAction[]>([]);

function statusTagType(status: string): "primary" | "success" | "info" | "warning" | "danger" {
  const greenStatuses = ["completed", "full", "committing"];
  const yellowStatuses = ["paused"];
  const redStatuses = ["failed", "aborting", "aborted", "rolled_back"];
  if (greenStatuses.includes(status)) return "success";
  if (yellowStatuses.includes(status)) return "warning";
  if (redStatuses.includes(status)) return "danger";
  return "primary";
}

function directionTagType(direction: string): "primary" | "warning" {
  return direction === "reverse" ? "warning" : "primary";
}

function riskTagType(risk: string): "primary" | "success" | "info" | "warning" | "danger" {
  const r = (risk || "").toLowerCase();
  if (r.includes("high") || r.includes("critical")) return "danger";
  if (r.includes("medium")) return "warning";
  if (r.includes("low")) return "success";
  return "info";
}

function isTerminalCanary(status: string): boolean {
  return ["completed", "aborted", "rolled_back", "failed"].includes(status);
}

function formatDate(s?: string): string {
  if (!s) return "";
  try {
    return new Date(s).toLocaleString("zh-CN");
  } catch {
    return s;
  }
}

async function loadMigrations() {
  loading.value = true;
  try {
    const data = await listMigrations(migrationFilter.value || undefined);
    migrations.value = data.items || [];
  } catch (e: any) {
    ElMessage.error("加载迁移列表失败: " + (e?.message || e));
  } finally {
    loading.value = false;
  }
}

async function loadCanaries() {
  loading.value = true;
  try {
    const data = await listCanaryStates(canaryFilter.value || undefined);
    canaries.value = data.items || [];
  } catch (e: any) {
    ElMessage.error("加载灰度列表失败: " + (e?.message || e));
  } finally {
    loading.value = false;
  }
}

async function loadRollbacks() {
  loading.value = true;
  try {
    const data = await listRollbacks(rollbackFilter.value || undefined);
    rollbacks.value = data.items || [];
  } catch (e: any) {
    ElMessage.error("加载回滚列表失败: " + (e?.message || e));
  } finally {
    loading.value = false;
  }
}

async function refreshAll() {
  await Promise.all([loadMigrations(), loadCanaries(), loadRollbacks()]);
}

function openPlanDialog() {
  planForm.value = { extension_id: "", from_version: "", to_version: "" };
  planResult.value = null;
  planVisible.value = true;
}

function openPlanDialogFromRow(row: MigrationDefinition) {
  planForm.value = {
    extension_id: row.extension_id,
    from_version: "",
    to_version: row.to_version,
  };
  planResult.value = null;
  planVisible.value = true;
}

async function doPlanMigration() {
  if (!planForm.value.extension_id || !planForm.value.from_version || !planForm.value.to_version) {
    ElMessage.warning("请填写完整的扩展ID、起始版本和目标版本");
    return;
  }
  subLoading.value = true;
  try {
    const input: MigrationPlanInput = {
      extension_id: planForm.value.extension_id,
      from_version: planForm.value.from_version,
      to_version: planForm.value.to_version,
    };
    planResult.value = await planMigration(input);
  } catch (e: any) {
    ElMessage.error("规划迁移失败: " + (e?.message || e));
  } finally {
    subLoading.value = false;
  }
}

async function doExecuteMigration() {
  if (!planForm.value.extension_id || !planForm.value.from_version || !planForm.value.to_version) {
    ElMessage.warning("请先完成迁移规划");
    return;
  }
  subLoading.value = true;
  try {
    const input: MigrationPlanInput = {
      extension_id: planForm.value.extension_id,
      from_version: planForm.value.from_version,
      to_version: planForm.value.to_version,
    };
    const op = await executeMigration(input);
    ElMessage.success(`迁移已启动: ${op.operation_id} (状态: ${op.status})`);
    planVisible.value = false;
    await loadMigrations();
  } catch (e: any) {
    ElMessage.error("执行迁移失败: " + (e?.message || e));
  } finally {
    subLoading.value = false;
  }
}

function openCanaryCreate() {
  canaryCreateForm.value = { extension_id: "", policy_id: "", generation_from: "", generation_to: "" };
  canaryCreateVisible.value = true;
}

async function doCreateCanary() {
  if (!canaryCreateForm.value.extension_id || !canaryCreateForm.value.policy_id) {
    ElMessage.warning("请填写扩展ID和策略ID");
    return;
  }
  subLoading.value = true;
  try {
    const state: Partial<CanaryState> = {
      extension_id: canaryCreateForm.value.extension_id,
      policy_id: canaryCreateForm.value.policy_id,
      generation_from: Number(canaryCreateForm.value.generation_from),
      generation_to: Number(canaryCreateForm.value.generation_to),
    };
    const policy: Partial<CanaryPolicy> = {
      policy_id: canaryCreateForm.value.policy_id,
      extension_id: canaryCreateForm.value.extension_id,
    };
    await createCanaryState(state, policy);
    ElMessage.success("灰度发布已创建");
    canaryCreateVisible.value = false;
    await loadCanaries();
  } catch (e: any) {
    ElMessage.error("创建灰度失败: " + (e?.message || e));
  } finally {
    subLoading.value = false;
  }
}

function showCanaryDetail(row: CanaryState) {
  canaryDetail.value = row;
  canaryDetailVisible.value = true;
}

async function doAdvanceCanary() {
  if (!canaryDetail.value) return;
  subLoading.value = true;
  try {
    canaryDetail.value = await advanceCanary(canaryDetail.value.canary_id);
    ElMessage.success("已推进阶段");
    await loadCanaries();
  } catch (e: any) {
    ElMessage.error("推进失败: " + (e?.message || e));
  } finally {
    subLoading.value = false;
  }
}

async function doPauseCanary() {
  if (!canaryDetail.value) return;
  subLoading.value = true;
  try {
    canaryDetail.value = await pauseCanary(canaryDetail.value.canary_id);
    ElMessage.success("已暂停灰度");
    await loadCanaries();
  } catch (e: any) {
    ElMessage.error("暂停失败: " + (e?.message || e));
  } finally {
    subLoading.value = false;
  }
}

async function doResumeCanary() {
  if (!canaryDetail.value) return;
  subLoading.value = true;
  try {
    canaryDetail.value = await resumeCanary(canaryDetail.value.canary_id);
    ElMessage.success("已恢复灰度");
    await loadCanaries();
  } catch (e: any) {
    ElMessage.error("恢复失败: " + (e?.message || e));
  } finally {
    subLoading.value = false;
  }
}

async function doAbortCanary() {
  if (!canaryDetail.value) return;
  try {
    const { value } = await ElMessageBox.prompt("请输入中止原因", "中止灰度", {
      confirmButtonText: "确定",
      cancelButtonText: "取消",
      inputType: "text",
      inputPlaceholder: "请输入中止原因...",
    });
    if (!value) {
      ElMessage.warning("请输入中止原因");
      return;
    }
    subLoading.value = true;
    canaryDetail.value = await abortCanary(canaryDetail.value.canary_id, value);
    ElMessage.success("已中止灰度");
    await loadCanaries();
  } catch (e: any) {
    if (e !== "cancel" && e?.message !== "cancel") {
      ElMessage.error("中止失败: " + (e?.message || e));
    }
  } finally {
    subLoading.value = false;
  }
}

async function doCommitCanary() {
  if (!canaryDetail.value) return;
  try {
    await ElMessageBox.confirm("确定提交灰度发布吗？提交后无法回退。", "提交确认", { type: "warning" });
    subLoading.value = true;
    canaryDetail.value = await commitCanary(canaryDetail.value.canary_id);
    ElMessage.success("已提交灰度发布");
    await loadCanaries();
  } catch (e: any) {
    if (e !== "cancel" && e?.message !== "cancel") {
      ElMessage.error("提交失败: " + (e?.message || e));
    }
  } finally {
    subLoading.value = false;
  }
}

async function showRollbackDetail(row: RollbackPlan) {
  rollbackDetail.value = row;
  rollbackSteps.value = [];
  journalEntries.value = [];
  journalLoaded.value = false;
  rollbackDetailVisible.value = true;
  await loadRollbackSteps(row.rollback_id);
}

async function loadRollbackSteps(rollbackId: string) {
  subLoading.value = true;
  try {
    const data = await listRollbackSteps(rollbackId);
    rollbackSteps.value = data.items || [];
  } catch (e: any) {
    ElMessage.error("加载步骤失败: " + (e?.message || e));
  } finally {
    subLoading.value = false;
  }
}

async function doExecuteRollback() {
  if (!rollbackDetail.value) return;
  try {
    await ElMessageBox.confirm("确定执行回滚吗？此操作可能影响数据。", "执行确认", { type: "warning" });
    subLoading.value = true;
    rollbackDetail.value = await executeRollback(rollbackDetail.value.rollback_id);
    ElMessage.success("回滚已执行");
    await loadRollbackSteps(rollbackDetail.value.rollback_id);
    await loadRollbacks();
  } catch (e: any) {
    if (e !== "cancel" && e?.message !== "cancel") {
      ElMessage.error("执行回滚失败: " + (e?.message || e));
    }
  } finally {
    subLoading.value = false;
  }
}

async function doRecoverRollback() {
  if (!rollbackDetail.value) return;
  subLoading.value = true;
  try {
    rollbackDetail.value = await recoverRollback(rollbackDetail.value.rollback_id);
    ElMessage.success("回滚已恢复");
    await loadRollbackSteps(rollbackDetail.value.rollback_id);
    await loadRollbacks();
  } catch (e: any) {
    ElMessage.error("恢复回滚失败: " + (e?.message || e));
  } finally {
    subLoading.value = false;
  }
}

async function doScanRecovery() {
  subLoading.value = true;
  try {
    const data = await scanRecovery();
    recoveryActions.value = data.items || [];
    recoveryVisible.value = true;
  } catch (e: any) {
    ElMessage.error("扫描恢复失败: " + (e?.message || e));
  } finally {
    subLoading.value = false;
  }
}

async function doExecuteRecovery(action: RecoveryAction) {
  subLoading.value = true;
  try {
    const result = await executeRecovery(action);
    ElMessage.success(`恢复已执行: ${result.operation_id} (状态: ${result.status})`);
    const data = await scanRecovery();
    recoveryActions.value = data.items || [];
  } catch (e: any) {
    ElMessage.error("执行恢复失败: " + (e?.message || e));
  } finally {
    subLoading.value = false;
  }
}

async function loadJournal(operationId: string) {
  subLoading.value = true;
  try {
    const data = await getJournalEntries(operationId);
    journalEntries.value = data.items || [];
    journalLoaded.value = true;
  } catch (e: any) {
    ElMessage.error("加载日志失败: " + (e?.message || e));
  } finally {
    subLoading.value = false;
  }
}

onMounted(async () => {
  await refreshAll();
});
</script>

<style scoped>
.migration-center {
  padding: 20px;
}

.migration-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.migration-header h2 {
  margin: 0 0 4px 0;
  font-size: 22px;
}

.subtitle {
  margin: 0;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.header-right {
  display: flex;
  gap: 8px;
}

.tab-toolbar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
  align-items: center;
}

.plan-result {
  margin-top: 16px;
}

.path-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.path-tag {
  margin: 2px;
}

.dialog-footer {
  margin-top: 16px;
  text-align: right;
}

.detail-actions {
  display: flex;
  gap: 8px;
  margin: 16px 0;
  flex-wrap: wrap;
}

.detail-section {
  margin-top: 20px;
}

.section-title {
  margin: 0 0 8px 0;
  font-size: 14px;
}
</style>
