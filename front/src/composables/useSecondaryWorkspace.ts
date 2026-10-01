import { computed, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { isNavigationAllowed } from "../navigation/nav-whitelist";
import { resolveSecondaryPage } from "../navigation/secondary-workspace";

export function useSecondaryWorkspace() {
  const route = useRoute();
  const secondaryPage = computed(() => resolveSecondaryPage(route.path));
  const returnTo = ref("/chat");

  watch(() => route.fullPath, (current, previous) => {
    if (resolveSecondaryPage(current) && !resolveSecondaryPage(previous) && isNavigationAllowed(previous)) {
      returnTo.value = previous;
    }
  });

  return { secondaryPage, returnTo };
}
