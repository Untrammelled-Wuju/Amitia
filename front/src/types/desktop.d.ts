import type { AmitiaDesktopAPI } from "../runtime/runtime-types";

interface ElectronWindowApi {
  readonly platform: string;
  getWindowState(): Promise<{ fullscreen: boolean; maximized: boolean }>;
  toggleFullscreen(): Promise<void>;
  syncCaptionColors(colors: { color: string; symbolColor: string }): Promise<void>;
  onWindowStateChanged(callback: (state: { fullscreen: boolean; maximized: boolean }) => void): () => void;
  minimize(windowType?: "main" | "child"): Promise<void>;
  toggleMaximize(): Promise<boolean>;
  close(windowType?: "main" | "child"): Promise<void>;
  editCommand(command: "undo" | "redo" | "cut" | "copy" | "paste" | "selectAll" | "delete"): Promise<void>;
  isMaximized(): Promise<boolean>;
  getWindowType(): Promise<"main" | "child">;
}

declare global {
  interface Window {
    amitiaDesktop?: AmitiaDesktopAPI;
    electronWindowApi?: ElectronWindowApi;
  }
}

export {};
