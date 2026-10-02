import { readonly, ref } from "vue";

const key = "amitia.reply-notifications.desktop.v1";
const soundKey = "amitia.reply-sound.desktop.v1";
function load(storageKey: string) {
  try {
    return localStorage.getItem(storageKey) === "true";
  } catch {
    return false;
  }
}

const enabled = ref(load(key));
const soundEnabled = ref(load(soundKey));
let audioContext: AudioContext | undefined;
const delivered = new Set<string>();

export function useReplyNotifications() {
  async function setEnabled(value: boolean) {
    if (value && !window.amitiaDesktop?.showReplyNotification) {
      if (typeof Notification === "undefined" || await Notification.requestPermission() !== "granted") {
        throw new Error("系统通知权限未授予，请在系统或浏览器设置中允许通知");
      }
    }
    localStorage.setItem(key, String(value));
    enabled.value = value;
  }
  async function setSoundEnabled(value: boolean) {
    if (value && !window.amitiaDesktop?.showReplyNotification) {
      audioContext ??= new AudioContext();
      await audioContext.resume();
    }
    localStorage.setItem(soundKey, String(value));
    soundEnabled.value = value;
  }
  return { enabled: readonly(enabled), soundEnabled: readonly(soundEnabled), setEnabled, setSoundEnabled };
}

async function playBrowserSound() {
  audioContext ??= new AudioContext();
  await audioContext.resume();
  if (document.visibilityState === "visible" && document.hasFocus()) return;
  const oscillator = audioContext.createOscillator();
  const gain = audioContext.createGain();
  const start = audioContext.currentTime;
  oscillator.frequency.value = 660;
  gain.gain.setValueAtTime(0, start);
  gain.gain.linearRampToValueAtTime(0.12, start + 0.02);
  gain.gain.exponentialRampToValueAtTime(0.001, start + 0.3);
  oscillator.connect(gain);
  gain.connect(audioContext.destination);
  oscillator.onended = () => { oscillator.disconnect(); gain.disconnect(); };
  oscillator.start(start);
  oscillator.stop(start + 0.3);
}

export async function notifyReplyCompleted(conversationId: string, turnId: string) {
  const id = `${conversationId}:${turnId}`;
  if ((!enabled.value && !soundEnabled.value) || !conversationId || !turnId || delivered.has(id)) return;
  delivered.add(id);
  if (delivered.size > 500) delivered.delete(delivered.values().next().value!);
  try {
    if (document.visibilityState === "visible" && document.hasFocus()) return;
    if (window.amitiaDesktop?.showReplyNotification) {
      await window.amitiaDesktop.showReplyNotification({ notify: enabled.value, sound: soundEnabled.value });
      return;
    }
    if (enabled.value && typeof Notification !== "undefined" && Notification.permission === "granted") {
      const notification = new Notification("Amitia · 回复完成", {
        body: "AI 已完成回复，打开应用查看。",
        tag: id,
        silent: true,
      });
      notification.onclick = () => {
        window.focus();
        notification.close();
      };
    }
    if (soundEnabled.value) await playBrowserSound();
  } catch {}
}
