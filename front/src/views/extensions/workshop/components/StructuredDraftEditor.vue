<template>
  <section class="draft-editor" aria-label="结构化 Draft 编辑器">
    <el-alert
      title="编辑内容只作为声明式 JSON 保存，不会执行其中的字符串"
      type="info"
      :closable="false"
      show-icon
    />
    <el-tabs v-model="activeTab" class="editor-tabs">
      <el-tab-pane label="基础信息" name="metadata">
        <el-form label-position="top" class="form-grid">
          <el-form-item label="Skill ID"
            ><el-input v-model="editable.metadata.id" autocomplete="off"
          /></el-form-item>
          <el-form-item label="版本"
            ><el-input v-model="editable.metadata.version" autocomplete="off"
          /></el-form-item>
          <el-form-item label="名称"
            ><el-input v-model="editable.metadata.name" autocomplete="off"
          /></el-form-item>
          <el-form-item label="作者"
            ><el-input v-model="editable.metadata.author" autocomplete="off"
          /></el-form-item>
          <el-form-item label="描述" class="wide"
            ><el-input
              v-model="editable.metadata.description"
              type="textarea"
              :rows="3"
          /></el-form-item>
        </el-form>
      </el-tab-pane>

      <el-tab-pane label="Schema" name="schemas">
        <el-alert
          title="支持 JSON Schema 2020-12 常用字段；失焦和保存时执行语法与结构检查"
          type="info"
          :closable="false"
        />
        <div class="schema-grid">
          <label
            v-for="item in schemaEditors"
            :key="item.key"
            class="editor-field"
          >
            <span>{{ item.label }}</span>
            <small>{{ item.hint }}</small>
            <div class="schema-hints">
              <el-tag
                v-for="keyword in schemaKeywords"
                :key="keyword"
                size="small"
                type="info"
                >{{ keyword }}</el-tag
              >
            </div>
            <el-input
              v-model="item.text.value"
              type="textarea"
              :rows="14"
              spellcheck="false"
              class="json-input"
              @blur="validateAll"
            />
          </label>
        </div>
      </el-tab-pane>

      <el-tab-pane label="工作流" name="workflow">
        <div class="capability-preview" aria-live="polite">
          <div>
            <strong>直接推导权限</strong
            ><el-tag
              v-for="item in inferredCapabilities"
              :key="item"
              size="small"
              >{{ item }}</el-tag
            ><span v-if="!inferredCapabilities.length">无</span>
          </div>
          <div v-if="addedCapabilities.length">
            <strong>新增</strong
            ><el-tag
              v-for="item in addedCapabilities"
              :key="item"
              type="warning"
              size="small"
              >{{ item }}</el-tag
            >
          </div>
          <div v-if="removedCapabilities.length">
            <strong>移除</strong
            ><el-tag
              v-for="item in removedCapabilities"
              :key="item"
              type="info"
              size="small"
              >{{ item }}</el-tag
            >
          </div>
          <small>call_skill 的递归权限与副作用由服务端保存后重新分析。</small>
        </div>
        <fieldset
          v-for="(step, index) in editable.workflow.steps"
          :key="`${step.id}-${index}`"
          class="step-card"
        >
          <legend>步骤 {{ index + 1 }}</legend>
          <div class="step-toolbar">
            <el-form-item label="步骤 ID"
              ><el-input v-model="step.id"
            /></el-form-item>
            <el-form-item label="白名单类型">
              <el-select v-model="step.type" @change="resetStepInput(index)"
                ><el-option
                  v-for="kind in allowedSteps"
                  :key="kind"
                  :label="kind"
                  :value="kind"
              /></el-select>
            </el-form-item>
            <el-form-item label="失败策略">
              <el-select v-model="step.onError.mode"
                ><el-option label="fail" value="fail" /><el-option
                  label="continue"
                  value="continue" /><el-option
                  label="use_default"
                  value="use_default"
              /></el-select>
            </el-form-item>
            <el-button
              type="danger"
              plain
              :disabled="editable.workflow.steps.length === 1"
              @click="removeStep(index)"
              >移除</el-button
            >
          </div>
          <p class="parameter-hint">
            <strong>参数提示：</strong>{{ stepHints[step.type] }}
          </p>
          <div class="parameter-form" aria-label="步骤参数 Schema 表单">
            <el-form-item
              v-for="field in stepFields[step.type]"
              :key="field.key"
              :label="`${field.label}${field.required ? ' *' : ''}`"
            >
              <el-select
                v-if="field.options"
                :model-value="stepFieldValue(index, field.key)"
                @update:model-value="setStepField(index, field.key, $event)"
                ><el-option
                  v-for="option in field.options"
                  :key="option"
                  :label="option"
                  :value="option"
              /></el-select>
              <el-input-number
                v-else-if="field.kind === 'number'"
                :model-value="Number(stepFieldValue(index, field.key) || 0)"
                :min="field.min"
                :max="field.max"
                controls-position="right"
                @update:model-value="setStepField(index, field.key, $event)"
              />
              <el-input
                v-else
                :model-value="String(stepFieldValue(index, field.key) ?? '')"
                @update:model-value="setStepField(index, field.key, $event)"
              />
            </el-form-item>
          </div>
          <div class="reference-hints">
            <strong>可用引用自动提示</strong
            ><code
              v-for="reference in referencesForStep(index)"
              :key="reference"
              >{{ reference }}</code
            >
          </div>
          <label class="editor-field"
            ><span>步骤参数 JSON</span
            ><el-input
              v-model="stepInputTexts[index]"
              type="textarea"
              :rows="8"
              spellcheck="false"
              class="json-input"
              @blur="validateAll"
          /></label>
          <label class="editor-field"
            ><span>when 条件 JSON（留空表示总是执行）</span
            ><el-input
              v-model="stepWhenTexts[index]"
              type="textarea"
              :rows="4"
              spellcheck="false"
              class="json-input"
              @blur="validateAll"
          /></label>
          <label v-if="step.onError.mode === 'use_default'" class="editor-field"
            ><span>默认结果 JSON</span
            ><el-input
              v-model="stepDefaultTexts[index]"
              type="textarea"
              :rows="4"
              spellcheck="false"
              class="json-input"
              @blur="validateAll"
          /></label>
        </fieldset>
        <el-button plain @click="addStep">添加白名单步骤</el-button>
        <label class="editor-field output-editor"
          ><span>最终 Output Mapping JSON</span
          ><el-input
            v-model="workflowOutputText"
            type="textarea"
            :rows="6"
            spellcheck="false"
            class="json-input"
            @blur="validateAll"
        /></label>
      </el-tab-pane>

      <el-tab-pane label="测试与 Mock" name="tests">
        <el-alert
          title="Mock 仅绑定当前测试用例和新 Revision，不会写入生产配置"
          type="info"
          :closable="false"
        />
        <fieldset
          v-for="(testCase, index) in editable.testCases"
          :key="`${testCase.id}-${index}`"
          class="test-card"
        >
          <legend>测试用例 {{ index + 1 }}</legend>
          <div class="test-toolbar">
            <el-form-item label="ID"
              ><el-input v-model="testCase.id"
            /></el-form-item>
            <el-form-item label="名称"
              ><el-input v-model="testCase.name"
            /></el-form-item>
            <el-form-item label="模式"
              ><el-select v-model="testCase.mode"
                ><el-option label="Dry Run" value="dry_run" /><el-option
                  label="Mocked"
                  value="mocked" /><el-option
                  label="Controlled Live"
                  value="controlled_live" /></el-select
            ></el-form-item>
            <el-button type="danger" plain @click="removeTestCase(index)"
              >移除</el-button
            >
          </div>
          <div class="test-json-grid">
            <label class="editor-field"
              ><span>输入 JSON</span
              ><el-input
                v-model="testEditors[index].input"
                type="textarea"
                :rows="7"
                spellcheck="false"
                class="json-input"
            /></label>
            <label class="editor-field"
              ><span>配置 JSON</span
              ><el-input
                v-model="testEditors[index].config"
                type="textarea"
                :rows="7"
                spellcheck="false"
                class="json-input"
            /></label>
            <label class="editor-field"
              ><span>预期输出 JSON</span
              ><el-input
                v-model="testEditors[index].expectedOutput"
                type="textarea"
                :rows="7"
                spellcheck="false"
                class="json-input"
            /></label>
            <label class="editor-field"
              ><span>断言 JSON 数组</span
              ><small
                >equals、exists、matches_schema、status_is、step_succeeded、side_effect_count
                等</small
              ><el-input
                v-model="testEditors[index].assertions"
                type="textarea"
                :rows="7"
                spellcheck="false"
                class="json-input"
            /></label>
          </div>
          <el-collapse class="mock-panels">
            <el-collapse-item title="HTTP Mock" name="http"
              ><p class="parameter-hint">
                数组项支持
                method、url、query、headers、body、status、responseHeaders、responseBody、delayMs、error。
              </p>
              <el-input
                v-model="testEditors[index].httpMocks"
                type="textarea"
                :rows="10"
                spellcheck="false"
                class="json-input"
            /></el-collapse-item>
            <el-collapse-item title="Skill Mock" name="skill"
              ><p class="parameter-hint">
                数组项支持
                skillId、input、output、status、delayMs、error、sideEffects。
              </p>
              <el-input
                v-model="testEditors[index].skillMocks"
                type="textarea"
                :rows="10"
                spellcheck="false"
                class="json-input"
            /></el-collapse-item>
          </el-collapse>
        </fieldset>
        <el-button plain @click="addTestCase">添加测试用例</el-button>
      </el-tab-pane>
    </el-tabs>
    <div
      v-if="errors.length"
      class="error-summary"
      role="alert"
      aria-live="assertive"
    >
      <strong>无法保存，请修复以下字段：</strong>
      <ul>
        <li v-for="item in errors" :key="item">{{ item }}</li>
      </ul>
    </div>
    <div class="editor-actions">
      <el-button @click="$emit('cancel')">取消</el-button
      ><el-button type="primary" :loading="saving" @click="submit"
        >保存为新 Revision</el-button
      >
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from "vue";
import type {
  ExtensionDraft,
  WorkflowStep,
  WorkshopTestCase,
} from "../../types";

