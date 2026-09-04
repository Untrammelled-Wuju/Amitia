import { computed } from "vue";
import logoDarkUrl from "@/assets/amitia-logo-dark.png";
import logoLightUrl from "@/assets/amitia-logo-light.png";
import { useTheme } from "./useTheme";

export function useBrandLogo() {
  const { resolvedMode } = useTheme();
  const logoUrl = computed(() =>
    resolvedMode.value === "dark" ? logoLightUrl : logoDarkUrl,
  );

  return {
    logoUrl,
    logoDarkUrl,
    logoLightUrl,
  };
}
