import { ref } from "vue";

export type ChatPermissionMode = "request_approval" | "full_access";

export const CHAT_PERMISSION_MODE_STORAGE_KEY =
  "amitia.chat.permission-mode.v1";

type ChatPermissionStorage = Pick<Storage, "getItem" | "setItem">;

function resolveStorage(storage?: ChatPermissionStorage): ChatPermissionStorage | null {
  if (storage) return storage;
  if (typeof window === "undefined") return null;
  return window.localStorage;
}

export function normalizeChatPermissionMode(value: unknown): ChatPermissionMode {
  return value === "full_access" ? "full_access" : "request_approval";
}

export function loadChatPermissionMode(
  storage?: ChatPermissionStorage,
): ChatPermissionMode {
  const target = resolveStorage(storage);
  if (!target) return "request_approval";
  try {
    const stored = target.getItem(CHAT_PERMISSION_MODE_STORAGE_KEY);
    if (stored === "full_access" || stored === "request_approval") {
      return stored;
    }
  } catch (error) {
  console.error("[original-error]", error);
return "request_approval";
  }
  return "request_approval";
}

export function saveChatPermissionMode(
  value: unknown,
  storage?: ChatPermissionStorage,
) {
  const target = resolveStorage(storage);
  if (!target) return;
  try {
    target.setItem(
      CHAT_PERMISSION_MODE_STORAGE_KEY,
      normalizeChatPermissionMode(value),
    );
  } catch (error) {
  console.error("[original-error]", error);
}
}

export function useChatPermissionPreference() {
  const permissionMode = ref<ChatPermissionMode>(loadChatPermissionMode());

  function setPermissionMode(value: unknown) {
    const next = normalizeChatPermissionMode(value);
    permissionMode.value = next;
    saveChatPermissionMode(next);
  }

  function reloadPermissionMode() {
    permissionMode.value = loadChatPermissionMode();
  }

  return {
    permissionMode,
    setPermissionMode,
    reloadPermissionMode,
  };
}
