package temporal

import (
	"testing"
	"time"
)

func TestRelationshipTimeOwnedCompletionRetryAndReunion(t *testing.T) {
	now := time.Date(2026, 10, 23, 1, 0, 0, 0, time.UTC)
	repo, _, clock := newRelationshipTimeTestRepository(t, now)
	c := NewRelationshipTimeCoordinator(repo, clock).ForOwner("phone")
	space := RelationshipOwnerSpace("space", "phone")
	input := PrepareInboundInput{SpaceID: space, CharacterID: "role", RequestID: "first", InteractionID: "turn-first", Message: "你好"}
	if _, err := c.PrepareInbound(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := c.FinishRequest(t.Context(), space, "role", "first", input.Message, false, false); err != nil {
			t.Fatal(err)
		}
	}
	state, err := c.GetState(t.Context(), space, "role")
	if err != nil || state.InteractionCount != 1 || state.FirstInteractionAt.IsZero() {
		t.Fatalf("completion: %+v %v", state, err)
	}
	clock.Advance(10 * 24 * time.Hour)
	input.RequestID, input.InteractionID, input.Message = "return", "turn-return", "好久不见"
	prepared, err := c.PrepareInbound(t.Context(), input)
	if err != nil || prepared.Reunion == nil || !prepared.Reunion.ShouldExpress || prepared.Policy == nil {
		t.Fatalf("prepare: %+v %v", prepared, err)
	}
	if RenderRelationshipTime(prepared, *prepared.Policy) == "" {
		t.Fatal("reunion context missing")
	}
	if err := c.FinishRequest(t.Context(), space, "role", "return", input.Message, true, false); err != nil {
		t.Fatal(err)
	}
	state, _ = c.GetState(t.Context(), space, "role")
	if state.InteractionCount != 1 {
		t.Fatal("failed reply advanced presence")
	}
	input.InteractionID = "turn-retry"
	prepared, err = c.PrepareInbound(t.Context(), input)
	if err != nil || prepared.Reunion == nil || !prepared.Reunion.ShouldExpress {
		t.Fatalf("retry did not reclaim: %+v %v", prepared, err)
	}
	for i := 0; i < 2; i++ {
		if err := c.FinishRequest(t.Context(), space, "role", "return", input.Message, false, false); err != nil {
			t.Fatal(err)
		}
	}
	state, _ = c.GetState(t.Context(), space, "role")
	if state.InteractionCount != 2 || state.ReacclimationTurnsLeft == 0 {
		t.Fatalf("reunion not committed: %+v", state)
	}
	episodes, err := c.ListReunionEpisodes(t.Context(), space, "role", 50)
	if err != nil || len(episodes) != 1 || episodes[0].Status != ReunionStateHandled {
		t.Fatalf("episodes: %+v %v", episodes, err)
	}
}

