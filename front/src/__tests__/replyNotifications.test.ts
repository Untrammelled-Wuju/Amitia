import { beforeEach, describe, expect, it, vi } from "vitest";

describe("reply notifications", () => {
  beforeEach(() => {
    vi.resetModules();
    vi.restoreAllMocks();
    localStorage.clear();
    delete window.amitiaDesktop;
    vi.spyOn(document, "hasFocus").mockReturnValue(false);
  });

  it("defaults off and persists changes", async () => {
    const show = vi.fn();
    window.amitiaDesktop = { showReplyNotification: show } as any;
    const module = await import("@/composables/useReplyNotifications");
    await module.notifyReplyCompleted("chat", "turn");
    expect(show).not.toHaveBeenCalled();
    await module.useReplyNotifications().setEnabled(true);
    expect(localStorage.getItem("amitia.reply-notifications.desktop.v1")).toBe("true");
    await module.notifyReplyCompleted("chat", "turn");
    await module.notifyReplyCompleted("chat", "turn");
    expect(show).toHaveBeenCalledTimes(1);
  });

  it("suppresses foreground completions even after losing focus", async () => {
    localStorage.setItem("amitia.reply-notifications.desktop.v1", "true");
    const show = vi.fn();
    window.amitiaDesktop = { showReplyNotification: show } as any;
    vi.spyOn(document, "visibilityState", "get").mockReturnValue("visible");
    vi.mocked(document.hasFocus).mockReturnValue(true);
    const module = await import("@/composables/useReplyNotifications");
    await module.notifyReplyCompleted("chat", "turn");
    vi.mocked(document.hasFocus).mockReturnValue(false);
    await module.notifyReplyCompleted("chat", "turn");
    expect(show).not.toHaveBeenCalled();
  });

  it("does not enable when browser permission is denied", async () => {
    vi.stubGlobal("Notification", { requestPermission: vi.fn().mockResolvedValue("denied") });
    const module = await import("@/composables/useReplyNotifications");
    await expect(module.useReplyNotifications().setEnabled(true)).rejects.toThrow("权限");
    expect(module.useReplyNotifications().enabled.value).toBe(false);
    expect(localStorage.getItem("amitia.reply-notifications.desktop.v1")).toBeNull();
    vi.unstubAllGlobals();
  });

  it("notification failures do not interrupt completion", async () => {
    localStorage.setItem("amitia.reply-notifications.desktop.v1", "true");
    window.amitiaDesktop = { showReplyNotification: vi.fn().mockRejectedValue(new Error("disabled")) } as any;
    const module = await import("@/composables/useReplyNotifications");
    await expect(module.notifyReplyCompleted("chat", "turn")).resolves.toBeUndefined();
  });
  it("sound can be enabled without displaying a notification and persists", async () => {
    const show = vi.fn();
    window.amitiaDesktop = { showReplyNotification: show } as any;
    const module = await import("@/composables/useReplyNotifications");
    expect(module.useReplyNotifications().soundEnabled.value).toBe(false);
    await module.useReplyNotifications().setSoundEnabled(true);
    expect(localStorage.getItem("amitia.reply-sound.desktop.v1")).toBe("true");
    await module.notifyReplyCompleted("chat", "turn");
    await module.notifyReplyCompleted("chat", "turn");
    expect(show).toHaveBeenCalledTimes(1);
    expect(show).toHaveBeenCalledWith({ notify: false, sound: true });
  });
  it("suppresses sound in foreground and sends one combined background request", async () => {
    localStorage.setItem("amitia.reply-notifications.desktop.v1", "true");
    localStorage.setItem("amitia.reply-sound.desktop.v1", "true");
    const show = vi.fn();
    window.amitiaDesktop = { showReplyNotification: show } as any;
    vi.spyOn(document, "visibilityState", "get").mockReturnValue("visible");
    vi.mocked(document.hasFocus).mockReturnValue(true);
    const module = await import("@/composables/useReplyNotifications");
    await module.notifyReplyCompleted("chat", "foreground");
    expect(show).not.toHaveBeenCalled();
    vi.mocked(document.hasFocus).mockReturnValue(false);
    await module.notifyReplyCompleted("chat", "background");
    expect(show).toHaveBeenCalledTimes(1);
    expect(show).toHaveBeenCalledWith({ notify: true, sound: true });
    await module.useReplyNotifications().setSoundEnabled(false);
    await module.notifyReplyCompleted("chat", "silent");
    expect(show).toHaveBeenLastCalledWith({ notify: true, sound: false });
  });
});
