import { readFile } from 'node:fs/promises';
import { basename } from 'node:path';

const DEFAULT_BASE_URL = 'http://127.0.0.1:18899/api';
const DEFAULT_OPERATION_TIMEOUT_MS = 120_000;

export interface GameCenterResponse<T = unknown> {
  code: number;
  msg: string;
  data?: T | null;
}

export interface GamePluginSummary {
  extensionId: string;
  pluginId: string;
  name: string;
  version: string;
  description?: string;
  enabled: boolean;
  installState: string;
  health: string;
  runtimeCount: number;
  managementTarget: string;
}

export interface GameRuntimeSummary {
  runtimeId: string;
  pluginId: string;
  extensionId: string;
  state: string;
  health: string;
  serviceCount: number;
  connected: boolean;
  ready: boolean;
  controlMode: string;
  authorityEpoch: number;
}

export interface GameRuntimeDetail {
  runtimeId: string;
  pluginId: string;
  extensionId: string;
  runtimeState: string;
  desiredState?: string;
  process?: {
    managed: boolean;
    running: boolean;
    processGeneration: number;
    restartCount: number;
  };
  connection?: {
    connected: boolean;
    protocolVersion?: string;
    peerGeneration?: number;
  };
  handshake?: {
    handshakeState: string;
    ready: boolean;
    protocol?: string;
  };
  controlAuthority?: {
    runtimeId: string;
    mode: string;
    epoch: number;
  };
}

export interface GamePluginList {
  items: GamePluginSummary[];
  total: number;
  page: number;
  pageSize: number;
}

export interface GameRuntimeList {
  items: GameRuntimeSummary[];
  total: number;
}

export interface ControlRequest {
  targetMode?: string;
  expectedEpoch?: number;
}

interface PackageImportPreview {
  sessionId: string;
  id: string;
  version: string;
  scopeType?: string;
  scopeId?: string;
  compatible?: boolean;
  managementTarget?: string;
  contributionKinds?: string[];
  capabilityConfirmations?: string[];
  currentVersion?: string;
  signature?: { status?: string };
  scripts?: number;
  highRiskCapabilities?: string[];
  upgradeDiff?: {
    signerChanged?: boolean;
    configMigrationRequired?: boolean;
  };
}

interface PackageArtifactPreviewResponse {
  artifactId: string;
  archiveHash: string;
  preview: PackageImportPreview;
}

interface PackageConfirmationResponse {
  confirmationToken: string;
  expiresAt?: string;
}

interface PackageOperationResult {
  operationId?: string;
  extensionId?: string;
  version?: string;
  status?: string;
  errorCode?: string;
  // Kernel uninstall currently serializes PackageOperationRecord directly.
  // Keep the E2E client compatible until that public response is normalized.
  OperationID?: string;
  ExtensionID?: string;
  TargetVersion?: string;
  Status?: string;
  ErrorCode?: string;
}

interface PackageUninstallPreview {
  extensionId: string;
  currentVersion: string;
  uninstallable: boolean;
  requiredConfirmations?: string[];
}

interface PackageUninstallConfirmation {
  confirmationToken: string;
  expiresAt?: string;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function apiErrorMessage(status: number, body: unknown): string {
  if (isRecord(body)) {
    for (const key of ['msg', 'message', 'error', 'detail', 'errorCode', 'code']) {
      const value = body[key];
      if (typeof value === 'string' && value.trim()) return `${value} (HTTP ${status})`;
      if (typeof value === 'number') return `${key}=${value} (HTTP ${status})`;
    }
  }
  return `HTTP ${status}`;
}

export class GameCenterClient {
  private readonly baseUrl: string;
  private readonly authToken: string;
  private readonly developerSessionId: string;

  constructor(
    baseUrl: string = process.env.GAMEHOST_BASE_URL || DEFAULT_BASE_URL,
    authToken: string = process.env.GAMEHOST_AUTH_TOKEN || '',
    developerSessionId: string = process.env.GAMEHOST_DEVELOPER_SESSION_ID || '',
  ) {
    this.baseUrl = baseUrl.replace(/\/$/, '');
    this.authToken = authToken.trim();
    this.developerSessionId = developerSessionId.trim();
  }

