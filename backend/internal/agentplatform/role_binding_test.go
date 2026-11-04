package agentplatform

import (
	"testing"
	"time"
)

func TestSyncRoleAgentCreatesAndMergesSingleLifecycle(t *testing.T) {
	service, spaceID, _ := newTestPlatform(t)
	now := time.Now().UTC()
	input := RoleAgentSyncInput{
		SpaceID: spaceID, CharacterID: "role-lifecycle", Name: "角色生命周期",
		Description: "统一角色与 Agent", Instructions: "保持角色身份连续。",
		Status: "enabled", ModelConfigID: 1, ReasoningEffort: "medium",
	}
	profile, err := service.SyncRoleAgent(service.db, input)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Status != "active" || profile.CurrentRevisionID == "" || profile.CurrentReleaseID == "" {
		t.Fatalf("role agent was not auto-published: %+v", profile)
	}
	duplicate := AgentProfile{
		ID: "agent_duplicate", SpaceID: spaceID, CharacterID: input.CharacterID,
		Name: input.Name, Archetype: "companion", Status: "draft",
		CurrentRevisionID: "revision_duplicate", Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := service.db.Create(&duplicate).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.db.Create(&AgentRevision{
		ID: duplicate.CurrentRevisionID, SpaceID: spaceID, AgentID: duplicate.ID,
		Revision: 1, ConfigJSON: "{}", ConfigHash: "duplicate", Status: "draft",
		CreatedAt: now, TestStatus: "not_run",
	}).Error; err != nil {
		t.Fatal(err)
	}
	team := Team{
		ID: "team_lifecycle", SpaceID: spaceID, Name: "生命周期团队",
		LeadAgentID: duplicate.ID, Status: "active", Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := service.db.Create(&team).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.db.Create(&TeamMember{
		ID: "member_duplicate", SpaceID: spaceID, TeamID: team.ID,
		MemberType: "agent", MemberID: duplicate.ID, Role: "member",
		ResponderMode: "mention_or_lead", Capabilities: "[]", CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	updated, err := service.SyncRoleAgent(service.db, input)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != profile.ID {
		t.Fatalf("canonical agent changed: got %s want %s", updated.ID, profile.ID)
	}
	var active []AgentProfile
	if err := service.db.Where("space_id = ? AND character_id = ? AND deleted_at IS NULL", spaceID, input.CharacterID).Find(&active).Error; err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].ID != profile.ID {
		t.Fatalf("duplicate role agents remain: %+v", active)
	}
	var member TeamMember
	if err := service.db.Where("id = ?", "member_duplicate").First(&member).Error; err != nil {
		t.Fatal(err)
	}
	if member.MemberID != profile.ID {
		t.Fatalf("team member was not migrated: %+v", member)
	}
	var currentTeam Team
	if err := service.db.Where("id = ?", team.ID).First(&currentTeam).Error; err != nil {
		t.Fatal(err)
	}
	if currentTeam.LeadAgentID != profile.ID {
		t.Fatalf("team lead was not migrated: %+v", currentTeam)
	}
	input.Name = "角色生命周期更新"
	updated, err = service.SyncRoleAgent(service.db, input)
	if err != nil {
		t.Fatal(err)
	}
	var revisions int64
	if err := service.db.Model(&AgentRevision{}).Where("agent_id = ?", updated.ID).Count(&revisions).Error; err != nil {
		t.Fatal(err)
	}
	if revisions != 2 {
		t.Fatalf("role update did not create one internal revision, got %d", revisions)
	}
	var releases int64
	if err := service.db.Model(&AgentRelease{}).Where("agent_id = ? AND status = ?", updated.ID, "published").Count(&releases).Error; err != nil {
		t.Fatal(err)
	}
	if releases != 1 {
		t.Fatalf("expected one current release, got %d", releases)
	}
}
