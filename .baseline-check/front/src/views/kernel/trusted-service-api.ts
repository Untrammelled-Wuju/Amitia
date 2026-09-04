import { apiClient } from "@/composables/useApi";

const BASE = "/api/extensions/services";

export interface TrustedServiceInstance {
  instance_id: string;
  service_id: string;
  state: string;
  pid?: number;
  generation: number;
  restart_count: number;
  health_fails: number;
  working_dir?: string;
  stdio_conn?: string;
  started_at?: string;
  stopped_at?: string;
  last_health_at?: string;
  name?: string;
  publisher?: string;
  trust_level?: string;
  protocol?: string;
  health_check_type?: string;
  platform?: string;
  executable_path?: string;
}

export interface ServiceRuntimeDefinition {
  service_id: string;
  extension_id: string;
  module_id: string;
  name: string;
  description: string;
  publisher: string;
  trust_level: string;
  executables: PlatformExecutable[];
  protocol: string;
  instance_policy: string;
  health_check: ServiceHealthCheck;
  recovery: ServiceRecoveryPolicy;
  shutdown: ServiceShutdownPolicy;
  limits: ServiceResourceLimits;
  network: ServiceNetworkPolicy;
  allowed_namespaces: string[];
  manifest_hash: string;
  definition_version: number;
  auto_start: boolean;
}

export interface PlatformExecutable {
  platform: string;
  path: string;
  sha256: string;
  entry?: string;
  args_template?: string[];
  signature: BinarySignature;
  env_template?: Record<string, string>;
}

export interface BinarySignature {
  algorithm: string;
  value: string;
  signer?: string;
  trusted: boolean;
}

export interface ServiceHealthCheck {
  type: string;
  interval: number;
  timeout: number;
  grace_period: number;
  max_consecutive_fails: number;
  endpoint?: string;
}

export interface ServiceRecoveryPolicy {
  max_restarts: number;
  restart_delay: number;
  backoff_multiplier: number;
  max_restart_delay: number;
  quarantine_on_fail: boolean;
}

export interface ServiceShutdownPolicy {
  grace_period: number;
  kill_timeout: number;
  cleanup_children: boolean;
  remove_temp_dir: boolean;
}

export interface ServiceResourceLimits {
  max_memory_mb: number;
  max_cpu_percent: number;
  max_file_descriptors: number;
  max_disk_mb: number;
  max_subprocesses: number;
}

export interface ServiceNetworkPolicy {
  allow_inbound: boolean;
  allow_outbound: boolean;
  allowed_domains?: string[];
  allowed_ports?: number[];
  loopback_only: boolean;
  require_proxy: boolean;
  audit_all: boolean;
}

export interface StartRequestBody {
  instance_id?: string;
  generation?: number;
  base_path?: string;
  working_dir?: string;
  session_token?: string;
  secret_lease?: string;
  log_level?: string;
  args?: Record<string, string>;
  trust_level?: string;
}

export interface StartResult {
  instance_id: string;
  pid: number;
  state: string;
  started_at: string;
  generation: number;
}

export interface StopRequestBody {
  reason?: string;
  force?: boolean;
}

export interface StopResult {
  service_id: string;
  state: string;
  stopped_at: string;
}

export interface HealthResult {
  service_id: string;
  status: string;
  details?: Record<string, unknown>;
}

export interface InvokeRequestBody {
  operation: string;
  input?: unknown;
  timeout?: string;
}

export interface InvokeResult {
  service_id: string;
  operation: string;
  output: unknown;
}

export interface QuarantineRecord {
  service_id: string;
  instance_id: string;
  reason: string;
  detail: string;
  evidence?: Record<string, unknown>;
  quarantined_at: string;
}

export interface QuarantineHistoryEntry {
  service_id: string;
  reason: string;
  quarantined_at: string;
  released_at?: string;
  release_reason?: string;
}

export interface QuarantineListResult {
  active: QuarantineRecord[];
  history: QuarantineHistoryEntry[];
}

export async function listServices(): Promise<{ services: TrustedServiceInstance[]; total: number }> {
  const res = await apiClient.get(BASE);
  return res.data;
}

export async function getService(serviceId: string): Promise<TrustedServiceInstance> {
  const res = await apiClient.get(`${BASE}/${encodeURIComponent(serviceId)}`);
  return res.data;
}

export async function registerService(def: Partial<ServiceRuntimeDefinition>): Promise<ServiceRuntimeDefinition> {
  const res = await apiClient.post(BASE, def);
  return res.data;
}

export async function unregisterService(serviceId: string): Promise<{ service_id: string; status: string }> {
  const res = await apiClient.delete(`${BASE}/${encodeURIComponent(serviceId)}`);
  return res.data;
}

export async function startService(serviceId: string, body: StartRequestBody): Promise<StartResult> {
  const res = await apiClient.post(`${BASE}/${encodeURIComponent(serviceId)}/start`, body);
  return res.data;
}

export async function stopService(serviceId: string, body: StopRequestBody): Promise<StopResult> {
  const res = await apiClient.post(`${BASE}/${encodeURIComponent(serviceId)}/stop`, body);
  return res.data;
}

export async function getServiceStatus(serviceId: string): Promise<TrustedServiceInstance> {
  const res = await apiClient.get(`${BASE}/${encodeURIComponent(serviceId)}/status`);
  return res.data;
}

export async function healthCheck(serviceId: string): Promise<HealthResult> {
  const res = await apiClient.get(`${BASE}/${encodeURIComponent(serviceId)}/health`);
  return res.data;
}

export async function invokeService(serviceId: string, body: InvokeRequestBody): Promise<InvokeResult> {
  const res = await apiClient.post(`${BASE}/${encodeURIComponent(serviceId)}/invoke`, body);
  return res.data;
}

export async function listQuarantined(): Promise<QuarantineListResult> {
  const res = await apiClient.get(`${BASE}/quarantine/list`);
  return res.data;
}

export async function releaseQuarantine(serviceId: string, reason?: string): Promise<{ service_id: string; status: string }> {
  const res = await apiClient.post(`${BASE}/quarantine/${encodeURIComponent(serviceId)}/release`, { reason: reason || "manual_release" });
  return res.data;
}
