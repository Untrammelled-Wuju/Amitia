export interface MigrationDefinition {
  migration_id: string;
  extension_id: string;
  module_id: string;
  from_version_range: string;
  to_version: string;
  entry: string;
  runtime_type: string;
  direction: string;
  data_domains: string[];
  idempotency: string;
  reversibility: string;
  definition_hash: string;
  created_at: string;
}

export interface MigrationPlanInput {
  extension_id: string;
  from_version: string;
  to_version: string;
  from_definition_hash?: string;
  to_definition_hash?: string;
}

export interface MigrationPlanOutput {
  path: string[];
  estimated_risk: string;
  has_irreversible: boolean;
  requires_user_confirm: boolean;
  reversibility: string;
}

export interface MigrationOperation {
  operation_id: string;
  extension_id: string;
  from_version: string;
  to_version: string;
  status: string;
  current_step: number;
  reversibility: string;
  requires_user_confirm: boolean;
  user_confirmed: boolean;
  started_at: string;
  finished_at?: string;
  error_code?: string;
  error_message?: string;
}

export interface RollbackPlan {
  rollback_id: string;
  operation_id: string;
  extension_id: string;
  from_generation: number;
  to_generation: number;
  level: string;
  status: string;
  automatic: boolean;
  requires_user_action: boolean;
  started_at?: string;
  finished_at?: string;
  error_code?: string;
  error_message?: string;
}

export interface RollbackStepRecord {
  step_id: string;
  rollback_id: string;
  step_type: string;
  status: string;
  started_at: string;
  finished_at?: string;
  error_code?: string;
  error_message?: string;
}

export interface LifecycleJournalEntry {
  entry_id: string;
  operation_id: string;
  step_id: string;
  step_type: string;
  status: string;
  input_hash?: string;
  output_hash?: string;
  started_at: string;
  finished_at?: string;
  error_code?: string;
  error_message?: string;
}

export interface RecoveryAction {
  operation_id: string;
  strategy: string;
  detail: string;
}

export interface CanaryPolicy {
  policy_id: string;
  extension_id: string;
  mode: string;
  max_duration_secs: number;
  created_at: string;
  updated_at: string;
}

export interface CanaryState {
  canary_id: string;
  extension_id: string;
  policy_id: string;
  current_stage: string;
  status: string;
  started_at?: string;
  finished_at?: string;
  abort_reason?: string;
  generation_from: number;
  generation_to: number;
}

export interface CanaryMetric {
  metric_id: string;
  extension_id: string;
  generation: number;
  metric_name: string;
  value: number;
  status: string;
  recorded_at: string;
  cohort_type?: string;
  cohort_id?: string;
}

export interface HealthCheck {
  name: string;
  passed: boolean;
  details: string;
}

export interface HealthEvaluation {
  should_abort: boolean;
  abort_reason: string;
  checks: HealthCheck[];
}

export interface GenerationRoute {
  route_id: string;
  extension_id: string;
  cohort_type: string;
  cohort_id: string;
  target_generation: number;
  routing_reason: string;
  created_at: string;
}
