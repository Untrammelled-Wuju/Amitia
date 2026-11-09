<template>
  <div class="attention-center">
    <header>
      <div>
        <h2>需要你处理</h2>
        <p>审批、等待和失败恢复仍由核心安全边界负责。</p>
      </div>
      <el-button :loading="loading" @click="loadItems">刷新</el-button>
    </header>
    <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon />
    <div v-loading="loading" class="attention-list">
      <el-card v-for="item in items" :key="item.id" shadow="never" class="attention-card">
        <div class="attention-title">
          <el-tag :type="tagType(item)" effect="light" round>{{ kindLabel(item.kind) }}</el-tag>
          <strong>{{ item.title }}</strong>
          <span>{{ formatTime(item.createdAt) }}</span>
        </div>
        <p v-if="item.summary">{{ item.summary }}</p>
        <small>{{ item.reason }}</small>
        <div class="attention-actions">
          <template v-if="item.kind === 'agent_approval'">
            <el-button size="small" @click="resolveAgentApproval(item, false)">拒绝</el-button>
            <el-button size="small" type="primary" @click="resolveAgentApproval(item, true)">本次允许</el-button>
          </template>
          <template v-else-if="item.kind === 'task_approval'">
            <el-button size="small" @click="resolveTaskApproval(item, false)">拒绝</el-button>
            <el-button size="small" type="primary" @click="resolveTaskApproval(item, true)">批准本次执行</el-button>
          </template>
          <el-button v-if="item.route" size="small" @click="router.push(item.route)">打开</el-button>
        </div>
      </el-card>
      <el-empty v-if="!loading && !items.length" description="现在没有需要你处理的事项" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import { apiClient } from "@/composables/useApi";
import { ownedChatRuntimeRequest } from "@/runtime/owned-chat-management";
import { decideSourceTaskApproval } from "@/views/kernel/tasks/api";

interface AttentionItem {
  id: string;
  kind: string;
  priority: number;
  title: string;
  summary: string;
  reason: string;
  roleId?: string;
  conversationId?: string;
  turnId?: string;
  approvalId?: string;
  route?: string;
  createdAt: string;
  payload?: Record<string, any>;
}

const route = useRoute();
const router = useRouter();
const loading = ref(false);
const error = ref("");
const items = ref<AttentionItem[]>([]);

function kindLabel(kind: string) {
  return ({
    agent_approval: "工具审批",
    task_approval: "设备审批",
    continuity: "事项等待",
    task_failed: "任务失败",
    task_blocked: "任务等待",
    task_result: "任务结果",
    automation_failed: "自动化失败",
    automation_approval: "自动化确认",
    source_error: "读取异常",
  } as Record<string, string>)[kind] || kind;
}

function tagType(item: AttentionItem): "danger" | "warning" | "success" | "info" {
  if (["task_failed", "automation_failed"].includes(item.kind)) return "danger";
  if (["automation_approval", "agent_approval", "task_approval"].includes(item.kind)) return "warning";
  if (item.kind === "task_result") return "success";
  return "info";
}

function formatTime(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

async function loadItems() {
  loading.value = true;
  error.value = "";
  try {
    const response = await apiClient.get<AttentionItem[]>("/api/attention/items");
    items.value = Array.isArray(response.data) ? response.data : [];
    const attentionId = String(route.query.attentionId || "").trim();
    if (attentionId) items.value = [...items.value].sort((left, right) => Number(right.id === attentionId) - Number(left.id === attentionId));
  } catch (reason: any) {
    error.value = reason?.response?.data?.msg || reason?.message || String(reason);
  } finally {
    loading.value = false;
  }
}

async function resolveAgentApproval(item: AttentionItem, approved: boolean) {
  if (!item.conversationId || !item.turnId || !item.approvalId) throw new Error("审批缺少会话或回合绑定");
  await ownedChatRuntimeRequest("approval", {
    conversationId: item.conversationId,
    turnId: item.turnId,
    approvalId: item.approvalId,
    approved,
  }, item.roleId || "");
  ElMessage.success(approved ? "本次工具调用已允许" : "本次工具调用已拒绝");
  await loadItems();
}

async function resolveTaskApproval(item: AttentionItem, approved: boolean) {
  const binding = item.payload?.binding;
  const revision = Number(item.payload?.revision);
  if (!binding) throw new Error("设备审批缺少授权绑定");
  await decideSourceTaskApproval({ id: item.approvalId, binding, revision, status: "pending" } as any, approved);
  ElMessage.success(approved ? "设备任务已批准" : "设备任务已拒绝");
  await loadItems();
}

onMounted(loadItems);
</script>

<style scoped>
.attention-center { display: grid; gap: 16px; max-width: 960px; padding: 20px; }
.attention-center header { display: flex; justify-content: space-between; gap: 16px; align-items: flex-start; }
.attention-center h2 { margin: 0 0 4px; }
.attention-center header p { margin: 0; color: var(--el-text-color-secondary); }
.attention-list { display: grid; gap: 10px; min-height: 160px; }
.attention-card { display: grid; gap: 8px; }
.attention-title, .attention-actions { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.attention-title strong { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.attention-title span { margin-left: auto; color: var(--el-text-color-secondary); font-size: 12px; }
.attention-card p, .attention-card small { margin: 0; color: var(--el-text-color-secondary); }
</style>
