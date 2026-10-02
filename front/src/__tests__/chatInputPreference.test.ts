import { beforeEach, describe, expect, it, vi } from "vitest";

describe("chat input preference", () => {
  beforeEach(() => { vi.resetModules(); localStorage.clear(); });
  it("shares and persists the desktop preference", async () => {
    const { useChatInputPreference } = await import("../composables/useChatInputPreference");
    const first = useChatInputPreference();
    expect(first.sendOnEnter.value).toBe(true);
    first.setSendOnEnter(false);
    expect(useChatInputPreference().sendOnEnter.value).toBe(false);
    vi.resetModules();
    expect((await import("../composables/useChatInputPreference")).useChatInputPreference().sendOnEnter.value).toBe(false);
  });
  it("protects composition and newline shortcuts", async () => {
    const { shouldEnterSend } = await import("../composables/useChatInputPreference");
    expect(shouldEnterSend(new KeyboardEvent("keydown", { key: "Enter" }), true)).toBe(true);
    for (const options of [{ shiftKey: true }, { isComposing: true }, { ctrlKey: true }, { repeat: true }]) {
      expect(shouldEnterSend(new KeyboardEvent("keydown", { key: "Enter", ...options }), true)).toBe(false);
    }
    expect(shouldEnterSend(new KeyboardEvent("keydown", { key: "Enter" }), false)).toBe(false);
  });
});
