import { readonly, ref } from "vue";

export type ChatMessageStyle = "flow" | "bubble";
export const chatMessageStyleStorageKey = "amitia.chat.message-style.desktop.v1";

function load(): ChatMessageStyle {
  try { return localStorage.getItem(chatMessageStyleStorageKey) === "bubble" ? "bubble" : "flow"; }
  catch { return "flow"; }
}

const messageStyle = ref<ChatMessageStyle>(load());

export function useChatAppearancePreference() {
  function setMessageStyle(value: ChatMessageStyle) {
    if (value !== "flow" && value !== "bubble") throw new Error("未知聊天风格");
    localStorage.setItem(chatMessageStyleStorageKey, value);
    messageStyle.value = value;
  }
  return { messageStyle: readonly(messageStyle), setMessageStyle };
}
