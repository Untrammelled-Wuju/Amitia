import { ref } from "vue";

export interface HostEnvironment {
  host: "web" | "desktop";
  platform: "windows" | "macos" | "linux" | "web";
  os: "windows" | "macos" | "linux" | "unknown";
}

function detectOS(): "windows" | "macos" | "linux" | "unknown" {
  if (typeof navigator === "undefined") return "unknown";
  const ua = navigator.userAgent.toLowerCase();
  if (ua.includes("win")) return "windows";
  if (ua.includes("mac")) return "macos";
  if (ua.includes("linux")) return "linux";
  return "unknown";
}

function isDesktopShell(): boolean {
  return typeof window !== "undefined" && !!(window as unknown as Record<string, unknown>).amitiaDesktop;
}

let cachedEnvironment: HostEnvironment | null = null;

export function resolveHostEnvironment(): HostEnvironment {
  if (cachedEnvironment) return cachedEnvironment;
  const os = detectOS();
  const host: "web" | "desktop" = isDesktopShell() ? "desktop" : "web";
  const platform: "windows" | "macos" | "linux" | "web" = (host === "desktop" && os !== "unknown") ? os : "web";
  cachedEnvironment = { host, platform, os };
  return cachedEnvironment;
}

export function resetHostEnvironmentCache(): void {
  cachedEnvironment = null;
}

const sharedEnvironment = ref<HostEnvironment>(resolveHostEnvironment());

export function useHostEnvironment() {
  return sharedEnvironment;
}
