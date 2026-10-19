import { getDeploymentConfig } from "./runtime-adapter";

export type DeviceServiceMode = "local" | "cloud";

export function deviceServiceName(mode: DeviceServiceMode): string {
  return mode === "cloud" ? "云端服务" : "本机 Core";
}

export function deviceProviderChangedNotice(
  mode: DeviceServiceMode,
  previousCoreId: string,
  currentCoreId: string,
  outcome: string,
): string {
  const subject = mode === "cloud" ? "云端服务提供者" : "本机 Core";
  return `${subject}${mode === "cloud" ? "" : " "}已从「${previousCoreId}」切换为「${currentCoreId}」，${outcome}`;
}

export function deviceProviderTransitionNotice(
  mode: DeviceServiceMode,
  previousCoreId: string,
  successorCoreId: string,
): string {
  const subject = mode === "cloud" ? "云端服务提供者" : "本机 Core";
  return `${subject}${mode === "cloud" ? "" : " "}正在从「${previousCoreId}」切换至「${successorCoreId || "新 Core"}」，当前回复已中断，等待新服务批准并连接。`;
}

export function deviceUnavailableNotice(mode: DeviceServiceMode): string {
  return mode === "cloud"
    ? "云端服务暂不可用，当前回复已中断；不会自动切换到本机模型。"
    : "本机 Core 服务暂不可用，当前回复已中断；不会自动切换运行位置。";
}

export async function currentDeviceServiceMode(): Promise<DeviceServiceMode> {
  try {
    return (await getDeploymentConfig()).mode;
  } catch {
    return "local";
  }
}
