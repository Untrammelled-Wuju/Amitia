export type DeploymentMode = "local" | "cloud";

export interface DeploymentModeConfig {
  mode: DeploymentMode;
  serverURL?: string;
}

export type RuntimeState =
  | "not-installed"
  | "starting"
  | "not-ready"
  | "ready"
  | "failed";

export interface RuntimeEndpointStatus {
  state: RuntimeState;
  baseURL: string;
  profile?: "local" | "device-agent";
  message?: string;
}

export interface RuntimeStatus {
  state: RuntimeState;
  mode: DeploymentMode;
  message?: string;
  updatedAt: string;

  businessCore?: RuntimeEndpointStatus;
  localRuntime?: RuntimeEndpointStatus;
}

export interface DesktopEnvironment {
  platform: string;
  arch: string;
  version: string;
  isPackaged: boolean;
}

export interface DesktopClientError {
  message: string;
  stack: string;
  source: string;
  at: string;
}

export interface AgentSkillDirectorySelection {
  rootName: string;
  files: Array<{ path: string; name: string; base64: string }>;
}

export interface WorkspaceDirectorySelection {
  path: string;
  name: string;
}

export interface ExtensionPackageSelection {
  name: string;
  size: number;
  base64: string;
}

export interface SaveExtensionPackageRequest {
  suggestedName: string;
  base64: string;
}

export interface SaveConversationAttachmentRequest {
  url: string;
  suggestedName: string;
  headers?: Record<string, string>;
}

export interface LocalVoiceASRFinalEvent {
  eventId: string;
  transcript: string;
  sessionId?: string;
  conversationId?: string;
  characterId?: string;
  visualContext?: string;
  visualSource?: "camera" | "screen";
  occurredAt?: string;
}

export interface RuntimeEndpoint {
  schemaVersion: number;
  version?: string;
  instanceId?: string;
  pid: number;
  startedAt: string;
  host: string;
  port: number;
  httpBaseURL: string;
  websocketBaseURL: string;
  compatibilityBaseURL?: string;
}

export interface AmitiaDesktopAPI {
  getEnvironment(): Promise<DesktopEnvironment>;
  getDeploymentConfig(): Promise<DeploymentModeConfig>;
  getRuntimeEndpoint(): Promise<RuntimeEndpoint | null>;
  saveDeploymentConfig(
    config: DeploymentModeConfig,
  ): Promise<DeploymentModeConfig>;
  getRuntimeStatus(): Promise<RuntimeStatus>;
  openLogsDirectory(): Promise<void>;
  selectAgentSkillDirectory(): Promise<AgentSkillDirectorySelection | null>;
  selectMCPRoot(): Promise<{ path: string; name: string } | null>;
  selectWorkspaceDirectory(): Promise<WorkspaceDirectorySelection | null>;
  getPathForFile(file: File): string;
  openPath(path: string): Promise<void>;
  selectExtensionPackage(): Promise<ExtensionPackageSelection | null>;
  saveExtensionPackage(
    request: SaveExtensionPackageRequest,
  ): Promise<{ saved: boolean; fileName?: string }>;
  saveConversationAttachment(
    request: SaveConversationAttachmentRequest,
  ): Promise<{ saved: boolean; fileName?: string }>;
  minimizeWindow(): Promise<void>;
  toggleMaximizeWindow(): Promise<boolean>;
  closeWindow(): Promise<void>;
  quitApp(): Promise<void>;
  zoomIn(): Promise<void>;
  zoomOut(): Promise<void>;
  zoomReset(): Promise<void>;
  getZoomFactor(): Promise<number>;
  writeClipboardText(text: string): Promise<void>;
  readClipboardText(): Promise<string>;
  notifyDesktopPetChatState(payload: {
    state:
      | "assistant_listening"
      | "assistant_thinking"
      | "assistant_speaking"
      | "assistant_finished"
      | "assistant_error";
    roundId?: string;
    error?: string;
  }): void;
  getAutoLaunch(): Promise<boolean>;
  setAutoLaunch(enabled: boolean): Promise<boolean>;
  onAutoLaunchChanged(callback: (enabled: boolean) => void): () => void;
  onRuntimeStatusChanged(callback: (status: RuntimeStatus) => void): () => void;
  getDataDir(): Promise<string>;
  getVersion(): Promise<string>;
  checkUpdate(): Promise<unknown>;
  checkNow(): Promise<unknown>;
  downloadUpdate(): Promise<void>;
  startDownload(): Promise<void>;
  skipVersion(): Promise<void>;
  restartNow(): Promise<void>;
  restartLater(): Promise<void>;
  installUpdateNow(): Promise<void>;
  cancelAndEnter(): Promise<void>;
  getCurrentVersion(): Promise<string>;
  getPendingUpdate(): Promise<{
    version: string;
    releaseDate?: string;
    releaseNotes?: unknown;
    currentVersion: string;
  } | null>;
  getUpdateChannel(): Promise<{ allowBetaChannel: boolean }>;
  setUpdateChannel(
    allowBetaChannel: boolean,
  ): Promise<{ allowBetaChannel: boolean }>;
  openGiteeRelease(): Promise<void>;
  getReleaseNotes(): Promise<string>;
  onUpdateChecking(callback: () => void): () => void;
  onUpdateAvailable(
    callback: (event: unknown, data: unknown) => void,
  ): () => void;
  onUpdateNotAvailable(callback: () => void): () => void;
  onUpdateDownloadProgress(
    callback: (event: unknown, data: unknown) => void,
  ): () => void;
  onUpdateDownloaded(
    callback: (event: unknown, data: unknown) => void,
  ): () => void;
  onUpdateError(callback: (event: unknown, data: unknown) => void): () => void;
  getBackendAuthHeaders(target?: "local" | "business"): Promise<Record<string, string>>;
  getClientDiagnostics?(): Promise<DesktopClientError[]>;
  clearClientDiagnostics?(): Promise<void>;
  publishLocalVoiceASRFinal(event: LocalVoiceASRFinalEvent): Promise<{ accepted: boolean; eventId: string; eventType: string }>;
  getMeshIdentity(): Promise<{ deviceId: string; runtimeId: string; platform: string } | null>;
  getMeshStatus(): Promise<{ state: string; cloudBaseUrl: string; deviceId: string; runtimeId: string; runtimeSessionId: string; fingerprint?: string; coreId?: string; providerChangePending?: boolean; successorCoreId?: string; previousCoreId?: string; providerChangeId?: string } | null>;
  getMeshPairingStatus(cloudBaseUrl: string): Promise<{ spaceId: string; trustedDeviceCount: number; firstDeviceSetupRequired: boolean }>;
  createMeshPairingOffer(cloudBaseUrl: string, ttlSeconds?: number): Promise<{ offerId: string; offerToken: string; qrPayload: string; expiresAt: string }>;
  provisionMesh(cloudBaseUrl: string, pairing?: { offerToken?: string; setupCode?: string; fingerprint?: string; coreId?: string }): Promise<{ ok: boolean }>;
  deprovisionMesh(): Promise<{ ok: boolean }>;
  onUINavigate(callback: (target: string) => void): () => void;
  onUIStyleCommand(callback: (data: string) => void): () => void;
}

export interface RuntimeConnection {
  apiBaseURL: string;
  websocketBaseURL: string;
}
