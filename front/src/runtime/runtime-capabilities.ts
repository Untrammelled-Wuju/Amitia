// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { readonly, ref } from "vue";
import { getRuntimeConnection } from "./runtime-adapter";

export type RuntimeCapabilityName =
  | "gameMode"
  | "devicePluginRuntime"
  | "deviceExecutionPlane"
  | "localUIEndpoints";

export interface RuntimeCapabilityFlags {
  gameMode: boolean;
  devicePluginRuntime: boolean;
  deviceExecutionPlane: boolean;
  localUIEndpoints: boolean;
}

export interface RuntimeCapabilitiesSnapshot {
  runtimeProfile: string;
  capabilities: RuntimeCapabilityFlags;
  loaded: boolean;
}

const failClosedSnapshot = (): RuntimeCapabilitiesSnapshot => ({
  runtimeProfile: "unknown",
  capabilities: {
    gameMode: false,
    devicePluginRuntime: false,
    deviceExecutionPlane: false,
    localUIEndpoints: false,
  },
  loaded: false,
});

const runtimeCapabilitiesState = ref<RuntimeCapabilitiesSnapshot>(failClosedSnapshot());
let capabilityLoad: Promise<RuntimeCapabilitiesSnapshot> | null = null;
let capabilityGeneration = 0;

export const runtimeCapabilities = readonly(runtimeCapabilitiesState);

function asBoolean(value: unknown): boolean {
  return value === true;
}

/** Parse the public backend capability contract. Missing fields fail closed. */
export function parseRuntimeCapabilitiesPayload(payload: unknown): RuntimeCapabilitiesSnapshot {
  const root = payload && typeof payload === "object" ? (payload as Record<string, unknown>) : {};
  const data = root.data && typeof root.data === "object" ? (root.data as Record<string, unknown>) : root;
  const rawCapabilities = data.capabilities && typeof data.capabilities === "object"
    ? (data.capabilities as Record<string, unknown>)
    : {};

  const runtimeProfile = typeof data.runtimeProfile === "string" && data.runtimeProfile.trim()
    ? data.runtimeProfile.trim().toLowerCase()
    : "unknown";
  const profileKnown = ["local", "cloud-core", "device-agent"].includes(runtimeProfile);

  return {
    runtimeProfile,
    capabilities: {
      gameMode: profileKnown && runtimeProfile === "local" && asBoolean(rawCapabilities.gameMode),
      devicePluginRuntime: profileKnown && asBoolean(rawCapabilities.devicePluginRuntime),
      deviceExecutionPlane: profileKnown && asBoolean(rawCapabilities.deviceExecutionPlane),
      localUIEndpoints: profileKnown && asBoolean(rawCapabilities.localUIEndpoints),
    },
    loaded: true,
  };
}

/**
 * Load runtime capabilities before the router performs its initial navigation.
 * Any network/protocol failure intentionally leaves all privileged local/device
 * surfaces disabled. A later forced refresh can recover after connectivity is
 * restored or the desktop deployment target changes.
 */
export async function initializeRuntimeCapabilities(force = false): Promise<RuntimeCapabilitiesSnapshot> {
  if (!force && runtimeCapabilitiesState.value.loaded) {
    return runtimeCapabilitiesState.value;
  }
  if (!force && capabilityLoad) {
    return capabilityLoad;
  }

  // A forced refresh means the runtime target may have changed. Revoke the
  // old capability snapshot immediately, before the new network request, so a
  // stale local snapshot cannot keep device-only UI enabled while switching to
  // a cloud core. The generation also prevents an older in-flight request from
  // overwriting the result for the newer target.
  if (force) {
    runtimeCapabilitiesState.value = failClosedSnapshot();
  }
  const generation = ++capabilityGeneration;

  const load = (async () => {
    try {
      const connection = await getRuntimeConnection();
      const baseURL = connection.apiBaseURL.replace(/\/+$/, "");
      const response = await fetch(`${baseURL}/api/public/runtime/capabilities`, {
        method: "GET",
        headers: {
          Accept: "application/json",
          "X-Amitia-Client-Type": "web",
        },
        credentials: "same-origin",
      });
      if (!response.ok) {
        throw new Error(`runtime capability request failed with HTTP ${response.status}`);
      }
      const parsed = parseRuntimeCapabilitiesPayload(await response.json());
      if (generation === capabilityGeneration) {
        runtimeCapabilitiesState.value = parsed;
        return parsed;
      }
      return runtimeCapabilitiesState.value;
    } catch (error) {
      if (generation === capabilityGeneration) {
        const closed = { ...failClosedSnapshot(), loaded: true };
        runtimeCapabilitiesState.value = closed;
        console.warn("[RuntimeCapabilities] capability discovery failed; device-only UI disabled", error);
        return closed;
      }
      return runtimeCapabilitiesState.value;
    } finally {
      if (generation === capabilityGeneration) {
        capabilityLoad = null;
      }
    }
  })();

  capabilityLoad = load;
  return load;
}

export function isRuntimeCapabilityAvailable(capability: RuntimeCapabilityName): boolean {
  return runtimeCapabilitiesState.value.loaded && runtimeCapabilitiesState.value.capabilities[capability] === true;
}

/** Runtime-owned route namespaces are denied unless the backend advertises them. */
export function isRuntimeRouteAvailable(path: string): boolean {
  const normalized = String(path || "").split("?", 1)[0].split("#", 1)[0];
  if (normalized === "/game-center" || normalized.startsWith("/game-center/")) {
    return isRuntimeCapabilityAvailable("gameMode");
  }
  return true;
}

export function isDesktopShell(): boolean {
  return typeof window !== "undefined" && !!window.amitiaDesktop;
}

export function shouldUseHashRouting(): boolean {
  return (
    isDesktopShell() ||
    (typeof window !== "undefined" && window.location.protocol === "file:")
  );
}

export function shouldRegisterServiceWorker(): boolean {
  return (
    typeof window !== "undefined" &&
    window.location.protocol !== "file:" &&
    !isDesktopShell()
  );
}
