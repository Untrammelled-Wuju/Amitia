import { ref } from "vue";
import { getRuntimeConnection } from "@/runtime/runtime-adapter";

export function useAssetUrl() {
  const baseURL = ref("");

  void getRuntimeConnection()
    .then((connection) => {
      baseURL.value = connection.apiBaseURL.replace(/\/$/, "");
    })
    .catch((error) => {
  console.error("[original-error]", error);
baseURL.value = "";
    });

  function assetUrl(path?: string): string {
    if (!path || /^(?:https?:|data:|blob:)/i.test(path) || !baseURL.value)
      return path || "";
    return `${baseURL.value}/${path.replace(/^\//, "")}`;
  }

  return { assetUrl };
}
