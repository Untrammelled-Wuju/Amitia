import { onMounted, onUnmounted, watch, type Ref } from "vue";
import { useRouter } from "vue-router";
import { useUIHostSSE } from "./useUIHostSSE";
import { useExtensionUIStore } from "../stores/extensionUI";
import { isNavigationAllowed } from "../navigation/nav-whitelist";

export function useAppUIHost(isPublicPage: Readonly<Ref<boolean>>) {
  const router = useRouter();
  const extensionUIStore = useExtensionUIStore();
  const { connect, disconnect } = useUIHostSSE();
  let disposeExtensionListener: (() => void) | undefined;
  let disposeNavigation: (() => void) | undefined;

  watch(isPublicPage, (isPublic) => {
    if (isPublic) disconnect();
    else void connect();
  }, { immediate: true });

  onMounted(() => {
    disposeExtensionListener = extensionUIStore.setupExtensionChangeListener();
    disposeNavigation = window.amitiaDesktop?.onUINavigate((target: string) => {
      if (isNavigationAllowed(target)) void router.push(target).catch(() => {});
    });
  });

  onUnmounted(() => {
    disposeExtensionListener?.();
    disposeNavigation?.();
    extensionUIStore.invalidateSnapshot();
  });
}
