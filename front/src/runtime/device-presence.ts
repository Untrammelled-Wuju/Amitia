export type DevicePresenceView = {
  trustState?: string;
  presence?: string;
};

export type RuntimePresenceView = {
  runtimeSessionId?: string;
};

export function isDeviceBound(device: DevicePresenceView): boolean {
  return device.trustState !== "revoked";
}

export function isDeviceOnline(device: DevicePresenceView): boolean {
  if (!isDeviceBound(device)) return false;
  return device.presence === "ready" || device.presence === "online";
}

export function canProbeRuntime(runtime: RuntimePresenceView): boolean {
  return Boolean(runtime.runtimeSessionId);
}
