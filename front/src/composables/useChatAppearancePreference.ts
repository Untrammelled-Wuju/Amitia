import { computed, readonly, ref } from "vue";

export type ChatMessageStyle = "flow" | "bubble";
export const chatMessageStyleStorageKey = "amitia.chat.message-style.desktop.v1";

function load(): ChatMessageStyle {
  try { return localStorage.getItem(chatMessageStyleStorageKey) === "bubble" ? "bubble" : "flow"; }
  catch { return "flow"; }
}

const messageStyle = ref<ChatMessageStyle>(load());
export const aiAvatarStorageKey = "amitia.chat.ai-avatar.desktop.v1";
const aiAvatarEnabled = ref(loadAvatar());
export const aiNameStorageKey = "amitia.chat.ai-name.desktop.v1";
const aiNameEnabled = ref(loadName());
function loadName(): boolean {
  try { return localStorage.getItem(aiNameStorageKey) !== "false"; }
  catch { return true; }
}
function loadAvatar(): boolean {
  try { return localStorage.getItem(aiAvatarStorageKey) !== "false"; }
  catch { return true; }
}
export const userMessageGlassStorageKey = "amitia.chat.user-message-glass.desktop.v1";
export type UserMessageMaterial = "solid" | "frosted" | "water";
export const userMessageMaterialStorageKey = "amitia.chat.user-message-material.desktop.v1";
const userMessageMaterial = ref<UserMessageMaterial>(loadMaterial());
const userMessageGlass = computed(() => userMessageMaterial.value === "frosted");
const userMessageWaterGlass = computed(() => userMessageMaterial.value === "water");

function loadMaterial(): UserMessageMaterial {
  try {
    const saved = localStorage.getItem(userMessageMaterialStorageKey);
    if (saved === "solid" || saved === "frosted" || saved === "water") return saved;
    return localStorage.getItem(userMessageGlassStorageKey) === "true" ? "frosted" : "solid";
  }
  catch { return "solid"; }
}

export function useChatAppearancePreference() {
  function setAiNameEnabled(value: boolean) {
    localStorage.setItem(aiNameStorageKey, String(value));
    aiNameEnabled.value = value;
  }
  function setAiAvatarEnabled(value: boolean) {
    localStorage.setItem(aiAvatarStorageKey, String(value));
    aiAvatarEnabled.value = value;
  }
  function setMessageStyle(value: ChatMessageStyle) {
    if (value !== "flow" && value !== "bubble") throw new Error("未知聊天风格");
    localStorage.setItem(chatMessageStyleStorageKey, value);
    messageStyle.value = value;
  }
  function setUserMessageGlass(value: boolean) {
    setMaterial(value ? "frosted" : userMessageGlass.value ? "solid" : userMessageMaterial.value);
  }
  function setUserMessageWaterGlass(value: boolean) {
    setMaterial(value ? "water" : userMessageWaterGlass.value ? "solid" : userMessageMaterial.value);
  }
  function setMaterial(value: UserMessageMaterial) {
    localStorage.setItem(userMessageMaterialStorageKey, value);
    userMessageMaterial.value = value;
  }
  return { messageStyle: readonly(messageStyle), setMessageStyle, userMessageGlass, setUserMessageGlass, userMessageWaterGlass, setUserMessageWaterGlass, aiAvatarEnabled: readonly(aiAvatarEnabled), setAiAvatarEnabled, aiNameEnabled: readonly(aiNameEnabled), setAiNameEnabled };
}
