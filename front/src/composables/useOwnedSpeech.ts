import { onMounted, onUnmounted } from "vue";
import { apiClient } from "./useApi";
import { getDeploymentConfig } from "../runtime/runtime-adapter";
import type { OwnedExecutionScope } from "../runtime/device-owned-chat";
import { ownedAuthorityFields as authorityFields, sameOwnedAuthority as sameAuthority, validateOwnedSpeechResult } from "../runtime/owned-speech-result";

const unwrap = (response: any) => response?.data?.data ?? response?.data;

export function useOwnedSpeech(onInvalidated: () => void = () => {}) {
  let generation = 0;
  const controllers = new Set<AbortController>();
  let activeScope: OwnedExecutionScope | null = null;
  let roleTimer: ReturnType<typeof setInterval> | undefined;
  let checkingRole = false;
  function invalidate() {
    generation++;
    for (const controller of controllers) controller.abort();
    controllers.clear();
    activeScope = null;
    if (roleTimer) clearInterval(roleTimer);
    roleTimer = undefined;
    onInvalidated();
  }
  onMounted(() => {
    window.addEventListener("amitia:runtime-connection-changed", invalidate);
    window.addEventListener("amitia:execution-scope-changed", invalidate);
  });

  function monitor(scope: OwnedExecutionScope) {
    activeScope = scope;
    if (roleTimer) clearInterval(roleTimer);
    roleTimer = setInterval(async () => {
      if (checkingRole || !activeScope) return;
      const currentScope = activeScope;
      const captured = generation;
      checkingRole = true;
      try {
        const deployment = await getDeploymentConfig();
        const status = unwrap(await apiClient.get("/api/device-mesh/v1/coordination/me"));
        const roles = unwrap(await apiClient.get("/api/device-mesh/v1/business/roles"));
        if (captured !== generation || activeScope !== currentScope) return;
        const role = roles?.roles?.find((item: any) => item.id === currentScope.roleId);
        if (deployment.mode !== "cloud" || status?.coreId !== currentScope.coreId || status?.policy?.providerEpoch !== currentScope.providerEpoch || status?.policy?.modeRevision !== currentScope.modeRevision || status?.policy?.permissionRevision !== currentScope.permissionRevision || status?.policy?.coordinated !== currentScope.coordinated || roles?.roleOwnerId !== currentScope.roleOwnerId || role?.revision !== currentScope.roleRevision) invalidate();
      } catch { if (captured === generation && activeScope === currentScope) invalidate(); }
      finally { checkingRole = false; }
    }, 1000);
  }
  onUnmounted(() => {
    window.removeEventListener("amitia:runtime-connection-changed", invalidate);
    window.removeEventListener("amitia:execution-scope-changed", invalidate);
    invalidate();
  });

  async function synthesizeIfBound(text: string, characterId?: string): Promise<string | null> {
    const captured = generation;
    const deployment = await getDeploymentConfig();
    if (captured !== generation) throw new Error("服务已切换，旧语音请求已中断");
    if (deployment.mode !== "cloud") return null;
    if (!text.trim() || new TextEncoder().encode(text).length > 8192) throw new Error("语音文本为空或超出8192字节上限");
    const controller = new AbortController();
    controllers.add(controller);
    const current = () => { if (captured !== generation || controller.signal.aborted) throw new Error("Core 或数据归属已变化，旧语音已丢弃"); };
    try {
      const options = { signal: controller.signal };
      let role = characterId;
      if (!role) {
        const catalog = unwrap(await apiClient.get("/api/device-mesh/v1/business/roles", options));
        current();
        if (!Array.isArray(catalog?.roles) || catalog.roles.length !== 1) throw new Error("请先选择一个已保存的角色，再试听语音");
        role = catalog.roles[0].id;
      }
      const query = () => apiClient.get("/api/device-mesh/v1/business/data", { ...options, params: { kind: "memory", characterId: role } });
      const snapshot = unwrap(await query());
      current();
      const expected = snapshot?.executionScope as OwnedExecutionScope;
      if (!expected?.coreId || expected.roleId !== role || !expected.resourceOwnerId || authorityFields.some((key) => expected[key] === undefined)) throw new Error("角色语音归属无法确认，请重新加载角色");
      const requestId = crypto.randomUUID();
      const result = unwrap(await apiClient.post("/api/device-mesh/v1/business/speech", { requestId, characterId: role, expectedExecutionScope: expected, text }, options));
      current();
      await validateOwnedSpeechResult(result, requestId, expected);
      const latest = unwrap(await query());
      current();
      if (!sameAuthority(expected, latest?.executionScope)) throw new Error("角色或服务权限已变化，旧语音已丢弃");
      monitor(expected);
      return `data:audio/mpeg;base64,${result.audio.data}`;
    } finally { controllers.delete(controller); }
  }
  return { synthesizeIfBound, invalidate };
}
