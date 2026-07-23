<template>
  <el-dialog
    v-model="visible"
    :close-on-click-modal="false"
    :close-on-press-escape="false"
    :show-close="false"
    width="min(640px, calc(100vw - 32px))"
    append-to-body
  >
    <template #header>
      <div class="guard-header">
        <div>
          <strong>{{ title }}</strong
          ><span>{{ current?.serverName || current?.serverId }}</span>
        </div>
        <el-tag type="warning">外部 MCP 请求</el-tag>
      </div>
    </template>
    <template
      v-if="current?.kind === 'sampling' || current?.kind === 'sampling_result'"
    >
      <el-alert
        :title="
          current.kind === 'sampling_result'
            ? '模型已生成结果。请审核内容，批准后才会返回给该 MCP 服务。'
            : '此服务请求 Amitia 调用一次模型。只有本次批准有效，模型密钥、角色记忆和会话历史不会提供给服务。'
        "
        type="warning"
        show-icon
        :closable="false"
      />
      <dl
        class="request-facts"
        :class="{ 'result-facts': current.kind === 'sampling_result' }"
      >
        <template v-if="current.kind === 'sampling'"
          ><div>
            <dt>最大 Token</dt>
            <dd>{{ samplingMaxTokens }}</dd>
          </div>
          <div>
            <dt>消息数</dt>
            <dd>{{ samplingMessages.length }}</dd>
          </div></template
        >
        <div>
          <dt>到期时间</dt>
          <dd>{{ formatTime(current.expiresAt) }}</dd>
        </div>
      </dl>
      <section class="request-preview">
        <strong>{{
          current.kind === "sampling_result" ? "模型结果" : "请求摘要"
        }}</strong
        ><template v-if="current.kind === 'sampling_result'"
          ><div>
            <span>助手</span>
            <p>{{ contentText(current.request.content) }}</p>
          </div></template
        ><template v-else
          ><div v-for="(message, index) in samplingMessages" :key="index">
            <span>{{ roleLabel(message.role) }}</span>
            <p>{{ contentText(message.content) }}</p>
          </div></template
        >
      </section>
    </template>
    <template v-else-if="current">
      <el-alert
        :title="
          elicitationMode === 'url'
            ? '服务请求你确认打开一个外部网页。页面不会被预取或自动打开。'
            : '服务请求你填写非敏感信息。密码、Token、支付凭证和私钥字段会被后端直接拒绝。'
        "
        type="info"
        show-icon
        :closable="false"
      />
      <div v-if="elicitationReason" class="reason">
        <strong>请求原因</strong>
        <p>{{ elicitationReason }}</p>
      </div>
      <div v-if="elicitationMode === 'url'" class="url-panel">
        <span>目标域名：{{ targetHost }}</span
        ><code>{{ targetURL }}</code>
      </div>
      <el-form v-else label-position="top" class="elicitation-form">
        <el-form-item
          v-for="field in formFields"
          :key="field.name"
          :label="field.title"
          :required="field.required"
        >
          <el-select
            v-if="field.enumValues.length"
            v-model="formValues[field.name]"
            clearable
            ><el-option
              v-for="option in field.enumValues"
              :key="String(option)"
              :label="String(option)"
              :value="option"
          /></el-select>
          <el-switch
            v-else-if="field.type === 'boolean'"
            v-model="formValues[field.name]"
          />
          <el-input-number
            v-else-if="field.type === 'number' || field.type === 'integer'"
            v-model="formValues[field.name]"
            :min="field.minimum"
            :max="field.maximum"
          />
          <el-input
            v-else
            v-model="formValues[field.name]"
            :type="field.multiline ? 'textarea' : 'text'"
            :maxlength="field.maxLength"
            :placeholder="field.description"
          />
          <div v-if="field.description" class="field-help">
            {{ field.description }}
          </div>
        </el-form-item>
      </el-form>
    </template>
    <template #footer>
      <el-button :loading="resolving" @click="resolve('cancel')"
        >取消</el-button
      >
      <el-button
        type="danger"
        plain
        :loading="resolving"
        @click="resolve('decline')"
        >拒绝</el-button
      >
      <el-button type="primary" :loading="resolving" @click="accept">{{
        elicitationMode === "url" && current?.kind === "elicitation"
          ? "同意并打开"
          : "批准"
      }}</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import {
  computed,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
  watch,
} from "vue";
import { ElMessage } from "element-plus";
import { listMCPInteractions, resolveMCPInteraction } from "@/views/mcp/api";
import type { MCPPendingInteraction } from "@/views/mcp/types";

