import packageMeta from "../../package.json";

export interface UIClientInfo {
  architecture: string;
  appVersion: string;
}

function normalizeArchitecture(raw: string): string {
  const value = raw.trim().toLowerCase();
  if (!value) return "";
  if (/arm64|aarch64/.test(value)) return "arm64";
  if (/x86_64|x64|win64|amd64/.test(value)) return "x86_64";
  if (/ia32|i[3-6]86|x86/.test(value)) return "x86";
  if (/armv7|\barm\b/.test(value)) return "armv7";
  return value;
}

function browserArchitecture(): string {
  if (typeof navigator === "undefined") return "";
  return normalizeArchitecture(`${navigator.platform ?? ""} ${navigator.userAgent ?? ""}`);
}

export function getUIClientInfo(): UIClientInfo {
  return {
    architecture: browserArchitecture(),
    appVersion: String(packageMeta.version ?? "").trim(),
  };
}

export async function resolveUIClientInfo(): Promise<UIClientInfo> {
  if (typeof window !== "undefined") {
    try {
      const environment = await window.amitiaDesktop?.getEnvironment?.();
      if (environment) {
        return {
          architecture: normalizeArchitecture(environment.arch),
          appVersion: environment.version?.trim() || String(packageMeta.version ?? "").trim(),
        };
      }
    } catch {
      // Fall back to browser-visible metadata below.
    }
  }
  return getUIClientInfo();
}

export async function uiClientQueryParams(): Promise<Record<string, string>> {
  const info = await resolveUIClientInfo();
  return {
    ...(info.architecture ? { architecture: info.architecture } : {}),
    ...(info.appVersion ? { appVersion: info.appVersion } : {}),
  };
}
