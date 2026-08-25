const DEFAULT_BASE_URL = 'http://127.0.0.1:18899/api';

export interface GameCenterResponse<T = unknown> {
  code: number;
  msg: string;
  data: T | null;
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

export interface InstallPluginRequest {
  archivePath: string;
}

export interface ControlRequest {
  targetMode?: string;
  expectedEpoch?: number;
}

export class GameCenterClient {
  private baseUrl: string;

  constructor(baseUrl: string = DEFAULT_BASE_URL) {
    this.baseUrl = baseUrl.replace(/\/$/, '');
  }

  async get<T>(path: string, params?: Record<string, string | number | boolean | undefined>): Promise<T> {
    const url = new URL(this.baseUrl + path);
    if (params) {
      for (const [key, value] of Object.entries(params)) {
        if (value !== undefined) {
          url.searchParams.set(key, String(value));
        }
      }
    }
    const resp = await fetch(url.toString(), { method: 'GET' });
    const body = (await resp.json()) as GameCenterResponse<T>;
    if (body.code !== 0 && body.code !== 200) {
      throw new Error(`GameCenter API error: ${body.msg || resp.statusText} (code=${body.code})`);
    }
    return body.data as T;
  }

  async post<T = unknown>(path: string, body?: unknown): Promise<T> {
    const url = this.baseUrl + path;
    const init: RequestInit = { method: 'POST' };
    if (body !== undefined) {
      init.headers = { 'Content-Type': 'application/json' };
      init.body = JSON.stringify(body);
    }
    const resp = await fetch(url, init);
    const respBody = (await resp.json()) as GameCenterResponse<T>;
    if (respBody.code !== 0 && respBody.code !== 200) {
      throw new Error(`GameCenter API error: ${respBody.msg || resp.statusText} (code=${respBody.code})`);
    }
    return respBody.data as T;
  }

  async del<T = unknown>(path: string): Promise<T> {
    const resp = await fetch(this.baseUrl + path, { method: 'DELETE' });
    const body = (await resp.json()) as GameCenterResponse<T>;
    if (body.code !== 0 && body.code !== 200) {
      throw new Error(`GameCenter API error: ${body.msg || resp.statusText} (code=${body.code})`);
    }
    return body.data as T;
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

  async installPlugin(archivePath: string): Promise<unknown> {
    return this.post('/game-center/plugins/install', { archivePath } satisfies InstallPluginRequest);
  }

  async enablePlugin(extensionId: string): Promise<unknown> {
    return this.post(`/game-center/plugins/${encodeURIComponent(extensionId)}/enable`);
  }

  async disablePlugin(extensionId: string): Promise<unknown> {
    return this.post(`/game-center/plugins/${encodeURIComponent(extensionId)}/disable`);
  }

  async uninstallPlugin(extensionId: string): Promise<unknown> {
    return this.del(`/game-center/plugins/${encodeURIComponent(extensionId)}`);
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

  async updatePlugin(extensionId: string, archivePath: string): Promise<unknown> {
    return this.post(`/game-center/plugins/${encodeURIComponent(extensionId)}/update`, { archivePath } satisfies InstallPluginRequest);
  }
}

export function createClient(baseUrl?: string): GameCenterClient {
  return new GameCenterClient(baseUrl);
}
