<template>
  <div class="privacy-scan-view">
    <div class="page-header">
      <div class="page-header-row">
        <el-button text @click="$router.back()" class="back-btn">
          <el-icon><ArrowLeft /></el-icon>
        </el-button>
        <h2>敏感数据扫描</h2>
      </div>
      <p class="page-desc">
        扫描历史记录、记忆和导入数据中的敏感信息，进行脱敏处理
      </p>
    </div>

    <el-alert
      title="不会自动删除数据"
      type="info"
      :closable="false"
      show-icon
      class="notice-alert"
    >
      <template #default>
        <p>
          扫描仅检测敏感信息，不会自动修改或删除你的数据。脱敏操作需要你手动确认。
        </p>
      </template>
    </el-alert>

    <ScanScopePanel :scanning="scanning" @scan="runScan" />

    <ScanResultsPanel v-if="scanSummary" :scan-summary="scanSummary" />

    <ScanHistoryPanel @view-result="onViewHistoryResult" />

    <el-card class="lifecycle-card" shadow="never">
      <template #header><strong>数据生命周期删除</strong></template>
      <p class="lifecycle-desc">按原始数据归属删除指定记忆、消息、会话或角色数据。以持有方回执确认读取阻断和清理结果。</p>
      <el-select v-if="deletionStats.legacyRecords?.length" clearable placeholder="选择需要确认归属的旧删除任务" @change="selectLegacyDeletion">
        <el-option v-for="item in deletionStats.legacyRecords" :key="item.id" :value="item.id" :label="`${item.targetType} / ${item.targetId} / ${item.scope}`" />
      </el-select>
      <el-row :gutter="12">
        <el-col :xs="24" :sm="8">
          <el-select v-model="deletionForm.targetType" style="width: 100%" :disabled="deletionBusy">
            <el-option label="记忆" value="memory" />
            <el-option label="消息" value="message" />
            <el-option label="会话" value="conversation" />
            <el-option label="角色" value="character" />
          </el-select>
        </el-col>
        <el-col :xs="24" :sm="8">
          <el-select v-model="deletionForm.scope" style="width: 100%" :disabled="deletionBusy">
            <el-option label="全部关联数据" value="all" />
            <el-option label="记忆" value="memory" />
            <el-option label="信念" value="belief" />
            <el-option label="关系" value="relation" />
            <el-option label="运行轨迹" value="trace" />
          </el-select>
        </el-col>
        <el-col :xs="24" :sm="8">
          <el-input v-model="deletionForm.targetId" placeholder="数据 ID" :disabled="deletionBusy" />
        </el-col>
      </el-row>
      <el-input v-model="deletionForm.reason" placeholder="删除原因（可选）" class="lifecycle-reason" :disabled="deletionBusy" />
      <div class="lifecycle-actions">
        <el-button :loading="deletionBusy" @click="testDeletion">运行安全测试</el-button>
        <el-button type="danger" :loading="deletionBusy" @click="submitDeletion">提交删除</el-button>
        <el-button :loading="deletionBusy" @click="cleanupDeletion">执行待处理清理</el-button>
      </div>
      <el-descriptions :column="3" border class="lifecycle-stats">
        <el-descriptions-item label="删除记录">{{ deletionStats.total ?? deletionStats.totalTombstones ?? "—" }}</el-descriptions-item>
        <el-descriptions-item label="待清理">{{ deletionStats.pending ?? deletionStats.pendingCleanup ?? "—" }}</el-descriptions-item>
        <el-descriptions-item label="当前状态">{{ deletionStatus?.status || "—" }}</el-descriptions-item>
      </el-descriptions>
      <el-alert v-if="deletionStatus?.lastError" type="warning" :closable="false" :title="deletionStatus.lastError" show-icon />
      <el-table v-if="securityResults.length" :data="securityResults" size="small" class="security-results">
        <el-table-column prop="kind" label="测试" min-width="180" />
        <el-table-column label="结果" width="100">
          <template #default="{ row }"><el-tag :type="row.passed ? 'success' : 'danger'">{{ row.passed ? "通过" : "未通过" }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="detail" label="详情" min-width="240" />
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { ArrowLeft } from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import ScanScopePanel from "./components/ScanScopePanel.vue";
import ScanResultsPanel from "./components/ScanResultsPanel.vue";
import ScanHistoryPanel from "./components/ScanHistoryPanel.vue";
import {
  getDeletionStats,
  getDeletionStatus,
  getScanResult,
  postScan,
  requestDeletion,
  previewDeletion,
  runDeletionCleanup,
  runDeletionSecurityTests,
} from "./api";

const scanning = ref(false);
const scanSummary = ref<any>(null);
const deletionBusy = ref(false);
const deletionStats = ref<any>({});
const deletionStatus = ref<any>(null);
const securityResults = ref<any[]>([]);
const deletionForm = reactive({ targetId: "", targetType: "memory", scope: "all", reason: "", legacyDeletionId: "" });
function selectLegacyDeletion(id: string) {
  const record = deletionStats.value.legacyRecords?.find((item: any) => item.id === id);
  deletionForm.legacyDeletionId = id || "";
  if (record) Object.assign(deletionForm, { targetId: record.targetId, targetType: record.targetType, scope: record.scope });
}