  private headers(extra?: HeadersInit): Headers {
    const headers = new Headers(extra);
    if (this.authToken) headers.set('Authorization', `Bearer ${this.authToken}`);
    return headers;
  }

  private async request<T>(path: string, init: RequestInit): Promise<T> {
    const response = await fetch(this.baseUrl + path, {
      ...init,
      headers: this.headers(init.headers),
    });

    const text = await response.text();
    let body: unknown = null;
    if (text.trim()) {
      try {
        body = JSON.parse(text) as unknown;
      } catch {
        throw new Error(`GameHost API returned non-JSON response (HTTP ${response.status})`);
      }
    }

    if (!response.ok) {
      throw new Error(`GameHost API error: ${apiErrorMessage(response.status, body)}`);
    }

    // Game Center endpoints use {code,msg,data}; canonical Extension Package
    // endpoints return raw JSON. Support both without inventing a second API.
    if (isRecord(body) && typeof body.code === 'number') {
      const code = body.code;
      if (code !== 0 && code !== 200) {
        throw new Error(`GameHost API error: ${apiErrorMessage(response.status, body)}`);
      }
      if (Object.prototype.hasOwnProperty.call(body, 'data')) {
        return body.data as T;
      }
    }

    return body as T;
  }

  async get<T>(path: string, params?: Record<string, string | number | boolean | undefined>): Promise<T> {
    const url = new URL(this.baseUrl + path);
    if (params) {
      for (const [key, value] of Object.entries(params)) {
        if (value !== undefined) url.searchParams.set(key, String(value));
      }
    }
    const relativePath = url.toString().slice(this.baseUrl.length);
    return this.request<T>(relativePath, { method: 'GET' });
  }

  async post<T = unknown>(path: string, body?: unknown): Promise<T> {
    const init: RequestInit = { method: 'POST' };
    if (body !== undefined) {
      init.headers = { 'Content-Type': 'application/json' };
      init.body = JSON.stringify(body);
    }
    return this.request<T>(path, init);
  }

  async del<T = unknown>(path: string): Promise<T> {
    return this.request<T>(path, { method: 'DELETE' });
  }

  async listPlugins(filter?: { page?: number; pageSize?: number; search?: string; status?: string }): Promise<GamePluginList> {
    return this.get('/game-center/plugins', filter as Record<string, string | number | undefined>);
  }

  async getPlugin(pluginId: string, extensionId: string): Promise<GamePluginSummary> {
    return this.get(`/game-center/plugins/${encodeURIComponent(pluginId)}`, { extensionId });
  }

  async listRuntimes(filter?: { page?: number; pageSize?: number; pluginId?: string; status?: string }): Promise<GameRuntimeList> {
    return this.get('/game-center/runtimes', filter as Record<string, string | number | undefined>);
  }

  async getRuntime(runtimeId: string, pluginId?: string): Promise<GameRuntimeDetail> {
    return this.get(`/game-center/runtimes/${encodeURIComponent(runtimeId)}`, pluginId ? { pluginId } : undefined);
  }

  async getRuntimeHealth(runtimeId: string): Promise<{ status: string }> {
    return this.get(`/game-center/runtimes/${encodeURIComponent(runtimeId)}/health`);
  }

  async getHandshakeStatus(runtimeId: string): Promise<{ handshakeState: string; ready: boolean }> {
    return this.get(`/game-center/runtimes/${encodeURIComponent(runtimeId)}/handshake`);
  }

  private buildInstallConfirmations(preview: PackageImportPreview): Record<string, boolean> {
    const confirmations: Record<string, boolean> = {};
    for (const key of preview.capabilityConfirmations ?? []) {
      const normalized = String(key).trim();
      if (normalized) confirmations[normalized] = true;
    }
    return confirmations;
  }

  private validateGamePreview(preview: PackageImportPreview, expectedExtensionId = ''): void {
    const kinds = new Set((preview.contributionKinds ?? []).map(String));
    if (preview.managementTarget !== 'game_center' && !kinds.has('game_plugin')) {
      throw new Error('package preview is not a GameHost/game_plugin extension');
    }
    if (preview.compatible === false) {
      throw new Error('package preview is not compatible/installable on this host');
    }
    if (expectedExtensionId && preview.id !== expectedExtensionId) {
      throw new Error(`update extensionId mismatch: expected ${expectedExtensionId}, got ${preview.id}`);
    }
  }

