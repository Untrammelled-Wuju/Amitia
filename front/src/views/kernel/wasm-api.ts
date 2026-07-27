import { apiClient } from "@/composables/useApi";

const BASE = "/api/wasm";

export interface WasmRuntimeDefinition {
  runtime_definition_id: string;
  module_id: string;
  extension_id: string;
  module_path: string;
  module_hash: string;
  module_sha256: string;
  engine_type: string;
  abi: string;
  abi_def?: {
    version: string;
    alloc_export: string;
    dealloc_export: string;
    invoke_export: string;
  };
  wasi_version: string;
  entry_export: string;
  allowed_imports: string[];
  memory_limit_bytes: number;
  fuel_limit: number;
  instance_policy: string;
  deterministic: boolean;
  max_output_bytes: number;
  max_host_calls: number;
  call_timeout: number;
  definition_hash: string;
  definition_version: number;
  version: string;
  generation: number;
  imports?: { module_name: string; function_name: string }[];
  exports?: { name: string; kind: string }[];
  permission_requirements?: string[];
  scope_rule?: string;
}

export interface WasmModule {
  module_id: string;
  path: string;
  hash: string;
  size: number;
  valid: boolean;
}

export interface WasmInstance {
  instance_id: string;
  identity: {
    instance_id: string;
    module_id: string;
    extension_id: string;
    state: string;
  };
  stats: {
    instance_id: string;
    module_id: string;
    state: string;
    invocations: number;
    traps: number;
    timeouts: number;
    last_error: string;
    last_used_at: string | null;
    memory_used: number;
    fuel_used: number;
  };
}

export interface InvokeResult {
  output: unknown;
  duration: string;
  fuel_used: number;
  cached: boolean;
}

export interface ValidationReport {
  valid: boolean;
  module_hash: string;
  module_size: number;
  exports: string[];
  imports: string[];
  errors: string[];
  warnings: string[];
}

export async function listDefinitions(extensionId?: string): Promise<WasmRuntimeDefinition[]> {
  const params = extensionId ? `?extension_id=${encodeURIComponent(extensionId)}` : "";
  const res = await apiClient.get(`${BASE}/definitions${params}`);
  return res.data;
}

export async function getDefinition(id: string): Promise<WasmRuntimeDefinition> {
  const res = await apiClient.get(`${BASE}/definitions/${encodeURIComponent(id)}`);
  return res.data;
}

export async function createDefinition(def: Partial<WasmRuntimeDefinition>): Promise<WasmRuntimeDefinition> {
  const res = await apiClient.post(`${BASE}/definitions`, def);
  return res.data;
}

export async function deleteDefinition(id: string): Promise<{ deleted: boolean }> {
  const res = await apiClient.delete(`${BASE}/definitions/${encodeURIComponent(id)}`);
  return res.data;
}

export async function listModules(): Promise<WasmModule[]> {
  const res = await apiClient.get(`${BASE}/modules`);
  return res.data;
}

export async function uploadModule(moduleId: string, file: File): Promise<WasmModule> {
  const formData = new FormData();
  formData.append("module_id", moduleId);
  formData.append("module", file);
  const res = await apiClient.post(`${BASE}/modules`, formData);
  return res.data;
}

export async function getModule(moduleId: string): Promise<WasmModule> {
  const res = await apiClient.get(`${BASE}/modules/${encodeURIComponent(moduleId)}`);
  return res.data;
}

export async function deleteModule(moduleId: string): Promise<{ unloaded: boolean }> {
  const res = await apiClient.delete(`${BASE}/modules/${encodeURIComponent(moduleId)}`);
  return res.data;
}

export async function invokeModule(moduleId: string, input: unknown): Promise<InvokeResult> {
  const res = await apiClient.post(`${BASE}/invoke`, {
    module_id: moduleId,
    input,
  });
  return res.data;
}

export async function listInstances(): Promise<WasmInstance[]> {
  const res = await apiClient.get(`${BASE}/instances`);
  return res.data;
}

export async function validateModule(file: File): Promise<ValidationReport> {
  const formData = new FormData();
  formData.append("module", file);
  const res = await apiClient.post(`${BASE}/validate`, formData);
  return res.data;
}
