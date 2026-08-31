package migration

func DefaultMigrations() []Migration {
	return []Migration{
		BackupMigration(),
		{
			Version: "001",
			Name:    "add_psyche_states_table",
			Up: func(s *Step) error {
				s.CreateTable("CREATE TABLE IF NOT EXISTS psyche_states (character_id TEXT PRIMARY KEY, version TEXT DEFAULT '', state_version INTEGER DEFAULT 0, emotion TEXT DEFAULT '{}', mood TEXT DEFAULT '{}', stress REAL DEFAULT 0, energy REAL DEFAULT 0.7, created_at TEXT DEFAULT '', updated_at TEXT DEFAULT '')")
				return nil
			},
		},
		{
			Version: "002",
			Name:    "add_psyche_events_table",
			Up: func(s *Step) error {
				s.CreateTable("CREATE TABLE IF NOT EXISTS psyche_events (id TEXT PRIMARY KEY, character_id TEXT NOT NULL DEFAULT '', event_type TEXT NOT NULL DEFAULT '', event_data TEXT DEFAULT '{}', created_at TEXT DEFAULT '')")
				return nil
			},
		},
		{
			Version: "003",
			Name:    "add_psyche_snapshots_table",
			Up: func(s *Step) error {
				s.CreateTable("CREATE TABLE IF NOT EXISTS psyche_snapshots (id TEXT PRIMARY KEY, character_id TEXT NOT NULL DEFAULT '', snapshot_data TEXT DEFAULT '{}', created_at TEXT DEFAULT '')")
				return nil
			},
		},
		{
			Version: "004",
			Name:    "add_relationship_states_table",
			Up: func(s *Step) error {
				s.CreateTable("CREATE TABLE IF NOT EXISTS relationship_states (id TEXT PRIMARY KEY, character_id TEXT NOT NULL DEFAULT '', relation_type TEXT NOT NULL DEFAULT '', relation_data TEXT DEFAULT '{}', created_at TEXT DEFAULT '', updated_at TEXT DEFAULT '')")
				return nil
			},
		},
		{
			Version: "005",
			Name:    "add_relationship_events_table",
			Up: func(s *Step) error {
				s.CreateTable("CREATE TABLE IF NOT EXISTS relationship_events (id TEXT PRIMARY KEY, character_id TEXT NOT NULL DEFAULT '', event_type TEXT NOT NULL DEFAULT '', event_data TEXT DEFAULT '{}', created_at TEXT DEFAULT '')")
				return nil
			},
		},
		MemoryScopeTypeMigration(),
		MemorySensitivityMigration(),
		ChatScopeIndexesMigration(),
		MessageSequenceCheckpointMigration(),
		TombstoneRebuildMigration(),
		InteractionRecordsCreateMigration(),
		InteractionRecordsV2Migration(),
		ProactiveDeliveryTrackingMigration(),
		RuntimeQueueMigration(),
		LegacyDataMigration(),
		TriggerHistoryMigration(),
		RelationshipScopeMigration(),
		NeedStatesMigration(),
		ExtensionsMigration(),
		PluginRuntimeMigration(),
		ExtensionWorkshopMigration(),
		ExtensionWorkshopPermissionScopesMigration(),
		ExtensionWorkshopPlannerMigration(),
		ExtensionWorkshopGenerationSummaryMigration(),
		ExtensionAgentSkillsMigration(),
		ExtensionAgentSkillTraceMigration(),
		ExtensionPackagesMigration(),
		ExtensionScopeBindingsMigration(),
		ExtensionOwnedResourcesMigration(),
		ExtensionPackageRecoveryMigration(),
		ExtensionArtifactRecoveryMigration(),
		ExtensionScheduleSourceMigration(),
		ExtensionScheduleOwnershipRepairMigration(),
		EmotesMigration(),
		MessagePlanMigration(),
		TemporalCoreMigration(),
		TemporalRelationshipTimeMigration(),
		CanonicalSingleUserMigration(),
		CharacterBaseColumnMigration(),
		MCPClientMigration(),
		UserProfileCharacterScopeMigration(),
		EpisodicMessageTimeMigration(),
		PipelineCheckpointLocalTimeMigration(),
		ImageGenConfigMigration(),
		DesktopPetActionDefinitionsMigration(),
		ImageGenConfigEnabledMigration(),
		DesktopPetGenerationTasksMigration(),
		DesktopPetTaskExecutionFieldsMigration(),
		DesktopPetActionExecutionFieldsMigration(),
		DesktopPetGenerationFramesMigration(),
		DesktopPetGenerationCallLogsMigration(),
		DesktopPetProcessingTasksMigration(),
		DesktopPetProcessingActionsMigration(),
		DesktopPetProcessedFramesMigration(),
		DesktopPetPackagesMigration(),
		DesktopPetInstallationsMigration(),
		DesktopPetRuntimeSettingsMigration(),
		ConsolidationAutoMigrateMigration(),
		QdrantMigration(),
		SurrealMigration(),
		MCPDuplicateRegistrationsMigration(),
		ProviderApiTypeMigration(),
		TtsAsrBaseUrlMigration(),
		DesktopPetDataConsistencyMigration(),
		DesktopPetStateMachineMigration(),
		DesktopPetRuntimeClientsMigration(),
		DesktopPetRuntimeCommandsMigration(),
		DesktopPetRuntimeActualStatesMigration(),
		DesktopPetActionSpecContractMigration(),
		DesktopPetRuntimeSettingsV2Migration(),
		DesktopPetGenerationChainMigration(),
		DesktopPetCatalogPopulateMigration(),
		DesktopPetQualitySystemMigration(),
		DesktopPetProcessingRevisionsMigration(),
		DesktopPetEditingMigration(),
		DesktopPetPackageReleaseMigration(),
		DesktopPetBehaviorMigration(),
		DesktopPetPhase8GatesMigration(),
		DesktopPetGenerationActiveArtifactMigration(),
		DesktopPetProcessingCommitRecoveryMigration(),
		DesktopPetRevisionBridgeMigration(),
		DesktopPetActionRevisionBridgeV2Migration(),
		DesktopPetActionRevisionDataMigrateMigration(),
		DesktopPetReleaseDomainMigration(),
		DesktopPetQualityBridgeMigration(),
		DesktopPetInstallationCoordinatorMigration(),
		DesktopPetEditingV2Migration(),
		DesktopPetEditingV3Migration(),
		DesktopPetRuntimeProtocolV2Migration(),
		DesktopPetRuntimeV2TablesMigration(),
		DesktopPetGenerationPlanTablesMigration(),
		DesktopPetProcessingAtomicCommitMigration(),
		DesktopPetQualityV2Migration(),
		DesktopPetReleaseDomainV2Migration(),
		DesktopPetReleaseDomainV3Migration(),
		DesktopPetInstallationV2Migration(),
		DesktopPetBehaviorV2ColumnsMigration(),
		DesktopPetLocalSessionMigration(),
		CharacterCardMigration(),
		BackupTablesUpgradeMigration(),
		RuntimeBootstrapTicketMigration(),
		RuntimeBootstrapTicketRuntimeIDForwardFix(),
		RotationJournalMigration(),
		DesktopPetRuntimeV2CommandForwardFixMigration(),
		DesktopPetImportStagingsMigration(),
		DesktopPetMigrationControlMigration(),
		DesktopPetCutoverUniqueMigration(),
		DesktopPetInstallationOperationIdempotencyMigration(),
		DesktopPetDevicesMigration(),
		DesktopPetLocalSessionFixMigration(),
		DesktopPetDesiredStateSchemaForwardFixMigration(),
		ExtensionEventOutboxDomainCausationMigration(),
		DesktopPetImportSagaFieldsMigration(),
		WorkspaceMountsMigration(),
		WorkspaceMountsRemoteMigration(),
		ModelConfigProtocolMigration(),
		ModelConfigProviderConfigMigration(),
		EmbeddingConfigProviderConfigMigration(),
		VoiceProfileMigration(),
		MemoryTimeQueryIndexesMigration(),
		MemorySummaryConsolidationMigration(),
		ProductionCutoverMigration(),
		TaskRunPauseColumnsMigration(),
		ArtifactsMigration(),
		AccountSessionSecurityMigration(),
		SyncChangeLogMigration(),
		SyncCursorMigration(),
		SyncSequenceMigration(),
		ExecutionResumeMigration(),
		SyncMutationIdempotentMigration(),
		SyncChangeLogUserIDMigration(),
		SyncCursorCompositeKeyMigration(),
		SyncMutationUserUniqueMigration(),
		SyncChangeLogScopeMigration(),
		AppSettingsRevisionMigration(),
		AppSettingsTombstoneMigration(),
		SecurityAuditEventsColumnsMigration(),
		SecurityAuditEventsOccurredAtMigration(),
		AuthSessionsMissingColumnsMigration(),
		SyncSchemaRevisionDeletedAtMigration(),
		SyncMutationClaimsMigration(),
		SessionTimestampCompatibilityMigration(),
		DesktopPetActionRevisionSourceIndexFixMigration(),
		DesktopPetQualityInboxFinalizationMigration(),
		DesktopPetEditingCanonicalFinalizationMigration(),
		DesktopPetSchemaFinalizationMigration(),
		DesktopPetBehaviorDecisionRecoveryMigration(),
		DesktopPetProcessingOwnershipBackfillMigration(),
		DesktopPetRuntimeBehaviorFinalizationMigration(),
		DesktopPetBehaviorReducerDedupMigration(),
		DesktopPetBehaviorInboxTenantDedupMigration(),
		DesktopPetBehaviorV2ColumnsRepairMigration(),
		DesktopPetActionRevisionDataRepairMigration(),
	}
}

