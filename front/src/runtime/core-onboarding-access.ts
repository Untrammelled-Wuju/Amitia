import { apiClient } from "../composables/useApi";
import { getApiBaseURL, getDeploymentConfig, isCurrentDevicePaired } from "./runtime-adapter";

export async function canSkipCoreOnboarding(): Promise<boolean> {
  try {
    if ((await getDeploymentConfig()).mode !== "cloud") return false;
    const base = await getApiBaseURL();
    if (!await isCurrentDevicePaired(base)) return false;
    const response = await apiClient.get("/api/device-mesh/v1/coordination/me");
    const status = response.data?.data || response.data;
    return status?.canAdminister === false && typeof status?.coreId === "string" && Boolean(status.coreId) && typeof status?.policy?.coordinated === "boolean";
  } catch { return false; }
}
