<template>
  <el-dialog
    v-model="visible"
    :close-on-click-modal="false"
    :close-on-press-escape="false"
    :show-close="false"
    width="min(560px, calc(100vw - 32px))"
    append-to-body
  >
    <template #header>
      <div class="approval-header">
        <div>
          <strong>工具执行需要批准</strong>
          <span>{{ current?.toolName || "工具调用" }}</span>
        </div>
        <el-tag type="warning">请求批准</el-tag>
      </div>
    </template>
    <el-alert
      title="本次批准仅允许当前工具调用继续执行，不会更改会话的权限模式。"
      type="warning"
      show-icon
      :closable="false"
    />
    <dl class="approval-facts">
      <div>
        <dt>风险等级</dt>
        <dd>{{ current?.riskLevel || "未知" }}</dd>
      </div>
      <div>
        <dt>到期时间</dt>
        <dd>{{ formatTime(current?.expiresAt || "") }}</dd>
      </div>
    </dl>
    <section class="approval-arguments">
      <strong>参数</strong>
      <pre>{{ prettyArguments }}</pre>
    </section>
    <template #footer>
      <el-button :loading="resolving" @click="resolve(false)">拒绝</el-button>
      <el-button type="primary" :loading="resolving" @click="resolve(true)">本次允许</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { ElMessage } from "element-plus";
import { apiClient } from "@/composables/useApi";

type ApprovalItem = {
  id: string;
  conversationId: string;
  turnId: string;
  toolCallId?: string;
  toolName: string;
  arguments?: string;
  riskLevel?: string;
  expiresAt?: string;
};

const route = useRoute();
const pending = ref<ApprovalItem[]>([]);
const resolving = ref(false);

const conversationId = computed(() =>
  route.path.startsWith("/chat")
    ? String(route.query.conversationId || "").trim()
    : "",
);
const current = computed(() =>
  pending.value.find((item) => item.conversationId === conversationId.value),
);
const visible = computed({
  get: () => Boolean(current.value),
  set: () => undefined,
});
const prettyArguments = computed(() => {
  const raw = String(current.value?.arguments || "").trim();
  if (!raw) return "无参数";
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw.slice(0, 8000);
  }
});

function formatTime(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function onApprovalEvent(event: Event) {
  const detail = (event as CustomEvent<Record<string, any>>).detail || {};
  const action = String(detail.action || "");
  const targetConversationId = String(detail.conversationId || "").trim();
  if (action === "reset") {
    pending.value = pending.value.filter(
      (item) => item.conversationId !== targetConversationId,
    );
    return;
  }
  const id = String(detail.id || detail.approvalId || "").trim();
  if (!id) return;
  if (action === "resolved") {
    pending.value = pending.value.filter((item) => item.id !== id);
    return;
  }
  if (action !== "requested") return;
  const item: ApprovalItem = {
    id,
    conversationId: targetConversationId,
    turnId: String(detail.turnId || "").trim(),
    toolCallId: String(detail.toolCallId || "").trim() || undefined,
    toolName: String(detail.toolName || detail.tool || "工具调用"),
    arguments: String(detail.arguments || ""),
    riskLevel: String(detail.riskLevel || detail.risk || ""),
    expiresAt: String(detail.expiresAt || ""),
  };
  const index = pending.value.findIndex((candidate) => candidate.id === id);
  if (index >= 0) pending.value[index] = item;
  else pending.value.push(item);
}

async function resolve(approved: boolean) {
  const item = current.value;
  if (!item) return;
  if (!item.conversationId || !item.turnId || !item.id) {
    ElMessage.error("审批请求缺少 Conversation 或 Turn 绑定");
    return;
  }
  resolving.value = true;
  try {
    await apiClient.post(
      `/api/web-chat/conversations/${encodeURIComponent(item.conversationId)}/turns/${encodeURIComponent(item.turnId)}/approvals/${encodeURIComponent(item.id)}`,
      { approved },
    );
  } catch (error: any) {
    const status = error?.response?.status;
    if (status === 400 || status === 404) {
      pending.value = pending.value.filter((candidate) => candidate.id !== item.id);
    } else {
      ElMessage.error(error?.response?.data?.msg || "审批处理失败");
    }
  } finally {
    resolving.value = false;
  }
}

watch(conversationId, () => {
  resolving.value = false;
});

onMounted(() => {
  window.addEventListener("amitia:agent-approval", onApprovalEvent);
});

onBeforeUnmount(() => {
  window.removeEventListener("amitia:agent-approval", onApprovalEvent);
});
</script>

<style scoped>
.approval-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.approval-header strong,
.approval-header span {
  display: block;
}

.approval-header strong {
  font-size: 15px;
}

.approval-header span {
  margin-top: 3px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.approval-facts {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin: 14px 0;
}

.approval-facts div {
  border-radius: 8px;
  padding: 9px 10px;
  background: var(--el-fill-color-light);
}

.approval-facts dt {
  color: var(--el-text-color-secondary);
  font-size: 11px;
}

.approval-facts dd {
  margin: 4px 0 0;
  color: var(--el-text-color-primary);
  font-size: 13px;
}

.approval-arguments strong {
  font-size: 12px;
}

.approval-arguments pre {
  max-height: 260px;
  margin: 7px 0 0;
  overflow: auto;
  border-radius: 8px;
  padding: 10px;
  background: var(--el-fill-color-light);
  color: var(--el-text-color-regular);
  font: 11px/1.55 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

@media (max-width: 560px) {
  .approval-facts {
    grid-template-columns: 1fr;
  }
}
</style>