func SyncMutationClaimsMigration() Migration {
	return Migration{
		Version:           "20260818004",
		Name:              "create_sync_mutation_claims_table",
		AcceptedChecksums: []string{"c3d4e5f6789012345678901234567890abcdef1234567890abcdef1234567890"},
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS sync_mutation_claims (
				user_id TEXT NOT NULL,
				scope TEXT NOT NULL DEFAULT 'device',
				mutation_id TEXT NOT NULL,
				status TEXT NOT NULL DEFAULT 'pending',
				created_at DATETIME NOT NULL DEFAULT '',
				PRIMARY KEY (user_id, scope, mutation_id)
			)`)
			s.CreateIndex("idx_sync_mutation_claims_status", "sync_mutation_claims", []string{"status"}, false)
			return nil
		},
	}
}

// AppSettingsTombstoneMigration intentionally uses a forward-only version.
// 20260818005 is owned by DesktopPetCutoverUniqueMigration; reusing it caused
// Runner checksum mismatches before this migration could execute.
func AppSettingsTombstoneMigration() Migration {
	return Migration{
		Version: "202608290002",
		Name:    "add_app_settings_deleted_at_column",
		Up: func(s *Step) error {
			s.AddColumn("app_settings", "deleted_at", "DATETIME")
			s.CreateIndex("idx_app_settings_deleted_at", "app_settings", []string{"deleted_at"}, false)
			s.Execute("UPDATE app_settings SET revision = 1 WHERE revision IS NULL OR revision < 1")
			return nil
		},
	}
}

func SecurityAuditEventsColumnsMigration() Migration {
	return Migration{
		Version: "20260817003",
		Name:    "add_security_audit_events_missing_columns",
		Up: func(s *Step) error {
			s.AddColumn("security_audit_events", "actor_type", "TEXT")
			s.AddColumn("security_audit_events", "auth_method", "TEXT")
			s.AddColumn("security_audit_events", "reason_code", "TEXT")
			s.AddColumn("security_audit_events", "details_json", "TEXT")
			return nil
		},
	}
}

func SecurityAuditEventsOccurredAtMigration() Migration {
	return Migration{
		Version: "20260817004",
		Name:    "add_security_audit_events_occurred_at",
		Up: func(s *Step) error {
			s.AddColumn("security_audit_events", "occurred_at", "TEXT")
			return nil
		},
	}
}

func AuthSessionsMissingColumnsMigration() Migration {
	return Migration{
		Version: "20260817005",
		Name:    "add_auth_sessions_missing_columns",
		Up: func(s *Step) error {
			s.AddColumn("auth_sessions", "username", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("auth_sessions", "role", "TEXT NOT NULL DEFAULT 'user'")
			return nil
		},
	}
}

func SyncSchemaRevisionDeletedAtMigration() Migration {
	return Migration{
		Version:           "20260818003",
		Name:              "add_sync_revision_deleted_at_to_core_tables",
		AcceptedChecksums: []string{"d4ee01cc64921410e5212b8dc11c39d09d22ca9f72ced95ba02ea758080d464f"},
		Up: func(s *Step) error {
			s.AddColumn("characters", "revision", "INTEGER NOT NULL DEFAULT 1")
			s.AddColumn("characters", "deleted_at", "DATETIME")
			s.AddColumn("conversations", "revision", "INTEGER NOT NULL DEFAULT 1")
			s.AddColumn("conversations", "deleted_at", "DATETIME")
			s.AddColumn("messages", "revision", "INTEGER NOT NULL DEFAULT 1")
			s.AddColumn("messages", "deleted_at", "DATETIME")
			return nil
		},
	}
}

func SyncMutationIdempotentMigration() Migration {
	return Migration{
		Version: "20260817002",
		Name:    "sync_mutation_idempotent_unique_index",
		Up: func(s *Step) error {
			s.Execute("DROP INDEX IF EXISTS idx_sync_changes_mutation")
			s.CreateIndex("idx_sync_changes_mutation", "sync_changes", []string{"mutation_id"}, true)
			return nil
		},
	}
}

func SyncChangeLogUserIDMigration() Migration {
	return Migration{
		Version: "20260817006",
		Name:    "add_sync_changes_user_id_column",
		Up: func(s *Step) error {
			s.AddColumn("sync_changes", "user_id", "TEXT NOT NULL DEFAULT ''")
			s.CreateIndex("idx_sync_changes_user", "sync_changes", []string{"user_id"}, false)
			s.CreateIndex("idx_sync_changes_user_mutation", "sync_changes", []string{"user_id", "mutation_id"}, false)
			return nil
		},
	}
}

func SyncCursorCompositeKeyMigration() Migration {
	return Migration{
		Version: "20260817007",
		Name:    "sync_cursors_composite_primary_key",
		Up: func(s *Step) error {
			s.Execute(`CREATE TABLE IF NOT EXISTS sync_cursors_new (
				device_id TEXT NOT NULL,
				user_id TEXT NOT NULL,
				scope TEXT NOT NULL DEFAULT 'device',
				last_applied INTEGER NOT NULL DEFAULT 0,
				last_pushed INTEGER NOT NULL DEFAULT 0,
				updated_at DATETIME NOT NULL,
				PRIMARY KEY (user_id, scope, device_id)
			)`)
			s.Execute(`INSERT OR IGNORE INTO sync_cursors_new (device_id, user_id, scope, last_applied, last_pushed, updated_at)
				SELECT device_id, user_id, scope, last_applied, last_pushed, updated_at FROM sync_cursors`)
			s.Execute("DROP TABLE IF EXISTS sync_cursors")
			s.Execute("ALTER TABLE sync_cursors_new RENAME TO sync_cursors")
			s.CreateIndex("idx_sync_cursors_user", "sync_cursors", []string{"user_id"}, false)
			return nil
		},
	}
}

func SyncMutationUserUniqueMigration() Migration {
	return Migration{
		Version: "20260817008",
		Name:    "sync_mutation_user_unique_index",
		Up: func(s *Step) error {
			s.Execute("DROP INDEX IF EXISTS idx_sync_changes_mutation")
			s.Execute("DROP INDEX IF EXISTS idx_sync_changes_user_mutation")
			s.CreateIndex("idx_sync_changes_user_mutation", "sync_changes", []string{"user_id", "mutation_id"}, true)
			return nil
		},
	}
}

func SyncChangeLogScopeMigration() Migration {
	return Migration{
		Version:           "20260818001",
		Name:              "add_sync_changes_scope_column_and_unique_index",
		AcceptedChecksums: []string{"1e1e5c00c95502f3af8dd55a41aece51599e42ff321935d63025da69c38b5762"},
		Up: func(s *Step) error {
			s.AddColumn("sync_changes", "scope", "TEXT NOT NULL DEFAULT 'device'")
			s.Execute("DROP INDEX IF EXISTS idx_sync_changes_user_mutation")
			s.CreateIndex("idx_sync_changes_scope", "sync_changes", []string{"scope"}, false)
			s.CreateIndex("idx_sync_changes_user_scope_mutation", "sync_changes", []string{"user_id", "scope", "mutation_id"}, true)
			return nil
		},
	}
}

func AppSettingsRevisionMigration() Migration {
	return Migration{
		Version:           "20260818002",
		Name:              "add_app_settings_revision_column",
		AcceptedChecksums: []string{"469fef008bc67be2344dc0c5b4c0d8734430c302ca9f7f5bae1f22e893ead64b"},
		Up: func(s *Step) error {
			s.AddColumn("app_settings", "revision", "INTEGER NOT NULL DEFAULT 0")
			return nil
		},
	}
}

func ExecutionResumeMigration() Migration {
	return Migration{
		Version: "20260816003",
		Name:    "execution_resumes_table",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS execution_resumes (
				resume_id TEXT PRIMARY KEY,
				root_execution_id TEXT,
				parent_execution_id TEXT,
				resume_type TEXT NOT NULL,
				resume_state TEXT NOT NULL,
				checkpoint_ref TEXT,
				required_capability_id TEXT,
				acquisition_transaction_id TEXT,
				task_id TEXT,
				payload_ref TEXT,
				reason TEXT,
				metadata TEXT,
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL
			)`)
			s.CreateIndex("idx_execution_resumes_state", "execution_resumes", []string{"resume_state"}, false)
			s.CreateIndex("idx_execution_resumes_root", "execution_resumes", []string{"root_execution_id"}, false)
			s.CreateIndex("idx_execution_resumes_acq_txn", "execution_resumes", []string{"acquisition_transaction_id"}, false)
			return nil
		},
	}
}

