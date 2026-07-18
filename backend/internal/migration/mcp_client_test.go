package migration

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMCPClientMigrationCreatesSafeSchema(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	runner := Runner{DB: db, SkipBackup: true}
	if err := runner.Apply([]Migration{MCPClientMigration()}); err != nil {
		t.Fatal(err)
	}
	tables := []string{"mcp_servers", "mcp_server_scope_bindings", "mcp_server_credentials", "mcp_server_capabilities", "mcp_tools", "mcp_resources", "mcp_resource_templates", "mcp_prompts", "mcp_dependency_links", "mcp_operations", "mcp_oauth_sessions", "mcp_tasks", "mcp_audit_logs"}
	for _, table := range tables {
		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count).Error; err != nil || count != 1 {
			t.Fatalf("missing table %s: %v", table, err)
		}
	}
	type column struct{ Name string }
	var credentials []column
	if err := db.Raw("PRAGMA table_info(mcp_server_credentials)").Scan(&credentials).Error; err != nil {
		t.Fatal(err)
	}
	for _, item := range credentials {
		name := strings.ToLower(item.Name)
		if name == "access_token" || name == "refresh_token" || name == "client_secret" || name == "password" || name == "private_key" {
			t.Fatalf("plaintext secret column is forbidden: %s", item.Name)
		}
	}
	if err := db.Exec("INSERT INTO mcp_servers (id,name,transport,normalized_identity) VALUES ('1','one','streamable_http','same')").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO mcp_servers (id,name,transport,normalized_identity) VALUES ('2','two','streamable_http','same')").Error; err == nil {
		t.Fatal("expected normalized server identity uniqueness")
	}
}
