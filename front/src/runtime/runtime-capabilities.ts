export function isDesktopShell(): boolean {
  return typeof window !== "undefined" && !!window.amitiaDesktop
}

export function shouldUseHashRouting(): boolean {
  return isDesktopShell() || (typeof window !== "undefined" && window.location.protocol === "file:")
}

export function shouldRegisterServiceWorker(): boolean {
  return typeof window !== "undefined" && window.location.protocol !== "file:" && !isDesktopShell()
}
