import { ref, onUnmounted, type Ref } from "vue";
import { useRouter } from "vue-router";
import { ElNotification, ElMessageBox } from "element-plus";
import { resolveApiUrl, createAuthorizedRequestInit } from "../runtime/runtime-adapter";

interface UINotifyPayload {
  extensionId: string;
  title: string;
  body: string;
  severity: string;
  timestamp: string;
}

interface UIDialogPayload {
  dialogId: string;
  extensionId: string;
  title?: string;
  message: string;
  buttons: string[];
  timestamp: string;
}

interface UINavigatePayload {
  extensionId: string;
  target: string;
  timestamp: string;
}

const severityMap: Record<string, "success" | "warning" | "info" | "error"> = {
  info: "info",
  success: "success",
  warning: "warning",
  error: "error",
  critical: "error",
};

export function useUIHostSSE(connected?: Ref<boolean>) {
  const router = useRouter();
  let eventSource: EventSource | null = null;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  const isConnected = connected ?? ref(false);

  async function sendDialogResponse(dialogId: string, result: string): Promise<void> {
    try {
      const url = await resolveApiUrl("/api/extensions/ui/dialog-response");
      const init = await createAuthorizedRequestInit({
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ dialogId, result }),
      });
      await fetch(url, init);
    } catch {
      // ignore
    }
  }

  function handleNotify(event: MessageEvent) {
    try {
      const payload: UINotifyPayload = JSON.parse(event.data);
      const type = severityMap[payload.severity] || "info";
      ElNotification({
        title: payload.title || "通知",
        message: payload.body || "",
        type,
        duration: 5000,
      });
    } catch {
      // ignore
    }
  }

  function handleNavigate(event: MessageEvent) {
    try {
      const payload: UINavigatePayload = JSON.parse(event.data);
      if (payload.target) {
        router.push(payload.target).catch(() => {
          // ignore navigation errors
        });
      }
    } catch {
      // ignore
    }
  }

  function handleDialog(event: MessageEvent) {
    try {
      const payload: UIDialogPayload = JSON.parse(event.data);
      const buttons = payload.buttons && payload.buttons.length > 0
        ? payload.buttons
        : ["确定"];

      ElMessageBox.confirm(payload.message || "", payload.title || "对话框", {
        confirmButtonText: buttons[0],
        cancelButtonText: buttons.length > 1 ? buttons[1] : "取消",
        distinguishCancelAndClose: true,
        type: "info",
      })
        .then(() => {
          void sendDialogResponse(payload.dialogId, buttons[0]);
        })
        .catch((action: string) => {
          if (action === "cancel" && buttons.length > 1) {
            void sendDialogResponse(payload.dialogId, buttons[1]);
          } else {
            void sendDialogResponse(payload.dialogId, "closed");
          }
        });
    } catch {
      // ignore
    }
  }

  async function connect() {
    disconnect();
    try {
      const url = await resolveApiUrl("/api/proactive-sse?clientId=ui-host");
      eventSource = new EventSource(url);
      eventSource.addEventListener("ui_notify", handleNotify);
      eventSource.addEventListener("ui_navigate", handleNavigate);
      eventSource.addEventListener("ui_dialog", handleDialog);
      eventSource.onopen = () => {
        isConnected.value = true;
      };
      eventSource.onerror = () => {
        isConnected.value = false;
        disconnect();
        reconnectTimer = setTimeout(() => {
          void connect();
        }, 5000);
      };
    } catch {
      reconnectTimer = setTimeout(() => {
        void connect();
      }, 5000);
    }
  }

  function disconnect() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
    if (eventSource) {
      eventSource.close();
      eventSource = null;
    }
    isConnected.value = false;
  }

  onUnmounted(() => {
    disconnect();
  });

  return {
    isConnected,
    connect,
    disconnect,
  };
}
