export const GAME_PROTOCOL_VERSION = 'amitia-game/2' as const;

export const GAME_METHODS = {
  sessionOpen: 'game.session.open',
  sessionClose: 'game.session.close',
  sessionSnapshot: 'game.session.snapshot',
  observationGet: 'game.observation.get',
  actionExecute: 'game.action.execute',
  goalSet: 'game.goal.set',
  capabilitiesGet: 'game.capabilities.get'
} as const;

export type GameSessionStatus = 'created' | 'connecting' | 'ready' | 'paused' | 'closed' | 'failed';
export type GameActionStatus = 'succeeded' | 'failed' | 'cancelled' | 'rejected';

export interface GameSession {
  id: string;
  gameId: string;
  gameVersion?: string;
  edition?: string;
  status: GameSessionStatus;
  characterId?: string;
  worldId?: string;
  connectionMode?: string;
  startedAt?: string;
  updatedAt?: string;
  metadata?: Record<string, unknown>;
  payload?: unknown;
}

export interface GameLocation {
  world?: string;
  region?: string;
  coordinates?: Record<string, number>;
  payload?: unknown;
}

export interface GameEntity {
  id: string;
  type: string;
  name?: string;
  location?: GameLocation;
  attributes?: Record<string, unknown>;
  payload?: unknown;
}

export interface GameInventoryItem {
  id: string;
  type: string;
  name?: string;
  quantity?: number;
  slot?: string;
  attributes?: Record<string, unknown>;
  payload?: unknown;
}

export interface GameInventory {
  items?: GameInventoryItem[];
  capacity?: number;
  payload?: unknown;
}

export interface GameObservation {
  sessionId: string;
  sequence: number;
  observedAt: string;
  location?: GameLocation;
  entities?: GameEntity[];
  inventory?: GameInventory;
  state?: Record<string, unknown>;
  payload?: unknown;
}

export interface GameEvent {
  id: string;
  sessionId: string;
  type: string;
  occurredAt: string;
  entityIds?: string[];
  payload?: unknown;
}

export interface GameAction {
  id: string;
  sessionId: string;
  type: string;
  parameters?: unknown;
  idempotencyKey?: string;
  deadlineMs?: number;
}

export interface GameActionResult {
  actionId: string;
  sessionId: string;
  status: GameActionStatus;
  observation?: GameObservation;
  output?: unknown;
  errorCode?: string;
  errorMessage?: string;
  retryable?: boolean;
}

export interface GameGoal {
  id: string;
  sessionId: string;
  description: string;
  priority?: number;
  state?: string;
  constraints?: unknown;
  payload?: unknown;
}

export interface GameCapability {
  id: string;
  kind: string;
  description?: string;
  schema?: unknown;
  metadata?: Record<string, unknown>;
}

export interface GameCompanionArtifact {
  id: string;
  type: string;
  platforms?: string[];
  gameVersions?: string[];
  source: string;
  installTarget?: string;
  required?: boolean;
  sha256?: string;
}

export interface GamePluginSpec {
  protocolVersion: string;
  gameProtocolVersion?: string;
  runtimeModuleId: string;
  gameId?: string;
  gameFamily?: string;
  editions?: string[];
  supportedVersions?: string[];
  connectionModes?: string[];
  services?: string[];
  actions?: GameCapability[];
  observations?: GameCapability[];
  companionArtifacts?: GameCompanionArtifact[];
}
