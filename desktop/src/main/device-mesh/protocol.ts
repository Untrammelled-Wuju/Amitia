export const DEVICE_MESH_PROTOCOL_NAME = "amitia.device-runtime";
export const DEVICE_MESH_ENVELOPE_VERSION = 1;
export const DEVICE_MESH_SCHEMA_VERSION = "1.0.0";
export const DEVICE_MESH_CONTRACT_VERSION = "1.0";
export const DEVICE_MESH_WS_PATH = "/api/device-mesh/v1/runtime/ws";

export const LOCAL_MESH_BASE_URL = "http://127.0.0.1:18899";
export const LOCAL_MESH_PREFIX = "/internal/device-mesh";

export type DeviceMeshAgentState =
  | "unprovisioned"
  | "connecting"
  | "handshaking"
  | "ready"
  | "degraded"
  | "backoff"
  | "revoked"
  | "stopped";

export interface DeviceMeshLocalIdentity {
  deviceId: string;
  runtimeId: string;
  platform: string;
}

export interface DeviceMeshStatusResponse {
  state: DeviceMeshAgentState;
  cloudBaseUrl: string;
  deviceId: string;
  runtimeId: string;
  runtimeSessionId: string;
  connectionGeneration: number;
  lastConnectedAt: string;
  lastHeartbeatAt: string;
  lastErrorCode: string;
  fingerprint?: string;
  coreId?: string;
  providerChangePending?: boolean;
  successorCoreId?: string;
  previousCoreId?: string;
  providerChangeId?: string;
}

export interface DeviceMeshBootstrapRequest {
  cloudBaseUrl: string;
  bootstrapTicket: string;
  fingerprint?: string;
  coreId?: string;
}

export interface MeshPairingInput {
  offerToken?: string;
  setupCode?: string;
  fingerprint?: string;
  coreId?: string;
}

export interface DeviceMeshBootstrapResponse {
  ok: boolean;
  credentialId: string;
  deviceId: string;
  runtimeId: string;
  expiresAt: string;
}

export interface CloudPairingDeviceIdentity {
  deviceId: string;
  runtimeId: string;
  platform: string;
  label?: string;
}

export interface CloudBootstrapTicketResponse {
  ticketId: string;
  ticket: string;
  spaceId: string;
  deviceId: string;
  runtimeId: string;
  expiresAt: string;
  ttlSeconds: number;
}


export interface CloudPairingStatusResponse {
  spaceId: string;
  trustedDeviceCount: number;
  firstDeviceSetupRequired: boolean;
}

export interface CloudPairingClaimRequest extends CloudPairingDeviceIdentity {
  offerToken?: string;
  setupCode?: string;
  proof?: DeviceIdentityProof;
}

export interface DeviceIdentityProof {
  publicKey: string;
  audience: string;
  method: string;
  path: string;
  bodyHash: string;
  issuedAt: number;
  nonce: string;
  signature: string;
}

export interface CloudPairingOfferResponse {
	qrImage?: string;
  offerId: string;
  offerToken: string;
  qrPayload: string;
  expiresAt: string;
}

export interface CloudDeviceRuntimeInfo {
  runtimeId: string;
  presence: string;
  runtimeSessionId?: string;
  connectionGeneration?: number;
}

export interface CloudDeviceInfo {
  deviceId: string;
  platform: string;
  label: string;
  trustState: string;
  presence?: string;
  lastHeartbeat?: string;
  runtimes: CloudDeviceRuntimeInfo[];
}

export interface CloudDeviceListResponse {
  devices: CloudDeviceInfo[];
}

export interface CloudProbeResponse {
  reachable: boolean;
  latencyMs: number;
  runtimeSessionId?: string;
  connectionGeneration?: number;
}
