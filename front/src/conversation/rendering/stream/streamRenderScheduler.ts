import { onScopeDispose, ref, watch, type Ref } from "vue";

export function useStreamRenderScheduler(
  source: Ref<string>,
  streaming: Ref<boolean>,
): Ref<string> {
  const rendered = ref(source.value);
  let frame: number | null = null;

  const flush = () => {
    frame = null;
    rendered.value = source.value;
  };

  const schedule = () => {
    if (!streaming.value) {
      if (frame !== null) cancelAnimationFrame(frame);
      frame = null;
      flush();
      return;
    }
    if (frame !== null) return;
    frame = requestAnimationFrame(flush);
  };

  watch([source, streaming], schedule, { flush: "post" });
  onScopeDispose(() => {
    if (frame !== null) cancelAnimationFrame(frame);
    frame = null;
  });

  return rendered;
}