const props = defineProps<{ draft: ExtensionDraft; saving: boolean }>();
const emit = defineEmits<{ save: [draft: ExtensionDraft]; cancel: [] }>();
const clone = <T,>(value: T): T => JSON.parse(JSON.stringify(value));
const editable = reactive<ExtensionDraft>(clone(props.draft));
const activeTab = ref("metadata");
const errors = ref<string[]>([]);
const allowedSteps: WorkflowStep["type"][] = [
  "http",
  "condition",
  "transform",
  "template",
  "call_skill",
  "schedule",
  "notification",
  "memory_candidate",
  "context_contribution",
];
const transformOperations = new Set([
  "pick",
  "omit",
  "rename",
  "set",
  "merge",
  "flatten",
  "array_map",
  "array_filter",
  "array_take",
  "array_sort",
  "to_string",
  "to_number",
  "to_boolean",
]);
type StepField = {
  key: string;
  label: string;
  required?: boolean;
  kind?: "number";
  min?: number;
  max?: number;
  options?: string[];
};
const stepFields: Record<WorkflowStep["type"], StepField[]> = {
  http: [
    { key: "url", label: "HTTPS URL", required: true },
    {
      key: "method",
      label: "Method",
      options: ["GET", "POST", "PUT", "PATCH", "DELETE"],
    },
    { key: "timeoutMs", label: "超时毫秒", kind: "number", min: 1, max: 10000 },
    {
      key: "maxResponseBytes",
      label: "响应上限",
      kind: "number",
      min: 1,
      max: 1048576,
    },
  ],
  condition: [
    {
      key: "op",
      label: "条件操作符",
      required: true,
      options: [
        "eq",
        "neq",
        "gt",
        "gte",
        "lt",
        "lte",
        "and",
        "or",
        "not",
        "exists",
        "empty",
        "contains",
        "starts_with",
        "ends_with",
        "in",
      ],
    },
  ],
  transform: [
    {
      key: "op",
      label: "转换操作",
      required: true,
      options: Array.from(transformOperations),
    },
  ],
  template: [{ key: "template", label: "安全模板", required: true }],
  call_skill: [
    { key: "skillId", label: "Skill ID", required: true },
    { key: "timeoutMs", label: "超时毫秒", kind: "number", min: 1, max: 10000 },
  ],
  schedule: [
    { key: "timezone", label: "时区", required: true },
    { key: "idempotencyKey", label: "幂等键", required: true },
    { key: "dueAt", label: "执行时间" },
  ],
  notification: [
    { key: "content", label: "通知内容", required: true },
    { key: "recipient", label: "收件范围", options: ["current_conversation"] },
  ],
  memory_candidate: [{ key: "source", label: "候选记忆来源", required: true }],
  context_contribution: [
    { key: "content", label: "上下文内容", required: true },
    {
      key: "tokenLimit",
      label: "Token 上限",
      required: true,
      kind: "number",
      min: 1,
      max: 1024,
    },
  ],
};
const schemaKeywords = [
  "type",
  "properties",
  "required",
  "items",
  "enum",
  "format",
  "additionalProperties",
];
const stepHints: Record<WorkflowStep["type"], string> = {
  http: "url 必填；method 限 GET/POST/PUT/PATCH/DELETE；建议设置 timeoutMs、maxResponseBytes、expectedStatus、responseType。",
  condition:
    "使用 op、left/right、value 或 args 组成受限条件 AST，不支持脚本表达式。",
  transform:
    "op 限 pick、omit、rename、set、merge、flatten、array_map、array_filter、array_take、array_sort、to_string、to_number、to_boolean。",
  template:
    "template 必填，只允许 {{ input.field }} 等简单引用及受控格式化器。",
  call_skill:
    "skillId 必填；可配置 input、optional、timeoutMs、resultMapping。",
  schedule: "timezone 与 idempotencyKey 必填，并提供 dueAt 或 cron。",
  notification: "content 必填；recipient 只能为空或 current_conversation。",
  memory_candidate: "source 必填，只提交候选记忆。",
  context_contribution: "content 与 tokenLimit 必填，tokenLimit 范围 1–1024。",
};
const stepExamples: Record<WorkflowStep["type"], Record<string, unknown>> = {
  http: {
    url: "https://example.com/api",
    method: "GET",
    timeoutMs: 5000,
    maxResponseBytes: 262144,
    expectedStatus: [200],
    responseType: "json",
  },
  condition: { op: "exists", value: { ref: "input.value" } },
  transform: { op: "pick", value: { $ref: "input" }, fields: [] },
  template: { template: "{{ input.value }}" },
  call_skill: {
    skillId: "dev.user.skill.example",
    input: {},
    optional: false,
    timeoutMs: 5000,
  },
  schedule: {
    timezone: "Asia/Shanghai",
    dueAt: "2026-08-01T10:00:00+08:00",
    idempotencyKey: "{{ runtime.runId }}",
  },
  notification: {
    content: "{{ input.message }}",
    recipient: "current_conversation",
  },
  memory_candidate: { source: "{{ input.message }}" },
  context_contribution: { content: "{{ input.context }}", tokenLimit: 256 },
};
const inputSchemaText = ref(pretty(editable.inputSchema));
const outputSchemaText = ref(pretty(editable.outputSchema));
const configSchemaText = ref(pretty(editable.configSchema));
const defaultConfigText = ref(pretty(editable.defaultConfig));
const workflowOutputText = ref(pretty(editable.workflow.output));
const stepInputTexts = ref(
  editable.workflow.steps.map((step) => pretty(step.input)),
);
const stepWhenTexts = ref(
  editable.workflow.steps.map((step) => (step.when ? pretty(step.when) : "")),
);
const stepDefaultTexts = ref(
  editable.workflow.steps.map((step) =>
    step.onError.default === undefined ? "" : pretty(step.onError.default),
  ),
);
type TestEditor = {
  input: string;
  config: string;
  expectedOutput: string;
  assertions: string;
  httpMocks: string;
  skillMocks: string;
};
const testEditors = ref<TestEditor[]>(editable.testCases.map(testEditor));
const schemaEditors = [
  {
    key: "input",
    label: "Input Schema",
    hint: "定义调用方可提交的字段",
    text: inputSchemaText,
  },
  {
    key: "output",
    label: "Output Schema",
    hint: "定义 SkillResult 输出结构",
    text: outputSchemaText,
  },
  {
    key: "config",
    label: "Config Schema",
    hint: "Secret 字段使用 writeOnly 或 format: secret/password",
    text: configSchemaText,
  },
  {
    key: "defaults",
    label: "Default Config",
    hint: "必须符合 Config Schema，禁止 Secret 明文",
    text: defaultConfigText,
  },
];
const capabilityByStep: Partial<Record<WorkflowStep["type"], string>> = {
  http: "network.https",
  schedule: "scheduler.own.manage",
  notification: "notification.send",
  memory_candidate: "memory.candidate.write",
  context_contribution: "context.inject",
};
const inferredCapabilities = computed(() =>
  Array.from(
    new Set(
      editable.workflow.steps
        .map((step) => capabilityByStep[step.type])
        .filter((item): item is string => Boolean(item)),
    ),
  ).sort(),
);
const addedCapabilities = computed(() =>
  inferredCapabilities.value.filter(
    (item) => !props.draft.capabilities.includes(item),
  ),
);
const removedCapabilities = computed(() =>
  props.draft.capabilities.filter(
    (item) =>
      !inferredCapabilities.value.includes(item) &&
      !editable.workflow.steps.some((step) => step.type === "call_skill"),
  ),
);

