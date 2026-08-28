// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import {
  getBackendAuthHeaders,
  getDeploymentConfig,
  isDeviceLocalApiPath,
} from "./runtime-adapter";
import { ensureValidToken } from "@/stores/refresh-coordinator";
import { getDeviceTimezone } from "@/utils/requestEnvelope";
import { resolveUIHostDeviceId } from "@/ui-runtime/deviceIdentity";

/**
 * Builds authorization headers for fetch()-based requests.
 *
 * Desktop-pet package/runtime APIs are always device-local, including when the
 * application is connected to a cloud Business Core. Those requests therefore
 * use the local Desktop Session and must never receive the cloud bearer token.
 * Business requests in cloud/web mode use the renewable cloud access token.
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
  const desktopLocalAuth =
    typeof window !== "undefined" &&
    Boolean(window.amitiaDesktop) &&
    (deployment.mode === "local" || deviceLocal);

  const headers = new Headers(init.headers ?? undefined);

  if (desktopLocalAuth) {
    const desktopHeaders = await getBackendAuthHeaders();
    for (const [key, value] of Object.entries(desktopHeaders)) {
      if (value) headers.set(key, value);
    }
    headers.delete("Authorization");
  } else {
    const token = await ensureValidToken();
    if (token) headers.set("Authorization", `Bearer ${token}`);
  }

  if (typeof window !== "undefined" && window.amitiaDesktop) {
    headers.set("X-Amitia-Client-Type", "desktop");
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

  return {
    ...init,
    headers,
  };
}
