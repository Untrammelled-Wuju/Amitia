import { ref, watch, type Ref } from "vue";

export function useStreamRenderScheduler(
  source: Ref<string>,
  streaming: Ref<boolean>,
  interval = 40,
): Ref<string> {
  const rendered = ref(source.value);
  let timer: ReturnType<typeof setTimeout> | null = null;
  let lastFlush = 0;

  const flush = () => {
    timer = null;
    lastFlush = Date.now();
    rendered.value = source.value;
  };

  const schedule = () => {
    if (!streaming.value) {
      if (timer) clearTimeout(timer);
      timer = null;
      flush();
      return;
    }
    if (timer) return;
    const wait = Math.max(0, interval - (Date.now() - lastFlush));
    timer = setTimeout(flush, wait);
  };

  watch([source, streaming], schedule, { flush: "post" });

  return rendered;
}

