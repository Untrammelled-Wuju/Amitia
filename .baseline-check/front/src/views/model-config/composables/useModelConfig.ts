// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, reactive, onMounted, inject } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import type { FormInstance, FormRules } from "element-plus";
import { useApi } from "../../../composables/useApi";

interface TestResultData {
  success: boolean;
  latencyMs: number;
  message: string;
  reply: string;
}

interface ModelConfigOptions {
  apiBase?: string;
  activatePath?: string;
  withScenario?: boolean;
  withDetect?: boolean;
  defaultApiType?: string;
  defaultModel?: string;
  defaultBaseUrl?: string;
  modelLabel?: string;
  showAdvanced?: boolean;
  showDetect?: boolean;
  modelPlaceholder?: string;
  extraFormFields?: Record<string, any>;
  transformPayload?: (payload: any) => any;
  transformConfig?: (config: any) => any;
  testResultMapper?: (result: any) => TestResultData;
  defaultIsActive?: number;
}

function defaultTestMapper(result: any): TestResultData {
  return {
    success:
      result?.test?.success ?? result?.success ?? result?.status === "ok",
    latencyMs:
      result?.test?.latencyMs ?? result?.latencyMs ?? result?.latency ?? 0,
    message: result?.test?.message ?? result?.message ?? "",
    reply: result?.test?.reply ?? result?.reply ?? "",
  };
}

