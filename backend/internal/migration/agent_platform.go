package migration

import "github.com/u-ai/backend/internal/agentplatform"

func AgentPlatformMigration() Migration {
	return Migration{
		Version: "20261103001",
		Name:    "create_agent_platform_tables",
		Up: func(s *Step) error {
			return s.DB().AutoMigrate(agentplatform.AllModels()...)
		},
	}
}
