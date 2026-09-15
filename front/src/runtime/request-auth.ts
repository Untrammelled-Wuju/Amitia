// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import {
  getBackendAuthHeaders,
  getDeploymentConfig,
  isDeviceLocalApiPath,
} from "./runtime-adapter";
import { getDeviceTimezone } from "@/utils/requestEnvelope";
import { resolveUIHostDeviceId } from "@/ui-runtime/deviceIdentity";

/**
 * Builds authorization headers for fetch()-based requests.
 *
 * Desktop-pet package/runtime APIs are always device-local in Electron and use
 * the local Desktop Session. Business requests use the paired DeviceCredential:
 * Electron obtains it from Device Agent, while the browser stores its own paired
 * web-device credential for the selected Cloud Core.
 */
export async function createAuthenticatedFetchInit(
  path: string,
  init: RequestInit = {},
): Promise<RequestInit> {
  const deployment = await getDeploymentConfig();
  const deviceLocal =
    typeof window !== "undefined" &&
    Boolean(window.amitiaDesktop) &&
    isDeviceLocalApiPath(path);
  const isPublic = String(path || "").split("?", 1)[0].startsWith("/api/public/");
  const headers = new Headers(init.headers ?? undefined);

  if (typeof window !== "undefined" && !isPublic) {
    const authHeaders = await getBackendAuthHeaders(deviceLocal ? "local" : "business");
    for (const [key, value] of Object.entries(authHeaders)) {
      if (value) headers.set(key, value);
    }
  }

  if (typeof window !== "undefined") {
    headers.set("X-Amitia-Client-Type", window.amitiaDesktop ? "desktop" : "web");
  }

  const deviceId = await resolveUIHostDeviceId();
  if (deviceId) {
    headers.set("X-Amitia-Device-ID", deviceId);
    const normalizedPath = String(path || "").split("?", 1)[0];
    const gameCenterRequest =
      normalizedPath === "/api/game-center" || normalizedPath.startsWith("/api/game-center/");
    if (deployment.mode === "cloud" && gameCenterRequest) {
      headers.set("X-Amitia-Target-Device-ID", deviceId);
    }
  }

  const timezone = getDeviceTimezone();
  if (timezone) headers.set("X-Device-Timezone", timezone);

  const credentials = typeof window !== "undefined" && !window.amitiaDesktop
    ? "include"
    : init.credentials;
  return { ...init, headers, credentials };
}
