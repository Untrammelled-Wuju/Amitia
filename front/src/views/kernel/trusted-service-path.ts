export type TrustedServiceOperation = "start" | "stop" | "status" | "health" | "invoke" | "quarantine/release";

export function trustedServicePath(serviceId: string, operation?: TrustedServiceOperation): string {
  if (!serviceId.trim()) throw new Error("服务 ID 不能为空");
  const suffix = operation ? `/${operation}` : "";
  return `/api/extensions/services${suffix}?service_id=${encodeURIComponent(serviceId)}`;
}