func SyncSequenceMigration() Migration {
	return Migration{
		Version:           "20260817001",
		Name:              "create_sync_sequence_table",
		AcceptedChecksums: []string{"4bef88e97e20a93914eb87805f639f4a7c2ad684ce552590fc0f79644dc5197"},
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS sync_sequence (
				id INTEGER PRIMARY KEY CHECK (id = 1),
				seq INTEGER NOT NULL DEFAULT 0
			)`)
			s.Execute("INSERT OR IGNORE INTO sync_sequence (id, seq) VALUES (1, 0)")
			return nil
		},
	}
}

func ArtifactsMigration() Migration {
	return Migration{
		Version: "20260815002",
		Name:    "create_artifacts_and_references_tables",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS artifacts (
				artifact_id TEXT PRIMARY KEY,
				owner_user_id TEXT NOT NULL,
				workspace_id TEXT NOT NULL DEFAULT '',
				kind TEXT NOT NULL,
				blob_digest TEXT NOT NULL,
				size_bytes INTEGER NOT NULL,
				mime_type TEXT NOT NULL,
				filename TEXT NOT NULL DEFAULT '',
				file_extension TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL,
				source TEXT NOT NULL,
				width INTEGER NOT NULL DEFAULT 0,
				height INTEGER NOT NULL DEFAULT 0,
				duration_ms INTEGER NOT NULL DEFAULT 0,
				revision INTEGER NOT NULL DEFAULT 1,
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL,
				deleted_at DATETIME
			)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS artifact_references (
				artifact_id TEXT NOT NULL,
				reference_type TEXT NOT NULL,
				reference_id TEXT NOT NULL,
				created_at DATETIME NOT NULL,
				PRIMARY KEY(artifact_id, reference_type, reference_id)
			)`)
			s.CreateIndex("idx_artifacts_owner", "artifacts", []string{"owner_user_id"}, false)
			s.CreateIndex("idx_artifacts_blob_digest", "artifacts", []string{"blob_digest"}, false)
			s.CreateIndex("idx_artifacts_status", "artifacts", []string{"status"}, false)
			s.CreateIndex("idx_artifacts_created_at", "artifacts", []string{"created_at"}, false)
			return nil
		},
	}
}

