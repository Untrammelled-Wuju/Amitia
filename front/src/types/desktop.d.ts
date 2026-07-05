import type { AmitiaDesktopAPI } from "../runtime/runtime-types"

interface ElectronWindowApi {
  minimize(windowType?: "main" | "child"): Promise<void>
  toggleMaximize(): Promise<boolean>
  close(windowType?: "main" | "child"): Promise<void>
  isMaximized(): Promise<boolean>
  getWindowType(): Promise<"main" | "child">
}

declare global {
  interface Window {
    amitiaDesktop?: AmitiaDesktopAPI
    electronWindowApi?: ElectronWindowApi
  }
}

export {}
