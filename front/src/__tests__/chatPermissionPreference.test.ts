import { afterEach, describe, expect, it } from "vitest";
import {
  CHAT_PERMISSION_MODE_STORAGE_KEY,
  loadChatPermissionMode,
  normalizeChatPermissionMode,
  saveChatPermissionMode,
} from "../composables/useChatPermissionPreference";

describe("chat permission preference", () => {
  afterEach(() => {
    localStorage.clear();
  });

  it("defaults to request approval for missing or invalid values", () => {
    expect(loadChatPermissionMode()).toBe("request_approval");
    expect(normalizeChatPermissionMode("invalid")).toBe("request_approval");
  });

  it("persists full access across reloads", () => {
    saveChatPermissionMode("full_access");
    expect(localStorage.getItem(CHAT_PERMISSION_MODE_STORAGE_KEY)).toBe(
      "full_access",
    );
    expect(loadChatPermissionMode()).toBe("full_access");
  });

  it("migrates the legacy model settings draft permission", () => {
    localStorage.setItem(
      "amitia.chat.model-settings.draft.v1",
      JSON.stringify({ permissionMode: "full_access" }),
    );

    expect(loadChatPermissionMode()).toBe("full_access");
    expect(localStorage.getItem(CHAT_PERMISSION_MODE_STORAGE_KEY)).toBe(
      "full_access",
    );
  });
});
