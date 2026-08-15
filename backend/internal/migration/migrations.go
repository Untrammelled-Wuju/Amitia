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
	DesktopPetDevicesMigration(),
	DesktopPetLocalSessionFixMigration(),
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
				created_at DATETIME NOT NULL DEFAULT (datetime('now'))
			)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS auth_login_guards (
				guard_key TEXT NOT NULL,
				dimension TEXT NOT NULL,
				failure_count INTEGER NOT NULL DEFAULT 0,
				window_started_at DATETIME NOT NULL,
				blocked_until DATETIME,
				updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
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
				created_at DATETIME NOT NULL DEFAULT (datetime('now'))
			)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS auth_recovery_codes (
				code_id TEXT PRIMARY KEY,
				user_id INTEGER NOT NULL,
				code_hash TEXT NOT NULL UNIQUE,
				status TEXT NOT NULL DEFAULT 'active',
				created_at DATETIME NOT NULL DEFAULT (datetime('now')),
				used_at DATETIME,
				expires_at DATETIME,
				generation INTEGER NOT NULL DEFAULT 1
			)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS auth_recovery_grants (
				grant_id TEXT PRIMARY KEY,
				user_id INTEGER NOT NULL,
				grant_hash TEXT NOT NULL UNIQUE,
				status TEXT NOT NULL DEFAULT 'active',
				created_at DATETIME NOT NULL DEFAULT (datetime('now')),
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

func TaskRunPauseColumnsMigration() Migration {
	return Migration{
		Version: "20260815001",
		Name:    "add_task_run_pause_and_updated_columns",
		Up: func(s *Step) error {
			s.AddColumn("extension_task_runs", "updated_at", "DATETIME NOT NULL DEFAULT (datetime('now'))")
			s.AddColumn("extension_task_runs", "pause_reason", "TEXT")
			s.AddColumn("extension_task_runs", "pause_requested_at", "DATETIME")
			s.AddColumn("extension_task_runs", "paused_at", "DATETIME")
			s.AddColumn("extension_task_runs", "resumed_at", "DATETIME")
			return nil
		},
	}
}