  private async previewPackage(archivePath: string, expectedExtensionId = ''): Promise<PackageImportPreview> {
    const bytes = await readFile(archivePath);
    const form = new FormData();
    form.set('scopeType', 'global');
    form.set('scopeId', '');
    if (this.developerSessionId) {
      form.set('allowUnsignedDev', 'true');
      form.set('developerSessionId', this.developerSessionId);
    }
    form.set('file', new Blob([new Uint8Array(bytes)]), basename(archivePath));

    const response = await this.request<PackageArtifactPreviewResponse>('/extensions/packages/artifacts', {
      method: 'POST',
      body: form,
    });
    if (!response?.preview?.sessionId) {
      throw new Error('canonical Extension Package API did not return a preview session');
    }
    this.validateGamePreview(response.preview, expectedExtensionId);
    return response.preview;
  }

  private operationId(result?: PackageOperationResult): string {
    return String(result?.operationId || result?.OperationID || '').trim();
  }

  private async waitForPackageOperation(operationId?: string, timeoutMs = DEFAULT_OPERATION_TIMEOUT_MS): Promise<void> {
    if (!operationId) return;
    const deadline = Date.now() + timeoutMs;
    while (Date.now() < deadline) {
      const operation = await this.get<PackageOperationResult>(`/extensions/packages/operations/${encodeURIComponent(operationId)}`);
      const status = String(operation?.status ?? '').toLowerCase();
      if (status === 'completed') return;
      if (status === 'failed' || status === 'requires_recovery' || status === 'cancelled' || status === 'rolled_back') {
        throw new Error(`package operation ${operationId} ended in ${status}: ${operation?.errorCode || 'unknown error'}`);
      }
      await new Promise(resolve => setTimeout(resolve, 250));
    }
    throw new Error(`package operation ${operationId} did not complete within ${timeoutMs}ms`);
  }

  private async commitPackage(preview: PackageImportPreview, expectedExtensionId = ''): Promise<PackageOperationResult> {
    this.validateGamePreview(preview, expectedExtensionId);
    const confirmation = await this.post<PackageConfirmationResponse>(
      `/extensions/packages/previews/${encodeURIComponent(preview.sessionId)}/confirm`,
      {
        scopeType: preview.scopeType || 'global',
        scopeId: preview.scopeId || '',
        confirmations: this.buildInstallConfirmations(preview),
      },
    );
    if (!confirmation?.confirmationToken) throw new Error('package confirmation token is missing');

    const isUpdate = Boolean(expectedExtensionId || preview.currentVersion);
    const result = await this.post<PackageOperationResult>(
      isUpdate ? '/extensions/packages/operations/update' : '/extensions/packages/operations/install',
      {
        sessionId: preview.sessionId,
        scopeType: preview.scopeType || 'global',
        scopeId: preview.scopeId || '',
        confirmationToken: confirmation.confirmationToken,
        ...(isUpdate ? { expectedExtensionId: expectedExtensionId || preview.id } : {}),
        idempotencyKey: `gamehost-e2e-${isUpdate ? 'update' : 'install'}-${Date.now()}-${Math.random().toString(16).slice(2)}`,
      },
    );
    await this.waitForPackageOperation(this.operationId(result));
    return result;
  }

  async installPlugin(archivePath: string): Promise<PackageOperationResult> {
    const preview = await this.previewPackage(archivePath);
    return this.commitPackage(preview);
  }

  async updatePlugin(extensionId: string, archivePath: string): Promise<PackageOperationResult> {
    const preview = await this.previewPackage(archivePath, extensionId);
    return this.commitPackage(preview, extensionId);
  }

  async enablePlugin(extensionId: string): Promise<unknown> {
    return this.post(`/game-center/extensions/${encodeURIComponent(extensionId)}/enable`);
  }

