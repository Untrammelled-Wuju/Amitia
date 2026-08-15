import type { DeploymentModeConfig } from "./types";

export class DeploymentConfigError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "DeploymentConfigError";
  }
}

function isPrivateHost(hostname: string): boolean {
  if (hostname === "127.0.0.1" || hostname === "localhost") return true;
  if (hostname.startsWith("10.")) return true;
  if (hostname.startsWith("192.168.")) return true;
  const parts = hostname.split(".");
  if (parts.length === 4 && parts[0] === "172") {
    const second = parseInt(parts[1], 10);
    if (second >= 16 && second <= 31) return true;
  }
  return false;
}

function normalizeServerURL(raw: string): string {
  let url = raw.trim().replace(/\/+$/, "");

  if (!/^https?:\/\//i.test(url)) {
    let hostPart = url;
    const slashIdx = hostPart.indexOf("/");
    if (slashIdx !== -1) hostPart = hostPart.slice(0, slashIdx);

    const colonIdx = hostPart.lastIndexOf(":");
    let hostname = hostPart;
    if (colonIdx !== -1) {
      const portStr = hostPart.slice(colonIdx + 1);
      if (/^\d+$/.test(portStr)) {
        hostname = hostPart.slice(0, colonIdx);
      }
    }

    if (isPrivateHost(hostname)) {
      url = "http://" + url;
    } else {
      url = "https://" + url;
    }
  }

  const parsed = new URL(url);

  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    throw new DeploymentConfigError("服务器地址只允许 http:// 或 https:// 协议");
  }

  if (parsed.username || parsed.password) {
    throw new DeploymentConfigError("服务器地址不允许包含凭据信息");
  }

  if (parsed.search) {
    throw new DeploymentConfigError("服务器地址不允许包含查询参数");
  }

  const pathname = parsed.pathname.replace(/\/+$/, "");
  if (pathname && pathname !== "/") {
    throw new DeploymentConfigError("当前版本仅支持远程Core根地址，不支持子路径部署");
  }

  parsed.hash = "";
  parsed.search = "";
  parsed.pathname = "";

  return parsed.origin;
}

export function validateDeploymentConfig(raw: unknown): DeploymentModeConfig {
  if (!raw || typeof raw !== "object") {
    return { mode: "local" };
  }

  const obj = raw as Record<string, unknown>;
  const mode = obj.mode;

  if (mode === "cloud") {
    const serverURL = obj.serverURL;
    if (typeof serverURL !== "string" || serverURL.trim().length === 0) {
      return { mode: "local" };
    }
    try {
      const normalized = normalizeServerURL(serverURL);
      return { mode: "cloud", serverURL: normalized };
    } catch {
      return { mode: "local" };
    }
  }

  return { mode: "local" };
}

export function validateDeploymentConfigForSave(raw: unknown): DeploymentModeConfig {
  if (!raw || typeof raw !== "object") {
    throw new DeploymentConfigError("无效的配置数据");
  }

  const obj = raw as Record<string, unknown>;
  const mode = obj.mode;

  if (mode === "cloud") {
    const serverURL = obj.serverURL;
    if (typeof serverURL !== "string" || serverURL.trim().length === 0) {
      throw new DeploymentConfigError("云端模式必须提供服务器地址");
    }
    const normalized = normalizeServerURL(serverURL);
    return { mode: "cloud", serverURL: normalized };
  }

  if (mode === "local") {
    return { mode: "local" };
  }

  throw new DeploymentConfigError("无效的部署模式");
}

export function configToLabel(config: DeploymentModeConfig): string {
  if (config.mode === "cloud") {
    return config.serverURL ? `云端 (${config.serverURL})` : "云端";
  }
  return "本地";
}
