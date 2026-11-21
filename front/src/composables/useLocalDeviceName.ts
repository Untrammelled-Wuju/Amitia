// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref } from "vue";
import { apiClient } from "@/composables/useApi";
import { resolveUIHostDeviceId } from "@/ui-runtime/deviceIdentity";
import { deviceDisplayName } from "@/runtime/device-display-name";

const localDeviceName = ref("");
let pending: Promise<void> | null = null;

type DeviceRow = { deviceId: string; label?: string; reportedName?: string };

function unwrap(payload: any): any {
  return payload?.data?.data ?? payload?.data ?? payload;
}

async function resolveLocalDeviceName() {
  const deviceId = String((await resolveUIHostDeviceId()) || "");
  if (!deviceId) return;
  const response = await apiClient.get("/api/device-mesh/v1/devices");
  const body = unwrap(response);
  const devices: DeviceRow[] = Array.isArray(body?.devices) ? body.devices : [];
  if (!devices.length) return;
  localDeviceName.value = deviceDisplayName({ deviceId }, devices);
}

export function useLocalDeviceName() {
  if (!pending) {
    pending = resolveLocalDeviceName().catch((error: unknown) => {
      console.error("[original-error]", error, (error as { stack?: string })?.stack);
      localDeviceName.value = "";
      pending = null;
    });
  }
  return { localDeviceName };
}
