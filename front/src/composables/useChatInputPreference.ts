import { readonly, ref } from "vue";

const storageKey = "amitia.chat.send-on-enter.desktop.v1";
function load() {
  try { return localStorage.getItem(storageKey) !== "false"; }
  catch { return true; }
}
const sendOnEnter = ref(load());

export function useChatInputPreference() {
  function setSendOnEnter(value: boolean) {
    localStorage.setItem(storageKey, String(value));
    sendOnEnter.value = value;
  }
  return { sendOnEnter: readonly(sendOnEnter), setSendOnEnter };
}

export function shouldEnterSend(event: KeyboardEvent, enabled: boolean) {
  return enabled && event.key === "Enter" && !event.shiftKey && !event.ctrlKey
    && !event.altKey && !event.metaKey && !event.isComposing && event.keyCode !== 229 && !event.repeat;
}
