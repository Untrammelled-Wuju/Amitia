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
export type AiAvatarShape = "rounded" | "circle" | "custom";
export const aiAvatarShapeStorageKey = "amitia.chat.ai-avatar-shape.desktop.v1";
const avatarShape = ref(loadAvatarShape());
const aiAvatarShape = computed(() => avatarShape.value.shape);
const aiAvatarRoundness = computed(() => avatarShape.value.roundness);
const aiAvatarRadius = computed(() => `${aiAvatarShape.value === "circle" ? 50 : aiAvatarShape.value === "custom" ? aiAvatarRoundness.value / 2 : 32}%`);
function loadAvatarShape(): { shape: AiAvatarShape; roundness: number } {
  try {
    const saved = JSON.parse(localStorage.getItem(aiAvatarShapeStorageKey) || "null");
    if (saved && ["rounded", "circle", "custom"].includes(saved.shape) && Number.isFinite(saved.roundness)) {
      return { shape: saved.shape, roundness: Math.min(100, Math.max(0, saved.roundness)) };
    }
  } catch {}
  return { shape: "rounded", roundness: 64 };
}
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
  function setAiAvatarShape(shape: AiAvatarShape, roundness = aiAvatarRoundness.value) {
    if (!["rounded", "circle", "custom"].includes(shape) || !Number.isFinite(roundness)) throw new Error("未知 AI 头像形状");
    const next = { shape, roundness: Math.min(100, Math.max(0, roundness)) };
    localStorage.setItem(aiAvatarShapeStorageKey, JSON.stringify(next));
    avatarShape.value = next;
  }
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
  return { messageStyle: readonly(messageStyle), setMessageStyle, userMessageGlass, setUserMessageGlass, userMessageWaterGlass, setUserMessageWaterGlass, aiAvatarEnabled: readonly(aiAvatarEnabled), setAiAvatarEnabled, aiAvatarShape, aiAvatarRoundness, aiAvatarRadius, setAiAvatarShape, aiNameEnabled: readonly(aiNameEnabled), setAiNameEnabled };
}
