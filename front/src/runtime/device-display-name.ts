export type DeviceNameSource = {
  deviceId: string;
  label?: string;
  reportedName?: string;
};

export const UNNAMED_DEVICE = "未命名设备";

export function resolveDeviceDisplayNames(devices: DeviceNameSource[]): Map<string, string> {
  const entries = devices.map((device) => ({
    deviceId: device.deviceId,
    name: (device.label || "").trim() || (device.reportedName || "").trim() || UNNAMED_DEVICE,
  }));
  const counts = new Map<string, number>();
  for (const entry of entries) {
    counts.set(entry.name, (counts.get(entry.name) || 0) + 1);
  }
  const seen = new Map<string, number>();
  const result = new Map<string, string>();
  for (const entry of entries) {
    if ((counts.get(entry.name) || 0) <= 1) {
      result.set(entry.deviceId, entry.name);
      continue;
    }
    const index = (seen.get(entry.name) || 0) + 1;
    seen.set(entry.name, index);
    result.set(entry.deviceId, index === 1 ? entry.name : `${entry.name} (${index})`);
  }
  return result;
}

export function deviceDisplayName(device: DeviceNameSource, devices: DeviceNameSource[]): string {
  return resolveDeviceDisplayNames(devices).get(device.deviceId) || UNNAMED_DEVICE;
}