func TestRelationshipTimeOwnedIsolationAndProactive(t *testing.T) {
	repo, _, clock := newRelationshipTimeTestRepository(t, time.Date(2026, 10, 23, 1, 0, 0, 0, time.UTC))
	base := NewRelationshipTimeCoordinator(repo, clock)
	phone, core := base.ForOwner("phone"), base.ForOwner("core")
	settings := DefaultRelationshipTimeSettings("role")
	settings.Enabled = false
	if err := phone.SaveSettings(t.Context(), &settings); err != nil {
		t.Fatal(err)
	}
	if value, err := core.GetSettings(t.Context(), "role"); err != nil || value != nil {
		t.Fatalf("settings leaked: %+v %v", value, err)
	}
	space := RelationshipOwnerSpace("space", "core")
	input := PrepareInboundInput{SpaceID: space, CharacterID: "role", RequestID: "human", InteractionID: "human-turn", Message: "你好"}
	if _, err := core.PrepareInbound(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	if err := core.FinishRequest(t.Context(), space, "role", "human", input.Message, false, false); err != nil {
		t.Fatal(err)
	}
	clock.Advance(time.Hour)
	input.RequestID, input.InteractionID, input.Source = "proactive", "proactive-turn", "proactive"
	if _, err := core.PrepareInbound(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	if err := core.FinishRequest(t.Context(), space, "role", "proactive", input.Message, false, true); err != nil {
		t.Fatal(err)
	}
	state, _ := core.GetState(t.Context(), space, "role")
	if state.InteractionCount != 1 || state.LastAssistantContactAt.IsZero() {
		t.Fatalf("proactive counted as human: %+v", state)
	}
	other, _ := phone.GetState(t.Context(), RelationshipOwnerSpace("space", "phone"), "role")
	if other.InteractionCount != 0 {
		t.Fatal("presence leaked")
	}
}

func TestRelationshipTimeOwnedConcurrentReunionAndPolicy(t *testing.T) {
	repo, _, clock := newRelationshipTimeTestRepository(t, time.Date(2026, 10, 23, 1, 0, 0, 0, time.UTC))
	c := NewRelationshipTimeCoordinator(repo, clock)
	prepare := func(request string) RelationshipTimeContext {
		t.Helper()
		v, e := c.PrepareInbound(t.Context(), PrepareInboundInput{SpaceID: "space", CharacterID: "role", RequestID: request, InteractionID: request, Message: "好久不见"})
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	prepare("first")
	if e := c.FinishRequest(t.Context(), "space", "role", "first", "你好", false, false); e != nil {
		t.Fatal(e)
	}
	clock.Advance(10 * 24 * time.Hour)
	first, second := prepare("one"), prepare("two")
	if first.Reunion == nil || second.Reunion == nil || second.Reunion.ShouldExpress {
		t.Fatal("duplicate reunion claimed")
	}
	if e := c.FinishRequest(t.Context(), "space", "role", "two", "好久不见", false, false); e != nil {
		t.Fatal(e)
	}
	if e := c.FinishRequest(t.Context(), "space", "role", "one", "好久不见", false, false); e != nil {
		t.Fatal(e)
	}
	settings := DefaultRelationshipTimeSettings("role")
	settings.Sensitivity = "expressive"
	settings.MaxMentionSentences = 0
	policy := ResolveRelationshipTimePolicy(first, "好久不见", false, &settings)
	if policy.MentionMode != ReunionMentionNone || policy.MaxMentionSentences != 0 {
		t.Fatalf("zero limit ignored: %+v", policy)
	}
}

func TestRelationshipTimeOwnedMigrationIsIdempotent(t *testing.T) {
	repo, db, clock := newRelationshipTimeTestRepository(t, time.Date(2026, 10, 23, 1, 0, 0, 0, time.UTC))
	if err := db.Exec("CREATE TABLE characters(id TEXT PRIMARY KEY)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO characters(id) VALUES('role')").Error; err != nil {
		t.Fatal(err)
	}
	c := NewRelationshipTimeCoordinator(repo, clock)
	if _, err := c.PrepareInbound(t.Context(), PrepareInboundInput{SpaceID: "legacy", CharacterID: "role", RequestID: "request", InteractionID: "turn"}); err != nil {
		t.Fatal(err)
	}
	if err := c.FinishRequest(t.Context(), "legacy", "role", "request", "你好", false, false); err != nil {
		t.Fatal(err)
	}
	settings := DefaultRelationshipTimeSettings("role")
	settings.MaxMentionSentences = 0
	if err := c.SaveSettings(t.Context(), &settings); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := c.MigrateOwner(t.Context(), "legacy", "phone"); err != nil {
			t.Fatal(err)
		}
	}
	owned := c.ForOwner("phone")
	state, err := owned.GetState(t.Context(), RelationshipOwnerSpace("legacy", "phone"), "role")
	if err != nil || state.InteractionCount != 1 {
		t.Fatalf("migration lost state: %+v %v", state, err)
	}
	saved, err := owned.GetSettings(t.Context(), "role")
	if err != nil || saved == nil || saved.CharacterID != "role" || saved.MaxMentionSentences != 0 {
		t.Fatalf("migration lost settings: %+v %v", saved, err)
	}
	old, _ := c.GetState(t.Context(), "legacy", "role")
	if old.InteractionCount != 0 {
		t.Fatal("legacy state remained active")
	}
}
