import { onUnmounted } from "vue";
import { getRuntimeConnection } from "../runtime/runtime-adapter";
import { useApi } from "./useApi";

export type DeviceManagementState = { coreId: string; canAdminister: boolean; coordinationAvailable?: boolean; policy: { deviceId: string; coordinated: boolean; administrator: boolean; providerEpoch: number; modeRevision: number; permissionRevision: number; selectedRole?: string } };
export type DeviceManagementIntent = { state: DeviceManagementState; apiBaseURL: string; controller: AbortController };
const changed = "Core、统筹模式或设备权限已变化，请刷新后重新操作";

export function deviceManagementFingerprint(state: DeviceManagementState, permissionIncrement = 0) {
  const policy = state?.policy;
  if (!state?.coreId || !policy?.deviceId || typeof state.canAdminister !== "boolean" || typeof policy.coordinated !== "boolean" || typeof policy.administrator !== "boolean" || [policy.providerEpoch, policy.modeRevision, policy.permissionRevision].some(value => !Number.isSafeInteger(value) || value < 1)) throw new Error("设备管理权限无法确认，请刷新");
  return JSON.stringify([state.coreId, policy.deviceId, state.canAdminister, policy.coordinated, policy.administrator, policy.providerEpoch, policy.modeRevision, policy.permissionRevision + permissionIncrement]);
}

export function deviceManagementRequestConfig(intent: DeviceManagementIntent) {
  deviceManagementFingerprint(intent.state);
  if (intent.controller.signal.aborted) throw new Error(changed);
  const policy = intent.state.policy;
  return { signal: intent.controller.signal, headers: { "X-Amitia-Expected-Core-ID": intent.state.coreId, "X-Amitia-Expected-Configuration-Policy": `${policy.providerEpoch}:${policy.modeRevision}:${policy.permissionRevision}` } };
}

export function useDeviceManagementIntent(onInvalidated?: () => void) {
  const api = useApi();
  const intents = new Set<DeviceManagementIntent>();
  function invalidate() {
    for (const intent of intents) intent.controller.abort(changed);
    intents.clear();
    onInvalidated?.();
  }
  async function capture() {
    const connection = await getRuntimeConnection();
    const state = await api.get<DeviceManagementState>("/api/device-mesh/v1/coordination/me");
    deviceManagementFingerprint(state);
    if ((await getRuntimeConnection()).apiBaseURL !== connection.apiBaseURL) throw new Error(changed);
    const intent = { state: structuredClone(state), apiBaseURL: connection.apiBaseURL, controller: new AbortController() };
    intents.add(intent);
    return intent;
  }
  async function validate(intent: DeviceManagementIntent, permissionIncrement = 0) {
    if (intent.controller.signal.aborted || (await getRuntimeConnection()).apiBaseURL !== intent.apiBaseURL) throw new Error(changed);
    const state = await api.get<DeviceManagementState>("/api/device-mesh/v1/coordination/me", undefined, { signal: intent.controller.signal });
    if (intent.controller.signal.aborted || (await getRuntimeConnection()).apiBaseURL !== intent.apiBaseURL || deviceManagementFingerprint(state) !== deviceManagementFingerprint(intent.state, permissionIncrement)) {
      intent.controller.abort(changed);
      throw new Error(changed);
    }
    return state;
  }
  window.addEventListener("amitia:runtime-connection-changed", invalidate);
  window.addEventListener("amitia:execution-scope-changed", invalidate);
  onUnmounted(() => {
    window.removeEventListener("amitia:runtime-connection-changed", invalidate);
    window.removeEventListener("amitia:execution-scope-changed", invalidate);
    invalidate();
  });
  return { capture, validate, invalidate };
}
