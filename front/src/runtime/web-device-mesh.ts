export interface WebDeviceMeshIdentity {
  deviceId: string;
  runtimeId: string;
  platform: "web";
}

export interface WebDeviceCredentialRecord extends WebDeviceMeshIdentity {
  cloudBaseURL: string;
  credentialId: string;
  credential: string;
  spaceId: string;
  expiresAt: string;
}

export interface PairingStatus {
  spaceId: string;
  trustedDeviceCount: number;
  firstDeviceSetupRequired: boolean;
}

export interface PairingInput {
  offerToken?: string;
  setupCode?: string;
}

export interface PairingOffer {
  offerId: string;
  offerToken: string;
  qrPayload: string;
  expiresAt: string;
}

const IDENTITY_KEY = "amitia.web.mesh.identity.v1";
const CREDENTIAL_PREFIX = "amitia.web.mesh.credential.v1:";

function normalizeBaseURL(raw: string): string {
  const url = new URL(raw, window.location.origin);
  url.hash = "";
  url.search = "";
  url.pathname = url.pathname.replace(/\/+$/, "") || "/";
  return url.toString().replace(/\/+$/, "");
}

function randomPart(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID().replace(/-/g, "");
  }
  const bytes = new Uint8Array(24);
  if (typeof crypto !== "undefined" && typeof crypto.getRandomValues === "function") {
    crypto.getRandomValues(bytes);
    return Array.from(bytes, (value) => value.toString(16).padStart(2, "0")).join("");
  }
  return `${Date.now().toString(36)}${Math.random().toString(36).slice(2)}${Math.random().toString(36).slice(2)}`;
}

function credentialKey(baseURL: string): string {
  return `${CREDENTIAL_PREFIX}${encodeURIComponent(normalizeBaseURL(baseURL))}`;
}

function readJSON<T>(key: string): T | null {
  try {
    const raw = window.localStorage.getItem(key);
    return raw ? (JSON.parse(raw) as T) : null;
  } catch {
    return null;
  }
}

function writeJSON(key: string, value: unknown): void {
  window.localStorage.setItem(key, JSON.stringify(value));
}

function unwrapJSON<T>(payload: any): T {
  if (payload && typeof payload === "object" && payload.data !== undefined && typeof payload.code === "number") {
    if (payload.code !== 200) throw new Error(String(payload.msg || payload.message || `请求失败 (${payload.code})`));
    return payload.data as T;
  }
  return payload as T;
}

async function readResponse<T>(response: Response): Promise<T> {
  let payload: any = null;
  try {
    payload = await response.json();
  } catch {
    payload = null;
  }
  if (!response.ok) {
    throw new Error(String(payload?.message || payload?.msg || `请求失败 (${response.status})`));
  }
  return unwrapJSON<T>(payload);
}

export function getWebDeviceMeshIdentity(): WebDeviceMeshIdentity {
  const existing = readJSON<WebDeviceMeshIdentity>(IDENTITY_KEY);
  if (existing?.deviceId && existing?.runtimeId && existing.platform === "web") return existing;

  const suffix = randomPart();
  const identity: WebDeviceMeshIdentity = {
    deviceId: `dev_web_${suffix}`,
    runtimeId: `rt_web_${randomPart()}`,
    platform: "web",
  };
  writeJSON(IDENTITY_KEY, identity);
  return identity;
}

export function getWebDeviceCredential(baseURL: string): WebDeviceCredentialRecord | null {
  const normalized = normalizeBaseURL(baseURL);
  const record = readJSON<WebDeviceCredentialRecord>(credentialKey(normalized));
  if (!record?.credential || !record.deviceId || !record.runtimeId || !record.spaceId) return null;
  if (normalizeBaseURL(record.cloudBaseURL) !== normalized) return null;
  if (record.expiresAt) {
    const expiry = Date.parse(record.expiresAt);
    if (Number.isFinite(expiry) && expiry <= Date.now() + 30_000) {
      clearWebDeviceCredential(normalized);
      return null;
    }
  }
  return record;
}

export function clearWebDeviceCredential(baseURL: string): void {
  try {
    window.localStorage.removeItem(credentialKey(baseURL));
  } catch {
    // Browser storage may be disabled. The caller will simply remain unpaired.
  }
}

