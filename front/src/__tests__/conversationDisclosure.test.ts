import { nextTick, ref } from "vue";
import { describe, expect, it } from "vitest";
import { useConversationDisclosure } from "../composables/useConversationDisclosure";

describe("useConversationDisclosure", () => {
  it("reveals sidebar conversations five at a time", () => {
    const source = ref(Array.from({ length: 12 }, (_, index) => index + 1));
    const disclosure = useConversationDisclosure(() => source.value);

    expect(disclosure.visible.value).toEqual([1, 2, 3, 4, 5]);
    expect(disclosure.hasMore.value).toBe(true);
    expect(disclosure.canToggle.value).toBe(true);

    disclosure.toggle();
    expect(disclosure.visible.value).toEqual([1, 2, 3, 4, 5, 6, 7, 8, 9, 10]);
    expect(disclosure.hasMore.value).toBe(true);

    disclosure.toggle();
    expect(disclosure.visible.value).toHaveLength(12);
    expect(disclosure.hasMore.value).toBe(false);

    disclosure.toggle();
    expect(disclosure.visible.value).toEqual([1, 2, 3, 4, 5]);
  });

  it("normalizes the visible count when the source shrinks and grows", async () => {
    const source = ref(Array.from({ length: 12 }, (_, index) => index + 1));
    const disclosure = useConversationDisclosure(() => source.value);

    disclosure.toggle();
    disclosure.toggle();
    expect(disclosure.visible.value).toHaveLength(12);

    source.value = [1, 2, 3];
    await nextTick();
    expect(disclosure.visible.value).toEqual([1, 2, 3]);
    expect(disclosure.hasMore.value).toBe(false);
    expect(disclosure.canToggle.value).toBe(false);

    source.value = [1, 2, 3, 4, 5, 6, 7];
    await nextTick();
    expect(disclosure.visible.value).toEqual([1, 2, 3, 4, 5]);
    expect(disclosure.hasMore.value).toBe(true);
  });
});
