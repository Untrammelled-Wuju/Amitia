import { computed, ref, watch } from "vue";

export const SIDEBAR_CONVERSATION_PAGE_SIZE = 5;

export function useConversationDisclosure<T>(source: () => readonly T[]) {
  const visibleCount = ref(SIDEBAR_CONVERSATION_PAGE_SIZE);
  const total = computed(() => source().length);
  const visible = computed(() => source().slice(0, visibleCount.value));
  const hasMore = computed(() => visibleCount.value < total.value);
  const canToggle = computed(() => total.value > SIDEBAR_CONVERSATION_PAGE_SIZE);

  watch(total, (nextTotal) => {
    const maximum = Math.max(SIDEBAR_CONVERSATION_PAGE_SIZE, nextTotal);
    if (visibleCount.value > maximum) {
      visibleCount.value = maximum;
    }
  });

  function revealNext() {
    visibleCount.value = Math.min(
      visibleCount.value + SIDEBAR_CONVERSATION_PAGE_SIZE,
      total.value,
    );
  }

  function collapse() {
    visibleCount.value = SIDEBAR_CONVERSATION_PAGE_SIZE;
  }

  function toggle() {
    if (hasMore.value) {
      revealNext();
      return;
    }
    collapse();
  }

  return {
    visible,
    hasMore,
    canToggle,
    toggle,
  };
}