func AccountSessionSecurityMigration() Migration {
	return Migration{
		Version: "20260815003",
		Name:    "account_session_security_tables",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS auth_refresh_tokens (
				token_id TEXT PRIMARY KEY,
				session_id TEXT NOT NULL,
				token_hash TEXT NOT NULL UNIQUE,
				status TEXT NOT NULL DEFAULT 'active',
				issued_at DATETIME NOT NULL,
				expires_at DATETIME NOT NULL,
				used_at DATETIME,
				revoked_at DATETIME,
				replaced_by_token_id TEXT,
				created_at DATETIME NOT NULL DEFAULT ''
			)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS auth_login_guards (
				guard_key TEXT NOT NULL,
				dimension TEXT NOT NULL,
				failure_count INTEGER NOT NULL DEFAULT 0,
				window_started_at DATETIME NOT NULL,
				blocked_until DATETIME,
				updated_at DATETIME NOT NULL DEFAULT '',
				PRIMARY KEY(guard_key, dimension)
			)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS security_audit_events (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				event_id TEXT NOT NULL UNIQUE,
				user_id TEXT,
				session_id TEXT,
				event_type TEXT NOT NULL,
				outcome TEXT NOT NULL DEFAULT 'success',
				severity TEXT NOT NULL DEFAULT 'info',
				ip_address TEXT,
				user_agent TEXT,
				device_name TEXT,
				detail TEXT,
				created_at DATETIME NOT NULL DEFAULT ''
			)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS auth_recovery_codes (
				code_id TEXT PRIMARY KEY,
				user_id INTEGER NOT NULL,
				code_hash TEXT NOT NULL UNIQUE,
				status TEXT NOT NULL DEFAULT 'active',
				created_at DATETIME NOT NULL DEFAULT '',
				used_at DATETIME,
				expires_at DATETIME,
				generation INTEGER NOT NULL DEFAULT 1
			)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS auth_recovery_grants (
				grant_id TEXT PRIMARY KEY,
				user_id INTEGER NOT NULL,
				grant_hash TEXT NOT NULL UNIQUE,
				status TEXT NOT NULL DEFAULT 'active',
				created_at DATETIME NOT NULL DEFAULT '',
				expires_at DATETIME NOT NULL,
				consumed_at DATETIME
			)`)
			s.AddColumn("auth_sessions", "public_id", "TEXT")
			s.AddColumn("auth_sessions", "status", "TEXT NOT NULL DEFAULT 'active'")
			s.AddColumn("auth_sessions", "revision", "INTEGER NOT NULL DEFAULT 1")
			s.AddColumn("auth_sessions", "absolute_expires_at", "DATETIME")
			s.AddColumn("auth_sessions", "revoked_at", "DATETIME")
			s.AddColumn("auth_sessions", "revoke_reason", "TEXT")
			s.AddColumn("auth_sessions", "last_refreshed_at", "DATETIME")
			s.CreateIndex("idx_auth_sessions_public_id", "auth_sessions", []string{"public_id"}, true)
			s.CreateIndex("idx_auth_sessions_user_status", "auth_sessions", []string{"user_id", "status"}, false)
			s.CreateIndex("idx_refresh_tokens_session", "auth_refresh_tokens", []string{"session_id"}, false)
			s.CreateIndex("idx_refresh_tokens_status", "auth_refresh_tokens", []string{"status", "expires_at"}, false)
			s.CreateIndex("idx_recovery_codes_user", "auth_recovery_codes", []string{"user_id", "status"}, false)
			s.CreateIndex("idx_recovery_grants_user", "auth_recovery_grants", []string{"user_id"}, false)
			s.CreateIndex("idx_audit_events_user", "security_audit_events", []string{"user_id", "created_at"}, false)
			s.CreateIndex("idx_audit_events_type", "security_audit_events", []string{"event_type", "created_at"}, false)
			return nil
		},
	}
}

func SyncChangeLogMigration() Migration {
	return Migration{
		Version: "20260816001",
		Name:    "create_sync_changes_table",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS sync_changes (
				change_id TEXT PRIMARY KEY,
				seq INTEGER NOT NULL,
				entity_type TEXT NOT NULL,
				entity_id TEXT NOT NULL,
				operation TEXT NOT NULL,
				revision INTEGER NOT NULL,
				mutation_id TEXT,
				origin_device TEXT,
				payload BLOB,
				checksum TEXT NOT NULL,
				created_at DATETIME NOT NULL
			)`)
			s.CreateIndex("idx_sync_changes_seq", "sync_changes", []string{"seq"}, true)
			s.CreateIndex("idx_sync_changes_entity", "sync_changes", []string{"entity_type", "entity_id"}, false)
			s.CreateIndex("idx_sync_changes_mutation", "sync_changes", []string{"mutation_id"}, false)
			s.CreateIndex("idx_sync_changes_origin", "sync_changes", []string{"origin_device"}, false)
			return nil
		},
	}
}

