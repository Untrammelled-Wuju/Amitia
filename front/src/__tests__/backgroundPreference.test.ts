import { beforeEach, describe, expect, it, vi } from "vitest";
import { backgroundFileKind, defaultBackground, normalizeBackground } from "../composables/backgroundStorage";
const storage = vi.hoisted(() => ({ readBackground: vi.fn(), writeBackground: vi.fn() }));
vi.mock("../composables/backgroundStorage", async original => ({ ...await original<typeof import("../composables/backgroundStorage")>(), ...storage }));

describe("background preference", () => {
  beforeEach(() => {
    vi.resetModules(); storage.readBackground.mockReset(); storage.writeBackground.mockReset();
    storage.readBackground.mockResolvedValue({ settings: { ...defaultBackground }, media: null });
    storage.writeBackground.mockResolvedValue(undefined);
    URL.createObjectURL = vi.fn().mockReturnValue("blob:background");
    URL.revokeObjectURL = vi.fn();
  });
  it("validates formats, limits and corrupt settings", () => {
    expect(backgroundFileKind({ name: "photo.PNG", size: 1 })).toBe("image");
    expect(backgroundFileKind({ name: "clip.mp4", size: 1 })).toBe("video");
    expect(() => backgroundFileKind({ name: "script.html", size: 1 })).toThrow();
    expect(() => backgroundFileKind({ name: "big.jpg", size: 21 * 1024 * 1024 })).toThrow();
    expect(() => backgroundFileKind({ name: "empty.mp4", size: 0 })).toThrow();
    expect(normalizeBackground({ opacity: -1, blurRadius: 100, kind: "invalid" })).toMatchObject({ opacity: 0, blurRadius: 30, kind: "image" });
  });
  it("stores the file, preserves it while disabled, restores it and releases removed URLs", async () => {
    const preferences = (await import("../composables/useBackgroundPreference")).useBackgroundPreference();
    await preferences.init();
    const file = new File(["image"], "photo.png", { type: "image/png" });
    await preferences.selectFile(file);
    expect(storage.writeBackground).toHaveBeenLastCalledWith(expect.objectContaining({ enabled: true, name: "photo.png" }), file);
    await preferences.update({ enabled: false, opacity: 0.6, blurEnabled: true, blurRadius: 12 });
    expect(preferences.source.value).toBe("blob:background");
    const saved = { ...preferences.settings.value };
    vi.resetModules();
    storage.readBackground.mockResolvedValue({ settings: saved, media: file });
    const restored = (await import("../composables/useBackgroundPreference")).useBackgroundPreference();
    await restored.init();
    expect(restored.settings.value).toMatchObject({ enabled: false, opacity: 0.6, blurRadius: 12 });
    await restored.update({ enabled: true });
    await restored.clear();
    expect(restored.source.value).toBe("");
    expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:background");
    expect(storage.writeBackground).toHaveBeenLastCalledWith(expect.objectContaining({ enabled: false, name: "" }), null);
  });
  it("keeps the previous background when saving fails and serializes slider changes", async () => {
    const preferences = (await import("../composables/useBackgroundPreference")).useBackgroundPreference();
    await preferences.selectFile(new File(["image"], "photo.png"));
    storage.writeBackground.mockRejectedValueOnce(new Error("quota"));
    await expect(preferences.selectFile(new File(["video"], "clip.mp4"))).rejects.toThrow("quota");
    expect(preferences.settings.value.name).toBe("photo.png");
    await Promise.all([preferences.update({ opacity: 0.8 }), preferences.update({ blurRadius: 16 })]);
    expect(preferences.settings.value).toMatchObject({ opacity: 0.8, blurRadius: 16 });
  });
});
