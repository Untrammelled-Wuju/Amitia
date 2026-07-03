package safety

import "testing"

func TestGovernorPreGenEmptyCharacter(t *testing.T) {
	g := NewGovernor(DefaultGovernorConfig())
	out := g.CheckPreGen(PreGenInput{CharacterID: ""})
	if out.Allowed {
		t.Error("expected blocked for empty character")
	}
}

func TestGovernorPreGenAllowed(t *testing.T) {
	g := NewGovernor(DefaultGovernorConfig())
	out := g.CheckPreGen(PreGenInput{CharacterID: "char-1", ProactiveCap: 5})
	if !out.Allowed {
		t.Errorf("expected allowed, got blocked: %v", out.Reasons)
	}
}

func TestGovernorPreGenProactiveCapExceeded(t *testing.T) {
	g := NewGovernor(DefaultGovernorConfig())
	out := g.CheckPreGen(PreGenInput{CharacterID: "char-1", ProactiveCap: 15})
	if out.Allowed {
		t.Error("expected blocked for exceeded proactive cap")
	}
}

func TestGovernorPostGenBlockedWord(t *testing.T) {
	g := NewGovernor(GovernorConfig{BlockedWords: []string{"badword"}})
	out := g.CheckPostGen(PostGenInput{GeneratedText: "contains badword here"})
	if out.Allowed {
		t.Error("expected blocked for bad word")
	}
}

func TestGovernorPostGenClean(t *testing.T) {
	g := NewGovernor(DefaultGovernorConfig())
	out := g.CheckPostGen(PostGenInput{GeneratedText: "clean text"})
	if !out.Allowed {
		t.Errorf("expected allowed, got blocked: %v", out.Reasons)
	}
}

func TestGovernorPreDeliverTombstone(t *testing.T) {
	g := NewGovernor(DefaultGovernorConfig())
	out := g.CheckPreDeliver(PreDeliverInput{TombstoneHit: true})
	if out.Allowed {
		t.Error("expected blocked for tombstone")
	}
}

func TestGovernorPreDeliverAllowed(t *testing.T) {
	g := NewGovernor(DefaultGovernorConfig())
	out := g.CheckPreDeliver(PreDeliverInput{OutputLeaseID: "lease-1", TombstoneHit: false})
	if !out.Allowed {
		t.Errorf("expected allowed, got blocked: %v", out.Reasons)
	}
}
