import { computed, type Ref } from "vue";
import { useExtensionUIStore, type UIContributionSummary } from "@/stores/extensionUI";

export function useCharacterExtensionTabs(context: Ref<Record<string, unknown>>) {
  const store = useExtensionUIStore();
  const tabs = computed<UIContributionSummary[]>(() =>
    store.getVisibleContributions("character.detail.tab", context.value),
  );
  return { tabs };
}