export function getWebDeviceAuthHeaders(baseURL: string): Record<string, string> {
  const record = getWebDeviceCredential(baseURL);
  if (!record) return {};
  return {
    Authorization: `AmitiaDevice ${record.credential}`,
    "X-Amitia-Device-ID": record.deviceId,
    "X-Amitia-Runtime-ID": record.runtimeId,
    "X-Amitia-Client-Type": "web",
  };
}

export async function getWebPairingStatus(baseURL: string): Promise<PairingStatus> {
  const normalized = normalizeBaseURL(baseURL);
  const response = await fetch(`${normalized}/api/public/device-mesh/v1/pairing/status`, {
    headers: { Accept: "application/json", "X-Amitia-Client-Type": "web" },
    credentials: "include",
  });
  return readResponse<PairingStatus>(response);
}

export async function provisionWebDevice(baseURL: string, pairing: PairingInput): Promise<WebDeviceCredentialRecord> {
  const normalized = normalizeBaseURL(baseURL);
  const identity = getWebDeviceMeshIdentity();
  const claimResponse = await fetch(`${normalized}/api/public/device-mesh/v1/pairing/claim`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json", "X-Amitia-Client-Type": "web" },
    credentials: "include",
    body: JSON.stringify({
      deviceId: identity.deviceId,
      runtimeId: identity.runtimeId,
      platform: identity.platform,
      label: `Web ${navigator.platform || navigator.userAgent || "Browser"}`.slice(0, 96),
      offerToken: pairing.offerToken?.trim() || undefined,
      setupCode: pairing.setupCode?.trim() || undefined,
    }),
  });
  const ticket = await readResponse<{ ticket: string; spaceId: string }>(claimResponse);
  if (!ticket?.ticket) throw new Error("Cloud Core 未返回 Bootstrap Ticket");

  const exchangeResponse = await fetch(`${normalized}/api/public/device-mesh/v1/bootstrap/exchange`, {
    method: "POST",
    headers: {
      Authorization: `AmitiaBootstrap ${ticket.ticket}`,
      "Content-Type": "application/json",
      Accept: "application/json",
      "X-Amitia-Client-Type": "web",
    },
    credentials: "include",
    body: JSON.stringify({
      deviceId: identity.deviceId,
      runtimeId: identity.runtimeId,
      platform: identity.platform,
    }),
  });
  const exchanged = await readResponse<{
    credentialId: string;
    credential: string;
    spaceId: string;
    deviceId: string;
    runtimeId: string;
    expiresAt: string;
  }>(exchangeResponse);
  if (!exchanged?.credential) throw new Error("Cloud Core 未返回 Device Credential");

  const record: WebDeviceCredentialRecord = {
    cloudBaseURL: normalized,
    credentialId: exchanged.credentialId,
    credential: exchanged.credential,
    spaceId: exchanged.spaceId || ticket.spaceId,
    deviceId: exchanged.deviceId || identity.deviceId,
    runtimeId: exchanged.runtimeId || identity.runtimeId,
    platform: "web",
    expiresAt: exchanged.expiresAt,
  };
  writeJSON(credentialKey(normalized), record);
  return record;
}

export async function createWebPairingOffer(baseURL: string, ttlSeconds = 600): Promise<PairingOffer> {
  const normalized = normalizeBaseURL(baseURL);
  const auth = getWebDeviceAuthHeaders(normalized);
  if (!auth.Authorization) throw new Error("当前浏览器尚未配对到 Cloud Core");
  const response = await fetch(`${normalized}/api/device-mesh/v1/pairing/offers`, {
    method: "POST",
    headers: { ...auth, "Content-Type": "application/json", Accept: "application/json" },
    credentials: "include",
    body: JSON.stringify({ ttlSeconds }),
  });
  const offer = await readResponse<PairingOffer>(response);
  const endpoint = new URL(normalized).origin;
  return {
    ...offer,
    qrPayload: `amitia://pair?endpoint=${encodeURIComponent(endpoint)}&offer=${encodeURIComponent(offer.offerToken)}`,
  };
}