const current = ref<MCPPendingInteraction>();
const resolving = ref(false);
const formValues = reactive<Record<string, any>>({});
let timer: ReturnType<typeof setInterval> | undefined;
const visible = computed({
  get: () => Boolean(current.value),
  set: () => undefined,
});
const title = computed(() =>
  current.value?.kind === "sampling"
    ? "批准模型 Sampling 请求"
    : current.value?.kind === "sampling_result"
      ? "审核 Sampling 结果"
      : "处理 Elicitation 请求",
);
const samplingMessages = computed(() =>
  Array.isArray(current.value?.request?.messages)
    ? current.value!.request.messages
    : [],
);
const samplingMaxTokens = computed(
  () => Number(current.value?.request?.maxTokens || 0) || "由客户端限额决定",
);
const elicitationMode = computed(() =>
  String(current.value?.request?.mode || "form"),
);
const elicitationReason = computed(() =>
  String(
    current.value?.request?.message || current.value?.request?.reason || "",
  ),
);
const targetURL = computed(() => String(current.value?.request?.url || ""));
const targetHost = computed(() => {
  try {
    return new URL(targetURL.value).hostname;
  } catch {
    return "无效地址";
  }
});
const formFields = computed(() => {
  const schema =
    current.value?.request?.requestedSchema ||
    current.value?.request?.schema ||
    {};
  const required = Array.isArray(schema.required)
    ? schema.required.map(String)
    : [];
  return Object.entries(schema.properties || {})
    .slice(0, 50)
    .map(([name, raw]: [string, any]) => ({
      name,
      title: String(raw?.title || name),
      description: String(raw?.description || ""),
      type: String(raw?.type || "string"),
      required: required.includes(name),
      enumValues: Array.isArray(raw?.enum) ? raw.enum.slice(0, 100) : [],
      minimum: Number.isFinite(raw?.minimum) ? raw.minimum : undefined,
      maximum: Number.isFinite(raw?.maximum) ? raw.maximum : undefined,
      maxLength: Number.isFinite(raw?.maxLength)
        ? Math.min(raw.maxLength, 10000)
        : 10000,
      multiline: raw?.format === "textarea" || Number(raw?.maxLength) > 200,
    }));
});

watch(current, () => {
  for (const key of Object.keys(formValues)) delete formValues[key];
  for (const field of formFields.value)
    formValues[field.name] =
      field.type === "boolean"
        ? false
        : field.type === "number" || field.type === "integer"
          ? undefined
          : "";
});
async function load() {
  if (!localStorage.getItem("ai-companion-token") || current.value) return;
  try {
    current.value = (await listMCPInteractions())[0];
  } catch {}
}
async function resolve(action: "accept" | "decline" | "cancel") {
  if (!current.value) return;
  resolving.value = true;
  try {
    await resolveMCPInteraction(
      current.value.id,
      action,
      action === "accept" ? { ...formValues } : {},
    );
    current.value = undefined;
    await load();
  } catch (error: any) {
    if (error?.response?.status === 400) current.value = undefined;
    else ElMessage.error(error?.response?.data?.detail || "请求处理失败");
  } finally {
    resolving.value = false;
  }
}
async function accept() {
  if (!current.value) return;
  for (const field of formFields.value) {
    if (
      field.required &&
      (formValues[field.name] === "" ||
        formValues[field.name] === undefined ||
        formValues[field.name] === null)
    ) {
      ElMessage.warning(`请填写“${field.title}”`);
      return;
    }
  }
  const url = elicitationMode.value === "url" ? targetURL.value : "";
  await resolve("accept");
  if (url) window.open(url, "_blank", "noopener,noreferrer");
}
function roleLabel(value: unknown) {
  return String(value) === "assistant" ? "助手" : "用户";
}
function contentText(value: unknown) {
  if (typeof value === "string") return value.slice(0, 1000);
  try {
    return JSON.stringify(value).slice(0, 1000);
  } catch {
    return "无法显示";
  }
}
function formatTime(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}
onMounted(() => {
  void load();
  timer = setInterval(load, 2000);
});
onBeforeUnmount(() => {
  if (timer) clearInterval(timer);
});
</script>

<style scoped>
.guard-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding-right: 12px;
}
.guard-header > div {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.guard-header strong {
  color: var(--ac-color-text);
  font-size: 18px;
}
.guard-header span,
.field-help {
  color: var(--ac-color-text-secondary);
  font-size: var(--ac-font-size-xs);
}
.request-facts {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
  margin: 16px 0;
}
.request-facts.result-facts {
  grid-template-columns: 1fr;
}
.request-facts div {
  padding: 10px 12px;
  border: 1px solid var(--console-border);
  border-radius: 8px;
}
.request-facts dt {
  color: var(--ac-color-text-secondary);
  font-size: var(--ac-font-size-xs);
}
.request-facts dd {
  margin: 4px 0 0;
  color: var(--ac-color-text);
  font-weight: 600;
}
.request-preview {
  max-height: 260px;
  overflow: auto;
  padding: 12px;
  border: 1px solid var(--console-border);
  border-radius: 8px;
}
.request-preview > div {
  margin-top: 10px;
}
.request-preview span {
  color: var(--ac-color-primary);
  font-size: var(--ac-font-size-xs);
  font-weight: 600;
}
.request-preview p,
.reason p {
  margin: 4px 0 0;
  color: var(--ac-color-text-secondary);
  line-height: 1.6;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.reason,
.url-panel,
.elicitation-form {
  margin-top: 16px;
}
.url-panel {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  border: 1px solid var(--console-border);
  border-radius: 8px;
}
.url-panel span {
  color: var(--ac-color-text-secondary);
  font-size: var(--ac-font-size-xs);
}
.url-panel code {
  color: var(--ac-color-text);
  overflow-wrap: anywhere;
}
.elicitation-form {
  max-height: 360px;
  overflow: auto;
  padding-right: 4px;
}
@media (max-width: 560px) {
  .request-facts {
    grid-template-columns: 1fr;
  }
}
</style>
