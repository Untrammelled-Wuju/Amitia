export const PROTOCOL_NAME = 'amitia-game-host';
export const PROTOCOL_MAJOR = 1;
export const PROTOCOL_VERSION = 'amitia-game-host/1';

export type MessageType = 'request' | 'response' | 'notification' | 'error';

export const MessageType = {
  REQUEST: 'request' as MessageType,
  RESPONSE: 'response' as MessageType,
  NOTIFICATION: 'notification' as MessageType,
  ERROR: 'error' as MessageType,
};

export interface ProtocolError {
  code: string;
  message: string;
  retryable?: boolean;
  data?: unknown;
}

export interface Envelope {
  protocol: string;
  type: MessageType;
  id?: string;
  requestId?: string;
  method?: string;
  runtimeId?: string;
  pluginId?: string;
  serviceId?: string;
  generation?: number;
  payload?: unknown;
  error?: ProtocolError;
  metadata?: Record<string, unknown>;
}

export type ServiceKind = 'process';

export const ServiceKind = {
  PROCESS: 'process' as ServiceKind,
};

export interface ServiceDescriptor {
  id: string;
  name?: string;
  kind: ServiceKind;
  required?: boolean;
  dependsOn?: string[];
  capabilities?: string[];
  metadata?: Record<string, unknown>;
}

export type ChannelKind = 'event' | 'state' | 'log' | 'metric' | 'custom' | 'binary';

export const ChannelKind = {
  EVENT: 'event' as ChannelKind,
  STATE: 'state' as ChannelKind,
  LOG: 'log' as ChannelKind,
  METRIC: 'metric' as ChannelKind,
  CUSTOM: 'custom' as ChannelKind,
  BINARY: 'binary' as ChannelKind,
};

export type ChannelDirection = 'plugin_to_host' | 'host_to_plugin' | 'bidirectional';

export const ChannelDirection = {
  PLUGIN_TO_HOST: 'plugin_to_host' as ChannelDirection,
  HOST_TO_PLUGIN: 'host_to_plugin' as ChannelDirection,
  BIDIRECTIONAL: 'bidirectional' as ChannelDirection,
};

export type FrequencyHint = 'low' | 'normal' | 'high' | 'realtime';

export const FrequencyHint = {
  LOW: 'low' as FrequencyHint,
  NORMAL: 'normal' as FrequencyHint,
  HIGH: 'high' as FrequencyHint,
  REALTIME: 'realtime' as FrequencyHint,
};

export interface ChannelDescriptor {
  id: string;
  kind: ChannelKind;
  schemaId?: string;
  direction?: ChannelDirection;
  frequencyHint?: FrequencyHint;
  metadata?: Record<string, unknown>;
}

export const HostFeature = {
  REALTIME_CONTROL: 'realtime_control',
  STATE_STREAMING: 'state_streaming',
  EVENT_STREAMING: 'event_streaming',
  CUSTOM_RPC: 'custom_rpc',
  HOST_API: 'host_api',
  SHARED_CONTROL: 'shared_control',
  MULTI_SERVICE: 'multi_service',
  BINARY_STREAMING: 'binary_streaming',
} as const;

export type HostFeature = typeof HostFeature[keyof typeof HostFeature];

export const ErrorCode = {
  INVALID_REQUEST: 'invalid_request',
  INVALID_ARGUMENT: 'invalid_argument',
  NOT_FOUND: 'not_found',
  ALREADY_EXISTS: 'already_exists',
  UNSUPPORTED: 'unsupported',
  PROTOCOL_MISMATCH: 'protocol_mismatch',
  CAPABILITY_UNSUPPORTED: 'capability_unsupported',
  RUNTIME_UNAVAILABLE: 'runtime_unavailable',
  SERVICE_UNAVAILABLE: 'service_unavailable',
  INVALID_RUNTIME_STATE: 'invalid_runtime_state',
  PERMISSION_DENIED: 'permission_denied',
  RESOURCE_EXHAUSTED: 'resource_exhausted',
  TIMEOUT: 'timeout',
  CANCELLED: 'cancelled',
  INTERNAL: 'internal',
};

export interface PluginSchema {
  services?: ServiceDescriptor[];
  channels?: ChannelDescriptor[];
  capabilities?: string[];
}

export type RequestHandler = (request: Envelope) => Promise<unknown>;
export type NotificationHandler = (notification: Envelope) => Promise<void>;
