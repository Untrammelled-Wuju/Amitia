import { onMounted, onUnmounted, ref } from "vue";
import { getDeploymentConfig } from "../runtime/runtime-adapter";
import { useApi } from "./useApi";

export function coreConfigurationRequestConfig(context: string) {
  if (context === "local") return {};
  let fields: unknown;
  try { fields = JSON.parse(context); } catch { throw new Error("配置归属无法确认，请重新加载配置"); }
  if (!Array.isArray(fields) || fields.length !== 5 || typeof fields[1] !== "string" || !fields[1] || fields[1].length > 512 || fields[1] !== fields[1].trim()) throw new Error("配置归属无法确认，请重新加载配置");
  if (fields.slice(2).some((value) => !Number.isSafeInteger(value) || value < 1)) throw new Error("配置权限版本无法确认，请重新加载配置");
  return { headers: { "X-Amitia-Expected-Core-ID": fields[1], "X-Amitia-Expected-Configuration-Policy": fields.slice(2).join(":") } };
}

export function useCoreConfigurationAccess() {
  const { get } = useApi();
  const canConfigure = ref(false);
  const checking = ref(true);
  const contextKey = ref("");
  const explanation = "AI 服务由 Core 提供，当前设备不能配置模型。请在 Core 控制页面修改，或由 Core 授予已开启统筹模式的设备管理员权限。";
  let generation = 0;
  let refreshNumber = 0;
  let pending: { generation: number; promise: Promise<string> } | undefined;
  let timer: ReturnType<typeof setInterval> | undefined;

  function refresh(): Promise<string> {
    if (pending?.generation === generation) return pending.promise;
    const promise = queryAccess();
    pending = { generation, promise };
    promise.finally(() => { if (pending?.promise === promise) pending = undefined; });
    return promise;
  }

  async function queryAccess() {
    const currentGeneration = generation;
    const request = ++refreshNumber;
    try {
      const deployment = await getDeploymentConfig();
      let key = "local";
      if (deployment.mode === "cloud") {
        const status = await get<any>("/api/device-mesh/v1/coordination/me");
        const policy = status?.policy;
        if (status?.canAdminister !== true || policy?.coordinated !== true || typeof status.coreId !== "string" || !status.coreId) throw new Error(explanation);
        const revisions = [policy.providerEpoch, policy.modeRevision, policy.permissionRevision];
        if (revisions.some((value) => !Number.isSafeInteger(value) || value < 1)) throw new Error(explanation);
        key = JSON.stringify([deployment.serverURL, status.coreId, ...revisions]);
      }
      if (generation !== currentGeneration || request !== refreshNumber) return "";
      contextKey.value = key;
      canConfigure.value = true;
      return key;
    } catch {
      if (generation === currentGeneration && request === refreshNumber) {
        canConfigure.value = false;
        contextKey.value = "";
      }
      return "";
    } finally {
      if (request === refreshNumber) checking.value = false;
    }
  }

  function invalidate() {
    generation++;
    canConfigure.value = false;
    contextKey.value = "";
    checking.value = true;
    void refresh();
  }

  async function requireAccess(expected?: string) {
    const current = await refresh();
    if (!current) throw new Error(explanation);
    if (expected !== undefined && expected !== current) throw new Error("Core 或管理员权限已变化，请重新加载配置后操作");
    return current;
  }

  onMounted(() => {
    void refresh();
    window.addEventListener("amitia:runtime-connection-changed", invalidate);
    window.addEventListener("amitia:execution-scope-changed", invalidate);
    timer = setInterval(() => { void refresh(); }, 2000);
  });
  onUnmounted(() => {
    generation++;
    window.removeEventListener("amitia:runtime-connection-changed", invalidate);
    window.removeEventListener("amitia:execution-scope-changed", invalidate);
    if (timer) clearInterval(timer);
  });
  return { canConfigure, checking, contextKey, explanation, refresh, requireAccess };
}
