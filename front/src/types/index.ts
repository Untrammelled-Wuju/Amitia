// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
// API
export interface ApiResponse<T = unknown> {
  code: number
  message: string
  data?: T
  detail?: string
}

// Error codes
export const ERR = {
  INTERNAL: 10000, SERVICE_UNAVAILABLE: 10001, TIMEOUT: 10002,
  DB_ERROR: 10003, VALIDATION: 10004, NOT_FOUND: 10005,
  BAD_REQUEST: 10006, CONFLICT: 10007, RATE_LIMITED: 10008,
  UNAUTHORIZED: 20000, INVALID_CREDENTIALS: 20001, TOKEN_EXPIRED: 20002,
  TOKEN_INVALID: 20003, FORBIDDEN: 20004, AUTH_SETUP_REQUIRED: 20005, AUTH_ALREADY_SETUP: 20006,
  CONFIG_ERROR: 30000, CONFIG_NOT_FOUND: 30001, CONFIG_INVALID: 30002,
  CONFIG_SAVE_FAILED: 30003, MODEL_NOT_CONFIGURED: 30004,
  MODEL_ERROR: 40000, MODEL_CONNECTION_FAILED: 40001, MODEL_TIMEOUT: 40002,
  MODEL_UNAUTHORIZED: 40003, MODEL_NOT_FOUND: 40004, MODEL_RATE_LIMITED: 40005,
  MODEL_INVALID_RESPONSE: 40006, MODEL_BASE_URL_UNREACHABLE: 40007,
  MODEL_NETWORK_ERROR: 40008, MODEL_CONFIG_INCOMPLETE: 40009,
  MODEL_UNSUPPORTED_TYPE: 40010, MODEL_TEST_FAILED: 40011,
  WECHAT_ERROR: 50000, WECHAT_NOT_CONNECTED: 50001, WECHAT_ACCOUNT_NOT_FOUND: 50002,
  WECHAT_SEND_FAILED: 50003, WECHAT_WEBHOOK_INVALID: 50004,
  AGENT_ERROR: 60000, AGENT_MODEL_FAILED: 60001, AGENT_NO_CHARACTER: 60002,
  AGENT_CONV_NOT_FOUND: 60003, AGENT_SAFETY_BLOCKED: 60004,
  AGENT_CHANNEL_UNSUPPORTED: 60005, AGENT_EMPTY_MESSAGE: 60006,
  IMPORT_ERROR: 70000, IMPORT_PARSE_FAILED: 70001, IMPORT_BATCH_NOT_FOUND: 70002,
  IMPORT_FILE_TOO_LARGE: 70003, IMPORT_FORMAT_UNSUPPORTED: 70004,
  IMPORT_SENSITIVE_CONTENT: 70005, IMPORT_MEMORY_FAILED: 70006, IMPORT_SUMMARY_FAILED: 70007,
  STORAGE_ERROR: 80000, BACKUP_FAILED: 80001, BACKUP_NOT_FOUND: 80002,
  RESTORE_FAILED: 80003, EXPORT_FAILED: 80004, IMPORT_FAILED: 80005,
  DATA_DIR_NOT_WRITABLE: 80006, DISK_SPACE_INSUFFICIENT: 80007,
} as const

// Message
export interface Message {
  id: string
  conversationId: string
  role: "user" | "assistant" | "system"
  content: string
  imageUrl?: string
  videoUrl?: string
  audioUrl?: string
  msgType?: string
  tokens?: number
  source: string
  importedItemId?: string | null
  createdAt: string
}

// Conversation
export interface Conversation {
  id: string
  characterId: string
  title: string
  channel: string
  source: string
  peerId: string
  importBatchId?: string | null
  messageCount: number
  createdAt: string
  updatedAt: string
}

// Character
export interface TtsConfig {
  id: number
  name: string
  apiKey: string
  resourceId: string
  voiceType: string
  emotion: string
  speed: number
  pitch: number
  volume: number
  isActive: number
  isCustom: number
  customVoiceId: string
  lastTestResult?: string
  hasApiKey: boolean
  createdAt: string
  updatedAt: string
}

export interface VoicePreset {
  name: string
  label: string
  gender: string
  language: string
}

export interface Character {
  id: string
  name: string
  avatar: string
  identity: string
  personality: string
  speakingStyle: string
  relationshipStyle: string
  systemPrompt: string
  boundaryRules: string
  personalitySliders: string
  description: string
  basePrompt: string
  generatedPrompt: string
  isDefault: number
  status: string
  personalityConfig: string
  chatStyleConfig: string
  sceneRules: string
  isActive: number
  sortOrder: number
  conversationId: string
  createdAt: string
  updatedAt: string
  gender: string
  genderLabel?: string | null
  pronoun: string
  selfReference: string
  userAddressingStyle?: string | null
  voiceConfigId?: string
  voiceType?: string
  voiceSpeed?: number
  voicePitch?: number
  voiceVolume?: number
  customVoiceId?: string
}



// Import
export interface ImportResult {
  batchId: string
  conversationId: string
  messageCount: number
  title: string
}

// Memory
export interface Memory {
  id: string
  characterId: string
  memoryType: string
  key: string
  value: string
  importance: number
  confidence: number
  source: string
  scope: string
  verifiedStatus: string
  useCount: number
  lastUsedAt?: string | null
  expiresAt?: string | null
  createdAt: string
  updatedAt: string
}

// LLM Config
export interface LLMConfig {
  baseUrl: string
  apiKey: string
  modelName: string
  temperature: number
  maxTokens: number
  topP: number
}
export interface RuntimeModeResponse {
  deployMode: string
  host: string
  port: number
  web: { enabled: boolean; publicBaseUrl: string; requireAuth: boolean }
  bridge: { enabled: boolean; mode: string; host: string; port: number }
  storage: { dataDir: string }
}

export interface RuntimeModeValidationResult {
  valid: boolean
  errors: string[]
  warnings?: string[]
  checks?: Array<{
    name: string
    level: "info" | "warn" | "error"
    passed: boolean
    message: string
    suggestion?: string
  }>
}

export type DeployMode = "desktop-local" | "cloud-web"

