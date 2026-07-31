import { contextBridge, ipcRenderer } from "electron";

type PetManagerState =
  | "uninitialized"
  | "ready"
  | "enabled"
  | "disabled"
  | "invalid";

type AssistantState =
  | "assistant_listening"
  | "assistant_thinking"
  | "assistant_speaking"
  | "assistant_finished"
  | "assistant_error";

interface PetActionSwitchPayload {
  actionKey: string;
  previousActionKey: string | null;
  source: string;
}

interface PetLoadErrorPayload {
  installationId: string;
  error: string;
  errorCode: string;
}

interface PetStatePayload {
  state: PetManagerState;
  installationId: string | null;
  reason?: string;
}

interface ChatStatePetPayload {
  state: AssistantState;
  actionKey: string;
  roundId: string | null;
}

const petApi = {
  onActionSwitch(
    callback: (payload: PetActionSwitchPayload) => void,
  ): () => void {
    const listener = (
      _event: Electron.IpcRendererEvent,
      payload: PetActionSwitchPayload,
    ) => callback(payload);
    ipcRenderer.on("pet:action-switch", listener);
    return () => ipcRenderer.removeListener("pet:action-switch", listener);
  },
  onLoadError(callback: (payload: PetLoadErrorPayload) => void): () => void {
    const listener = (
      _event: Electron.IpcRendererEvent,
      payload: PetLoadErrorPayload,
    ) => callback(payload);
    ipcRenderer.on("pet:load-error", listener);
    return () => ipcRenderer.removeListener("pet:load-error", listener);
  },
  onState(callback: (payload: PetStatePayload) => void): () => void {
    const listener = (
      _event: Electron.IpcRendererEvent,
      payload: PetStatePayload,
    ) => callback(payload);
    ipcRenderer.on("pet:state", listener);
    return () => ipcRenderer.removeListener("pet:state", listener);
  },
  onChatState(callback: (payload: ChatStatePetPayload) => void): () => void {
    const listener = (
      _event: Electron.IpcRendererEvent,
      payload: ChatStatePetPayload,
    ) => callback(payload);
    ipcRenderer.on("pet:chat-state", listener);
    return () => ipcRenderer.removeListener("pet:chat-state", listener);
  },
  sendClick(x: number, y: number): void {
    ipcRenderer.send("pet:click", { x, y });
  },
  sendDoubleClick(x: number, y: number): void {
    ipcRenderer.send("pet:double-click", { x, y });
  },
  sendHover(x: number, y: number): void {
    ipcRenderer.send("pet:hover", { x, y });
  },
};

contextBridge.exposeInMainWorld("petApi", petApi);
