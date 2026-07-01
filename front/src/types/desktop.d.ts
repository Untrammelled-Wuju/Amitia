import type { AmitiaDesktopAPI } from "../runtime/runtime-types"

declare global {
  interface Window {
    amitiaDesktop?: AmitiaDesktopAPI
  }
}

export {}