  async disablePlugin(extensionId: string): Promise<unknown> {
    return this.post(`/game-center/extensions/${encodeURIComponent(extensionId)}/disable`);
  }

  async uninstallPlugin(extensionId: string): Promise<PackageOperationResult> {
    const preview = await this.post<PackageUninstallPreview>('/extensions/kernel/extensions/uninstall/preview', {
      extensionId,
      scopeType: 'global',
      scopeId: '',
    });
    if (preview?.uninstallable === false) {
      throw new Error(`extension ${extensionId} is not uninstallable`);
    }
    const confirmations: Record<string, boolean> = {};
    for (const key of preview?.requiredConfirmations ?? []) {
      const normalized = String(key).trim();
      if (normalized) confirmations[normalized] = true;
    }
    const confirmed = await this.post<PackageUninstallConfirmation>('/extensions/kernel/extensions/uninstall/confirm', {
      extensionId,
      scopeType: 'global',
      scopeId: '',
      confirmations,
    });
    if (!confirmed?.confirmationToken) throw new Error('uninstall confirmation token is missing');
    const result = await this.post<PackageOperationResult>('/extensions/kernel/extensions/uninstall', {
      extensionId,
      scopeType: 'global',
      scopeId: '',
      confirmationToken: confirmed.confirmationToken,
    });
    await this.waitForPackageOperation(this.operationId(result));
    return result;
  }

  async startRuntime(runtimeId: string): Promise<unknown> {
    return this.post(`/game-center/runtimes/${encodeURIComponent(runtimeId)}/start`);
  }

  async stopRuntime(runtimeId: string): Promise<unknown> {
    return this.post(`/game-center/runtimes/${encodeURIComponent(runtimeId)}/stop`);
  }

  async restartRuntime(runtimeId: string): Promise<unknown> {
    return this.post(`/game-center/runtimes/${encodeURIComponent(runtimeId)}/restart`);
  }

  async takeover(runtimeId: string, body?: ControlRequest): Promise<unknown> {
    return this.post(`/game-center/runtimes/${encodeURIComponent(runtimeId)}/takeover`, body);
  }

  async release(runtimeId: string, body?: ControlRequest): Promise<unknown> {
    return this.post(`/game-center/runtimes/${encodeURIComponent(runtimeId)}/release`, body);
  }

  async emergencyStop(runtimeId: string): Promise<unknown> {
    return this.post(`/game-center/runtimes/${encodeURIComponent(runtimeId)}/emergency-stop`);
  }

  async rearm(runtimeId: string): Promise<unknown> {
    return this.post(`/game-center/runtimes/${encodeURIComponent(runtimeId)}/rearm`);
  }

  async bindAgentContext(runtimeId: string, body: { serviceId: string; userId?: string; characterId?: string; conversationId?: string; channel?: string; sessionId?: string }): Promise<unknown> {
    return this.post(`/game-center/runtimes/${encodeURIComponent(runtimeId)}/agent-context`, body);
  }

  async invokeCanonicalTool<T = unknown>(body: { toolId: string; input?: unknown; userId?: string; characterId?: string; conversationId?: string; channel?: string; sessionId?: string; requestId?: string; toolCallId?: string }): Promise<T> {
    return this.post<T>('/game-center-debug/tools/invoke', body);
  }

  async invokeRuntimeRPC<T = unknown>(runtimeId: string, method: string, payload: unknown = {}, timeoutMs = 30_000, serviceId = process.env.MOCK_PLUGIN_SERVICE_ID || 'mock-game-runtime'): Promise<T> {
    const response = await this.post<{ code: number; msg: string; payload?: T }>(
      `/game-center/runtimes/${encodeURIComponent(runtimeId)}/services/${encodeURIComponent(serviceId)}/rpc`,
      { method, payload, timeoutMs },
    );
    if (!response || response.code !== 200) {
      throw new Error(`RPC ${method} failed: ${response?.code ?? 'unknown'} ${response?.msg ?? ''}`.trim());
    }
    if (response.payload === undefined) throw new Error(`RPC ${method} returned no payload`);
    return response.payload;
  }
}

export function createClient(baseUrl?: string): GameCenterClient {
  return new GameCenterClient(baseUrl);
}