func SyncCursorMigration() Migration {
	return Migration{
		Version: "20260816002",
		Name:    "create_sync_cursors_table",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS sync_cursors (
				device_id TEXT PRIMARY KEY,
				user_id TEXT NOT NULL,
				scope TEXT NOT NULL DEFAULT 'device',
				last_applied INTEGER NOT NULL DEFAULT 0,
				last_pushed INTEGER NOT NULL DEFAULT 0,
				updated_at DATETIME NOT NULL
			)`)
			s.CreateIndex("idx_sync_cursors_user", "sync_cursors", []string{"user_id"}, false)
			return nil
		},
	}
}

func TaskRunPauseColumnsMigration() Migration {
	return Migration{
		Version: "20260815001",
		Name:    "add_task_run_pause_and_updated_columns",
		Up: func(s *Step) error {
			s.AddColumn("extension_task_runs", "updated_at", "DATETIME NOT NULL DEFAULT ''")
			s.AddColumn("extension_task_runs", "pause_reason", "TEXT")
			s.AddColumn("extension_task_runs", "pause_requested_at", "DATETIME")
			s.AddColumn("extension_task_runs", "paused_at", "DATETIME")
			s.AddColumn("extension_task_runs", "resumed_at", "DATETIME")
			return nil
		},
	}
}