function pretty(value: unknown) {
  return JSON.stringify(value ?? {}, null, 2);
}
function stepFieldValue(index: number, key: string) {
  try {
    return JSON.parse(stepInputTexts.value[index] || "{}")[key];
  } catch {
    return "";
  }
}
function setStepField(index: number, key: string, value: unknown) {
  let input: Record<string, unknown>;
  try {
    input = JSON.parse(stepInputTexts.value[index] || "{}");
  } catch {
    input = {};
  }
  input[key] = value;
  stepInputTexts.value[index] = pretty(input);
  editable.workflow.steps[index].input = input;
}
function schemaReferences(text: string, prefix: string) {
  try {
    const value = JSON.parse(text);
    return Object.keys(value?.properties || {})
      .sort()
      .map((key) => `${prefix}.${key}`);
  } catch {
    return [];
  }
}
function referencesForStep(index: number) {
  const references = [
    ...schemaReferences(inputSchemaText.value, "input"),
    ...schemaReferences(configSchemaText.value, "config"),
    "runtime.traceId",
    "runtime.runId",
    "runtime.characterId",
    "runtime.conversationId",
    "runtime.channel",
  ];
  const outputs: Partial<Record<WorkflowStep["type"], string[]>> = {
    http: ["status", "body", "headers"],
    condition: ["result"],
    template: ["text"],
    schedule: ["planned"],
    notification: ["planned"],
    memory_candidate: ["planned"],
    context_contribution: ["planned"],
  };
  for (const step of editable.workflow.steps.slice(0, index))
    for (const field of outputs[step.type] || [])
      references.push(`steps.${step.id}.${field}`);
  return references;
}
function testEditor(testCase: WorkshopTestCase): TestEditor {
  return {
    input: pretty(testCase.input),
    config: pretty(testCase.config),
    expectedOutput:
      testCase.expectedOutput === undefined
        ? ""
        : pretty(testCase.expectedOutput),
    assertions: pretty(testCase.assertions || []),
    httpMocks: pretty(testCase.httpMocks || []),
    skillMocks: pretty(testCase.skillMocks || []),
  };
}
function parseJSON(text: string, path: string, required = true): any {
  if (!text.trim() && !required) return undefined;
  try {
    return JSON.parse(text);
  } catch (error: any) {
    errors.value.push(`${path}：JSON 语法错误（${error.message}）`);
    return undefined;
  }
}
function validateSchema(value: any, path: string) {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    errors.value.push(`${path}：必须是 JSON Schema 对象`);
    return;
  }
  const validTypes = new Set([
    "object",
    "array",
    "string",
    "number",
    "integer",
    "boolean",
    "null",
  ]);
  if (typeof value.type === "string" && !validTypes.has(value.type))
    errors.value.push(`${path}.type：未知 JSON Schema 类型 ${value.type}`);
  if (
    value.required !== undefined &&
    (!Array.isArray(value.required) ||
      value.required.some((item: unknown) => typeof item !== "string"))
  )
    errors.value.push(`${path}.required：必须是字符串数组`);
  if (
    value.properties !== undefined &&
    (!value.properties ||
      typeof value.properties !== "object" ||
      Array.isArray(value.properties))
  )
    errors.value.push(`${path}.properties：必须是对象`);
  if (
    value.additionalProperties !== undefined &&
    typeof value.additionalProperties !== "boolean" &&
    (typeof value.additionalProperties !== "object" ||
      Array.isArray(value.additionalProperties))
  )
    errors.value.push(
      `${path}.additionalProperties：必须是布尔值或 Schema 对象`,
    );
}
function validateStepInput(step: WorkflowStep, value: any, path: string) {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    errors.value.push(`${path}：必须是对象`);
    return;
  }
  const required: Partial<Record<WorkflowStep["type"], string[]>> = {
    http: ["url"],
    condition: ["op"],
    transform: ["op"],
    template: ["template"],
    call_skill: ["skillId"],
    schedule: ["timezone", "idempotencyKey"],
    notification: ["content"],
    memory_candidate: ["source"],
    context_contribution: ["content", "tokenLimit"],
  };
  for (const key of required[step.type] || [])
    if (value[key] === undefined || value[key] === "")
      errors.value.push(`${path}.${key}：必填`);
  if (step.type === "transform" && !transformOperations.has(String(value.op)))
    errors.value.push(`${path}.op：不在 transform 白名单中`);
}
function validateAll() {
  errors.value = [];
  if (!/^[a-z0-9]+(?:[.-][a-z0-9]+)*$/.test(editable.metadata.id))
    errors.value.push("metadata.id：必须使用小写字母、数字、点或短横线");
  if (
    !/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$/.test(
      editable.metadata.version,
    )
  )
    errors.value.push("metadata.version：必须是 SemVer");
  const inputSchema = parseJSON(inputSchemaText.value, "inputSchema");
  const outputSchema = parseJSON(outputSchemaText.value, "outputSchema");
  const configSchema = parseJSON(configSchemaText.value, "configSchema");
  const defaultConfig = parseJSON(defaultConfigText.value, "defaultConfig");
  if (inputSchema !== undefined) validateSchema(inputSchema, "inputSchema");
  if (outputSchema !== undefined) validateSchema(outputSchema, "outputSchema");
  if (configSchema !== undefined) validateSchema(configSchema, "configSchema");
  const seen = new Set<string>();
  editable.workflow.steps.forEach((step, index) => {
    const path = `workflow.steps[${index}]`;
    if (
      !/^[a-z_][a-z0-9_-]*$/.test(step.id) ||
      ["input", "config", "secrets", "steps", "runtime"].includes(step.id)
    )
      errors.value.push(`${path}.id：非法或使用了保留名称`);
    if (seen.has(step.id)) errors.value.push(`${path}.id：步骤 ID 重复`);
    seen.add(step.id);
    if (!allowedSteps.includes(step.type))
      errors.value.push(`${path}.type：未知步骤`);
    const input = parseJSON(stepInputTexts.value[index] || "", `${path}.input`);
    if (input !== undefined) {
      validateStepInput(step, input, `${path}.input`);
      step.input = input;
    }
    step.when = parseJSON(
      stepWhenTexts.value[index] || "",
      `${path}.when`,
      false,
    );
    if (step.onError.mode === "use_default")
      step.onError.default = parseJSON(
        stepDefaultTexts.value[index] || "",
        `${path}.onError.default`,
      );
    else delete step.onError.default;
  });
  const output = parseJSON(workflowOutputText.value, "workflow.output");
  editable.testCases.forEach((testCase, index) => {
    const editor = testEditors.value[index];
    const path = `testCases[${index}]`;
    testCase.input = parseJSON(editor.input, `${path}.input`);
    testCase.config = parseJSON(editor.config, `${path}.config`);
    testCase.expectedOutput = parseJSON(
      editor.expectedOutput,
      `${path}.expectedOutput`,
      false,
    );
    testCase.assertions =
      parseJSON(editor.assertions, `${path}.assertions`) || [];
    testCase.httpMocks = parseJSON(editor.httpMocks, `${path}.httpMocks`) || [];
    testCase.skillMocks =
      parseJSON(editor.skillMocks, `${path}.skillMocks`) || [];
    if (!Array.isArray(testCase.assertions))
      errors.value.push(`${path}.assertions：必须是数组`);
    if (!Array.isArray(testCase.httpMocks))
      errors.value.push(`${path}.httpMocks：必须是数组`);
    if (!Array.isArray(testCase.skillMocks))
      errors.value.push(`${path}.skillMocks：必须是数组`);
  });
  if (!errors.value.length) {
    editable.inputSchema = inputSchema;
    editable.outputSchema = outputSchema;
    editable.configSchema = configSchema;
    editable.defaultConfig = defaultConfig;
    editable.workflow.output = output;
  }
  return errors.value.length === 0;
}
function resetStepInput(index: number) {
  stepInputTexts.value[index] = pretty(
    stepExamples[editable.workflow.steps[index].type],
  );
  stepWhenTexts.value[index] = "";
  stepDefaultTexts.value[index] = "";
}
function addStep() {
  const index = editable.workflow.steps.length;
  editable.workflow.steps.push({
    id: `step_${index + 1}`,
    type: "transform",
    input: clone(stepExamples.transform),
    onError: { mode: "fail" },
  });
  stepInputTexts.value.push(pretty(stepExamples.transform));
  stepWhenTexts.value.push("");
  stepDefaultTexts.value.push("");
}
function removeStep(index: number) {
  editable.workflow.steps.splice(index, 1);
  stepInputTexts.value.splice(index, 1);
  stepWhenTexts.value.splice(index, 1);
  stepDefaultTexts.value.splice(index, 1);
}
function addTestCase() {
  const index = editable.testCases.length + 1;
  const testCase: WorkshopTestCase = {
    id: `case_${index}`,
    name: `测试用例 ${index}`,
    mode: "mocked",
    input: {},
    config: {},
    secretRefs: [],
    httpMocks: [],
    skillMocks: [],
    assertions: [],
  };
  editable.testCases.push(testCase);
  testEditors.value.push(testEditor(testCase));
}
function removeTestCase(index: number) {
  editable.testCases.splice(index, 1);
  testEditors.value.splice(index, 1);
}
function submit() {
  if (validateAll()) emit("save", clone(editable));
}
</script>