async function loadDeletionStats() {
  try {
    deletionStats.value = await getDeletionStats();
  } catch {
    deletionStats.value = {};
  }
}

async function submitDeletion() {
  if (!deletionForm.targetId.trim()) {
    ElMessage.warning("请输入数据 ID");
    return;
  }
  deletionBusy.value = true;
  try {
    const intents = await previewDeletion({ ...deletionForm, targetId: deletionForm.targetId.trim() });
    let selected = 0;
    if (intents.length > 1) {
      const answer = await ElMessageBox.prompt(intents.map((item, index) => `${index + 1}：${item.target?.ownerId} / ${item.target?.roleId} / ${item.target?.legacy ? "历史记录" : "当前记录"}`).join("\n"), "请选择要删除的数据来源", { inputPattern: /^[1-9]\d*$/, inputErrorMessage: "请输入来源序号" });
      selected = Number(answer.value) - 1;
      if (!intents[selected]) throw new Error("数据来源序号无效");
    }
    const intent = intents[selected]!;
    await ElMessageBox.confirm(`删除 ${intent.targetType}：${intent.targetId}；范围：${intent.scope}；数据来源：${intent.target?.ownerId || "当前记忆持有方"}。此操作不可撤销。`, "确认删除", { type: "warning" });
    deletionStatus.value = await requestDeletion(intent);
    const result = deletionStatus.value;
    if (result.status === "completed") ElMessage.success("所选范围已完成并通过存储核验");
    else if (result.retrievalBlocked === true) ElMessage.warning("所选范围已停止读取，派生数据仍在清理，请稍后核验");
    else ElMessage.warning("删除尚未完成，请查看状态并重试清理");
    await loadDeletionStats();
  } catch (err: any) {
    if (err === "cancel" || err === "close") return;
    ElMessage.error("创建删除请求失败: " + (err?.response?.data?.message || err.message));
  } finally {
    deletionBusy.value = false;
  }
}

async function cleanupDeletion() {
  deletionBusy.value = true;
  try {
    const result = await runDeletionCleanup();
    const id = deletionStatus.value?.id;
    if (id) deletionStatus.value = await getDeletionStatus(id);
    await loadDeletionStats();
    ElMessage.success(`清理执行完成：${result.cleaned ?? 0} 项`);
  } catch (err: any) {
    ElMessage.error("清理失败: " + (err?.response?.data?.message || err.message));
  } finally {
    deletionBusy.value = false;
  }
}

async function testDeletion() {
  if (!deletionForm.targetId.trim()) {
    ElMessage.warning("请输入数据 ID");
    return;
  }
  deletionBusy.value = true;
  try {
    securityResults.value = await runDeletionSecurityTests({
      targetId: deletionForm.targetId.trim(),
      targetType: deletionForm.targetType,
    });
  } catch (err: any) {
    ElMessage.error("安全测试失败: " + (err?.response?.data?.message || err.message));
  } finally {
    deletionBusy.value = false;
  }
}

onMounted(loadDeletionStats);

async function runScan(scope: string[]) {
  scanning.value = true;
  scanSummary.value = null;
  try {
    const d = await postScan(scope);
    scanSummary.value = d;
    ElMessage.success(d.message || "扫描完成");
  } catch (err: any) {
    ElMessage.error(
      "扫描失败: " + (err.response?.data?.message || err.message),
    );
  } finally {
    scanning.value = false;
  }
}

async function onViewHistoryResult(scanId: number | string) {
  try {
    const result = await getScanResult(scanId);
    if (!result) {
      ElMessage.warning("未找到该历史扫描结果");
      return;
    }
    scanSummary.value = result;
    ElMessage.success("已加载历史扫描结果");
  } catch (err: any) {
    ElMessage.error("加载历史扫描失败: " + (err?.response?.data?.message || err.message));
  }
}
</script>

<style scoped>
.back-btn {
  padding: 4px;
}
.privacy-scan-view {
  max-width: 900px;
}
.page-header {
  margin-bottom: 16px;
}
.page-header-row {
  display: flex;
  align-items: center;
  gap: 8px;
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
.notice-alert {
  margin-bottom: 16px;
}
.notice-alert p {
  margin: 0;
  font-size: 13px;
}

.lifecycle-card {
  margin-top: 16px;
}
.lifecycle-desc {
  margin: 0 0 12px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.lifecycle-reason {
  margin-top: 12px;
}
.lifecycle-actions {
  display: flex;
  gap: 8px;
  margin: 12px 0;
  flex-wrap: wrap;
}
.lifecycle-stats,
.security-results {
  margin-top: 12px;
}

@media (max-width: 600px) {
  .privacy-scan-view {
    max-width: 100%;
  }
}
</style>
