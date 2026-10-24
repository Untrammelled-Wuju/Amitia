package migration

func RuntimeModelBudgetMigration() Migration {
	return Migration{
		Version: "20261024002",
		Name:    "runtime_model_and_tool_budget_ledger",
		Up: func(step *Step) error {
			step.CreateTable(`CREATE TABLE IF NOT EXISTS runtime_model_usage (
    sequence INTEGER PRIMARY KEY AUTOINCREMENT,
    id TEXT NOT NULL,
    space_id TEXT NOT NULL,
    device_id TEXT,
    owner_id TEXT,
    role_id TEXT,
    request_id TEXT,
    model TEXT,
    provider TEXT,
    source TEXT,
    status TEXT,
    started_at TEXT,
    completed_at TEXT,
    input_tokens INTEGER,
    output_tokens INTEGER,
    cached_tokens INTEGER,
    total_tokens INTEGER,
    usage_reported NUMERIC,
    coordination_id TEXT DEFAULT '',
    parent_interaction_id TEXT DEFAULT '',
    assignment_id TEXT DEFAULT '',
    reserved_tokens INTEGER NOT NULL DEFAULT 0,
    reserved_cost_micro_usd INTEGER NOT NULL DEFAULT 0,
    budget_cost_limit_micro_usd INTEGER NOT NULL DEFAULT 0,
    budget_input_rate INTEGER NOT NULL DEFAULT 0,
    budget_output_rate INTEGER NOT NULL DEFAULT 0,
    budget_rate_version TEXT DEFAULT '',
    budget_overrun NUMERIC NOT NULL DEFAULT 0,
    review_evidence_ref TEXT NOT NULL DEFAULT '',
    review_actor_id TEXT NOT NULL DEFAULT '',
    review_recorded_at TEXT NOT NULL DEFAULT ''
)`)
			step.CreateTable(`CREATE TABLE IF NOT EXISTS runtime_usage_baselines (
    space_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    sequence INTEGER,
    updated_at TEXT,
    PRIMARY KEY (space_id, device_id)
)`)
			step.CreateTable(`CREATE TABLE IF NOT EXISTS runtime_owned_tool_calls (
    effect_id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    coordination_id TEXT NOT NULL,
    parent_interaction_id TEXT NOT NULL,
    owner_id TEXT NOT NULL,
    role_id TEXT NOT NULL,
    assignment_id TEXT NOT NULL,
    tool_name TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    review_evidence_ref TEXT NOT NULL DEFAULT '',
    review_actor_id TEXT NOT NULL DEFAULT '',
    review_recorded_at TEXT NOT NULL DEFAULT ''
)`)
			if err := step.AddColumn("runtime_model_usage", "coordination_id", "TEXT DEFAULT ''"); err != nil {
				return err
			}
			if err := step.AddColumn("runtime_model_usage", "parent_interaction_id", "TEXT DEFAULT ''"); err != nil {
				return err
			}
			if err := step.AddColumn("runtime_model_usage", "assignment_id", "TEXT DEFAULT ''"); err != nil {
				return err
			}
			if err := step.AddColumn("runtime_model_usage", "reserved_tokens", "INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
			if err := step.AddColumn("runtime_model_usage", "reserved_cost_micro_usd", "INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
			if err := step.AddColumn("runtime_model_usage", "budget_cost_limit_micro_usd", "INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
			if err := step.AddColumn("runtime_model_usage", "budget_input_rate", "INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
			if err := step.AddColumn("runtime_model_usage", "budget_output_rate", "INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
			if err := step.AddColumn("runtime_model_usage", "budget_rate_version", "TEXT DEFAULT ''"); err != nil {
				return err
			}
			if err := step.AddColumn("runtime_model_usage", "budget_overrun", "NUMERIC NOT NULL DEFAULT 0"); err != nil {
				return err
			}
			if err := step.AddColumn("runtime_model_usage", "review_evidence_ref", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := step.AddColumn("runtime_model_usage", "review_actor_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := step.AddColumn("runtime_model_usage", "review_recorded_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := step.AddColumn("runtime_owned_tool_calls", "review_evidence_ref", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := step.AddColumn("runtime_owned_tool_calls", "review_actor_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := step.AddColumn("runtime_owned_tool_calls", "review_recorded_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := step.CreateIndex("idx_runtime_model_usage_id", "runtime_model_usage", []string{"id"}, true); err != nil {
				return err
			}
			if err := step.CreateIndex("idx_model_usage_scope", "runtime_model_usage", []string{"space_id", "device_id"}, false); err != nil {
				return err
			}
			if err := step.CreateIndex("idx_model_usage_coord", "runtime_model_usage", []string{"space_id", "coordination_id"}, false); err != nil {
				return err
			}
			if err := step.CreateIndex("idx_runtime_model_usage_started_at", "runtime_model_usage", []string{"started_at"}, false); err != nil {
				return err
			}
			if err := step.CreateIndex("idx_owned_tool_coord", "runtime_owned_tool_calls", []string{"space_id", "coordination_id"}, false); err != nil {
				return err
			}
			return nil
		},
	}
}