<style scoped>
.draft-editor {
  display: grid;
  gap: 16px;
}
.editor-tabs {
  min-height: 520px;
}
.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 16px;
}
.form-grid .wide {
  grid-column: 1 / -1;
}
.schema-grid,
.test-json-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}
.schema-hints,
.reference-hints {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.parameter-form {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}
.parameter-form .el-form-item {
  margin: 0;
}
.reference-hints {
  align-items: center;
  color: var(--console-text-muted);
  font-size: 12px;
}
.reference-hints code {
  padding: 3px 6px;
  border: 1px solid var(--console-border);
  border-radius: 5px;
  color: var(--console-text);
}
.editor-field {
  display: grid;
  gap: 6px;
  color: var(--console-text);
  font-weight: 600;
}
.editor-field small,
.parameter-hint,
.capability-preview small {
  color: var(--console-text-muted);
  font-weight: 400;
  line-height: 1.5;
}
.json-input :deep(textarea) {
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  line-height: 1.55;
}
.step-card,
.test-card {
  display: grid;
  gap: 12px;
  margin: 0 0 16px;
  padding: 16px;
  border: 1px solid var(--console-border);
  border-radius: 10px;
}
.step-card legend,
.test-card legend {
  padding: 0 8px;
  color: var(--console-text);
  font-weight: 700;
}
.step-toolbar,
.test-toolbar {
  display: grid;
  grid-template-columns:
    minmax(130px, 1fr) minmax(160px, 1fr) minmax(150px, 1fr)
    auto;
  align-items: end;
  gap: 12px;
}
.step-toolbar .el-form-item,
.test-toolbar .el-form-item {
  margin: 0;
}
.capability-preview {
  display: grid;
  gap: 8px;
  margin-bottom: 16px;
  padding: 12px;
  border: 1px solid var(--console-border);
  border-radius: 8px;
}
.capability-preview > div {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
.output-editor,
.mock-panels {
  margin-top: 16px;
}
.error-summary {
  padding: 12px 16px;
  border: 1px solid var(--el-color-danger);
  border-radius: 8px;
  color: var(--el-color-danger);
}
.error-summary ul {
  margin: 8px 0 0;
  padding-left: 22px;
}
.editor-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}
@media (max-width: 900px) {
  .parameter-form {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 760px) {
  .form-grid,
  .schema-grid,
  .test-json-grid,
  .step-toolbar,
  .test-toolbar,
  .parameter-form {
    grid-template-columns: minmax(0, 1fr);
  }
  .form-grid .wide {
    grid-column: auto;
  }
  .editor-tabs {
    min-height: 0;
  }
}
</style>
