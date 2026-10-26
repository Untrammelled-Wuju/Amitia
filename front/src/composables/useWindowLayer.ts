import { computed, onUnmounted, shallowRef, toValue, watch, type MaybeRefOrGetter } from "vue"

const blockers = shallowRef(new Set<string>())
const fallbackBlocker = shallowRef(false)

export const windowLayerBlocked = computed(() => blockers.value.size > 0 || fallbackBlocker.value)

function setBlocker(id: string, active: boolean) {
  if (active === blockers.value.has(id)) return
  const next = new Set(blockers.value)
  if (active) next.add(id)
  else next.delete(id)
  blockers.value = next
}

export function setWindowLayerFallback(active: boolean) {
  fallbackBlocker.value = active
}

export function useWindowLayerBlocker(id: string, active: MaybeRefOrGetter<boolean>) {
  watch(
    () => Boolean(toValue(active)),
    (value) => setBlocker(id, value),
    { immediate: true, flush: "sync" },
  )
  onUnmounted(() => setBlocker(id, false))
}

export function useWindowLayerState() {
  return windowLayerBlocked
}