export function useModelConfig(options?: ModelConfigOptions) {
  const { get, post, put, del } = useApi();
  const refreshHealth = inject<() => void>("refreshHealth", () => {});

  const apiBase = options?.apiBase ?? "/api/model";
  const activatePath = options?.activatePath ?? "/active";
  const withScenario = options?.withScenario ?? true;
  const withDetect = options?.withDetect ?? true;
  const defaultApiType = options?.defaultApiType ?? "openai";
  const defaultModel = options?.defaultModel ?? "";
  const defaultBaseUrl = options?.defaultBaseUrl ?? "";
  const modelLabel = options?.modelLabel ?? "模型";
  const showAdvanced = options?.showAdvanced ?? true;
  const showDetect = options?.showDetect ?? true;
  const modelPlaceholder =
    options?.modelPlaceholder ?? "gpt-4o-mini / qwen2.5:7b / deepseek-chat";
  const extraFormFields = options?.extraFormFields;
  const transformPayload = options?.transformPayload;
  const transformConfig = options?.transformConfig;
  const testResultMapper = options?.testResultMapper ?? defaultTestMapper;
  const defaultIsActive = options?.defaultIsActive ?? 0;

  const configs = ref<any[]>([]);
  const providers = ref<any[]>([]);
  const currentProviderSchema = ref<any>(null);

  const dialogVisible = ref(false);
  const detectingModels = ref(false);
  const detectedModels = ref<{ id: string; owned_by?: string }[]>([]);
  const detectError = ref("");
  const editingId = ref<number | null>(null);
  const saving = ref(false);
  const showApiKey = ref(false);
  const showKeyId = ref<number | null>(null);
  const originalApiKey = ref("");
  const testingId = ref<number | null>(null);
  const testResultVisible = ref(false);
  const testResult = ref<any>(null);
  const dialogFormRef = ref<FormInstance | null>(null);
  const scenarioRoutes = ref<any[]>([]);
  const routeAssignments = ref<Record<string, number | null>>({});

  const form = reactive<{
    name: string;
    apiType: string;
    baseUrl: string;
    apiKey: string;
    modelName: string;
    temperature: number;
    maxTokens: number;
    timeoutSeconds: number;
    retryCount: number;
    [key: string]: any;
  }>({
    name: "",
    apiType: defaultApiType,
    baseUrl: defaultBaseUrl,
    apiKey: "",
    modelName: defaultModel,
    temperature: 0.7,
    maxTokens: 4096,
    timeoutSeconds: 60,
    retryCount: 1,
  });

  if (extraFormFields) {
    for (const [key, val] of Object.entries(extraFormFields)) {
      (form as any)[key] = val;
    }
  }

  const rules: FormRules = {
    name: [{ required: true, message: "请输入名称", trigger: "blur" }],
    apiType: [{ required: true, message: "请选择类型", trigger: "change" }],
    baseUrl: [{ required: true, message: "请输入 Base URL", trigger: "blur" }],
    modelName: [{ required: true, message: `请输入${modelLabel}名称`, trigger: "blur" }],
  };

  function providerName(apiType: string): string {
    const p = providers.value.find((pr: any) => pr.id === apiType);
    return p?.name || apiType;
  }

  function capLabel(key: string | number): string {
    const labels: Record<string, string> = {
      chat: "聊天",
      stream: "流式",
      vision: "视觉",
      tools: "工具",
      embeddings: "嵌入",
      local: "本地",
      remote: "远程",
    };
    const name = String(key);
    return labels[name] || name.replace(/_/g, " ");
  }

  function maskKey(key: string): string {
    if (!key) return "未设置";
    if (key.length <= 8) return "****";
    return key.slice(0, 4) + "****" + key.slice(-4);
  }

  function toggleKey(id: number) {
    showKeyId.value = showKeyId.value === id ? null : id;
  }

  function fmtDate(dateStr: string): string {
    if (!dateStr) return "";
    try {
      return new Date(dateStr).toLocaleString("zh-CN");
    } catch {
      return dateStr;
    }
  }

  async function fetchConfigs() {
    configs.value = ((await get<any[]>(`${apiBase}/configs`)) || []).map(
      (c) => {
        if (transformConfig) c = transformConfig(c);
        return { ...c, isActive: !!c.isActive };
      },
    );
  }

  async function loadProviders() {
    try {
      providers.value = (await get<any[]>(`${apiBase}/providers`)) || [];
    } catch {
      providers.value = [];
    }
    onProviderChange(form.apiType);
  }

  function onProviderChange(apiType = form.apiType) {
    currentProviderSchema.value =
      providers.value.find((p: any) => p.id === apiType) || null;
    detectedModels.value = [];
    detectError.value = "";
    const provider = currentProviderSchema.value as any;
    if (provider) {
      if (provider.defaultBaseUrl) {
        form.baseUrl = provider.defaultBaseUrl;
      }
      if (provider.defaultModel) {
        form.modelName = provider.defaultModel;
      }
    }
  }

  async function detectModels() {
    if (!withDetect) return;
    detectError.value = "";
    detectedModels.value = [];
    detectingModels.value = true;
    try {
      const res = await post<any>(`${apiBase}/detect-models`, {
        baseUrl: form.baseUrl,
        apiKey: form.apiKey,
        apiType: form.apiType,
      });
      detectedModels.value = res?.models || [];
      if (detectedModels.value.length === 0) {
        detectError.value = "未检测到可用模型";
      }
    } catch (err: any) {
      detectError.value =
        err?.message || "检测失败，请检查 Base URL 和 API Key";
    } finally {
      detectingModels.value = false;
    }
  }

  async function showDialog(row: any) {
    editingId.value = row?.id || null;
    showApiKey.value = false;
    let fullConfig: any = null;
    if (row) {
      let cfg = row;
      if (transformConfig) cfg = transformConfig(row);
      form.name = cfg.name || "";
      form.apiType = cfg.apiType || defaultApiType;
      form.baseUrl = cfg.baseUrl || "";
      try {
        fullConfig = await get<any>(`${apiBase}/configs/${row.id}`);
        if (transformConfig) fullConfig = transformConfig(fullConfig);
        form.apiKey = fullConfig?.apiKey || "";
        originalApiKey.value = fullConfig?.apiKey || "";
      } catch {
        form.apiKey = "";
        originalApiKey.value = "";
      }
      form.modelName = cfg.modelName || "";
      form.temperature = cfg.temperature ?? 0.7;
      form.maxTokens = cfg.maxTokens ?? 4096;
      form.timeoutSeconds = cfg.timeoutSeconds ?? 60;
      form.retryCount = cfg.retryCount ?? 1;
    } else {
      form.name = "";
      form.apiType = defaultApiType;
      form.baseUrl = defaultBaseUrl;
      form.apiKey = "";
      form.modelName = defaultModel;
      form.temperature = 0.7;
      form.maxTokens = 4096;
      form.timeoutSeconds = 60;
      form.retryCount = 1;
    }
    if (extraFormFields) {
      for (const [key, defaultVal] of Object.entries(extraFormFields)) {
        (form as any)[key] = fullConfig?.[key] ?? row?.[key] ?? defaultVal;
      }
    }
    onProviderChange(form.apiType);
    dialogVisible.value = true;
    setTimeout(() => dialogFormRef.value?.clearValidate(), 0);
  }

  async function saveConfig() {
    const valid = await dialogFormRef.value?.validate().catch(() => false);
    if (!valid) return;

    saving.value = true;
    try {
      let payload: any = { ...form };
      if (transformPayload) {
        payload = transformPayload(payload);
      }
      if (!payload.apiKey || payload.apiKey === originalApiKey.value) {
        delete payload.apiKey;
      }
      if (editingId.value) {
        await put(`${apiBase}/configs/${editingId.value}`, payload);
      } else {
        if (defaultIsActive) {
          payload.isActive = defaultIsActive;
        }
        await post(`${apiBase}/configs`, { ...payload });
      }
      dialogVisible.value = false;
      ElMessage.success(editingId.value ? "保存成功" : "新建成功");
      await fetchConfigs();
    } catch {
    } finally {
      saving.value = false;
    }
  }

  async function testConfig(id: number) {
    testingId.value = id;
    try {
      const result = await post<any>(`${apiBase}/configs/${id}/test`, {
        configId: id,
      });
      testResult.value = testResultMapper(result);
      testResultVisible.value = true;
      await fetchConfigs();
    } catch {
      testResult.value = {
        success: false,
        latencyMs: 0,
        message: "请求失败",
        reply: "",
      };
      testResultVisible.value = true;
    } finally {
      testingId.value = null;
    }
  }

  async function setActive(id: number) {
    try {
      await post(`${apiBase}/configs/${id}${activatePath}`);
      ElMessage.success("已设为默认");
      await fetchConfigs();
      refreshHealth();
    } catch {}
  }

  async function delConfig(id: number) {
    const cfg = configs.value.find((c) => c.id === id);
    if (cfg?.isActive && configs.value.length <= 1) {
      ElMessage.warning("不能删除唯一的激活配置");
      return;
    }
    await ElMessageBox.confirm(
      "确定删除此配置？如果是当前激活配置，将自动切换到其他配置。",
      "确认删除",
      { type: "warning", confirmButtonText: "删除" },
    );
    try {
      await del(`${apiBase}/configs/${id}`);
      ElMessage.success("已删除");
      await fetchConfigs();
    } catch {}
  }

  async function fetchRoutes() {
    if (!withScenario) return;
    try {
      const data = await get<any[]>(`${apiBase}/routes`);
      scenarioRoutes.value = Array.isArray(data)
        ? data
        : (data as any)?.data || [];
      for (const r of scenarioRoutes.value) {
        routeAssignments.value[r.scenario] = r.modelConfigId;
      }
    } catch {}
  }

  async function assignRoute(scenario: string, modelConfigId: number | null) {
    if (!withScenario) return;
    try {
      await put(`${apiBase}/routes`, { routes: { [scenario]: modelConfigId } });
      ElMessage.success("用途分配已更新");
      await fetchRoutes();
    } catch {
      await fetchRoutes();
    }
  }

  function scenarioLabel(scenario: string): string {
    const labels: Record<string, string> = {
      chat: "聊天对话",
      summary: "会话摘要",
      memory_extract: "记忆提取",
      safety_rewrite: "安全改写",
      import_parse: "导入解析",
      reply_timing_check: "完整性判断",
    };
    return labels[scenario] || scenario;
  }

  function scenarioDesc(scenario: string): string {
    const descs: Record<string, string> = {
      chat: "日常聊天和对话回复",
      summary: "生成对话历史摘要",
      memory_extract: "从对话中提取用户记忆",
      safety_rewrite: "安全边界内容改写",
      import_parse: "解析导入的聊天记录文本",
      reply_timing_check: "判断回复用户是否发送完成完整信息",
    };
    return descs[scenario] || "";
  }

  onMounted(async () => {
    await loadProviders();
    fetchConfigs();
    if (withScenario) fetchRoutes();
  });

  return {
    configs,
    providers,
    currentProviderSchema,
    dialogVisible,
    detectingModels,
    detectedModels,
    detectError,
    editingId,
    saving,
    showApiKey,
    showKeyId,
    originalApiKey,
    testingId,
    testResultVisible,
    testResult,
    dialogFormRef,
    scenarioRoutes,
    routeAssignments,
    form,
    rules,
    showAdvanced,
    showDetect,
    modelPlaceholder,
    providerName,
    capLabel,
    maskKey,
    toggleKey,
    fmtDate,
    fetchConfigs,
    loadProviders,
    onProviderChange,
    detectModels,
    showDialog,
    saveConfig,
    testConfig,
    setActive,
    delConfig,
    fetchRoutes,
    assignRoute,
    scenarioLabel,
    scenarioDesc,
  };
}
