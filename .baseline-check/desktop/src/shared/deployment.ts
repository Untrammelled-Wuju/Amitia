import type { DeploymentModeConfig } from "./types";

export class DeploymentConfigError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "DeploymentConfigError";
  }
}

function isPrivateHost(hostname: string): boolean {
  const host = hostname.toLowerCase().replace(/^\[|\]$/g, "");
  if (host === "localhost" || host.endsWith(".localhost")) return true;
  if (host === "::1" || host === "0:0:0:0:0:0:0:1") return true;
  if (/^(fc|fd)[0-9a-f]{2}:/i.test(host) || host.startsWith("fe80:")) return true;

  const parts = host.split(".");
  if (parts.length !== 4) return false;
  const octets = parts.map((part) => Number.parseInt(part, 10));
  if (octets.some((part, index) => !/^\d+$/.test(parts[index]) || part < 0 || part > 255)) {
    return false;
  }
  const [a, b] = octets;
  if (a === 127 || a === 10) return true;
  if (a === 192 && b === 168) return true;
  if (a === 172 && b >= 16 && b <= 31) return true;
  if (a === 169 && b === 254) return true;
  return false;
}

function normalizeServerURL(raw: string): string {
  let url = raw.trim();
  if (url.startsWith("//")) url = url.slice(2);

  if (!/^https?:\/\//i.test(url)) {
    const probe = new URL(`http://${url}`);
    url = `${isPrivateHost(probe.hostname) ? "http" : "https"}://${url}`;
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

  if (parsed.hash) {
    throw new DeploymentConfigError("服务器地址不允许包含片段标识");
  }

  const pathname = parsed.pathname.replace(/\/+$/, "");
  if (pathname && pathname !== "/") {
    throw new DeploymentConfigError("当前版本仅支持远程Core根地址，不支持子路径部署");
  }

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
