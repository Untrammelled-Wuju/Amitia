package migration

import (
	"context"
	"strings"
	"testing"
)

func migrationString(value string) *string {
	return &value
}

func reversibleDefinition(extensionID, id, from, to string, direction MigrationDirection, linked string) MigrationDefinition {
	definition := MigrationDefinition{
		MigrationID:      id,
		ExtensionID:      extensionID,
		FromVersionRange: from,
		ToVersion:        to,
		Entry:            "migrations/" + id + ".js",
		RuntimeType:      "javascript",
		Direction:        direction,
		Idempotency:      IdempotencyIdempotent,
		Reversibility:    ReversibilityFullyReversible,
		DefinitionHash:   "hash-" + id,
	}
	if direction == DirectionForward {
		definition.ReverseMigrationID = migrationString(linked)
	} else {
		definition.ForwardMigrationID = migrationString(linked)
	}
	return definition
}

func reversibleChain(extensionID string) []MigrationDefinition {
	return []MigrationDefinition{
		reversibleDefinition(extensionID, "f1", "1.0.0", "1.1.0", DirectionForward, "r1"),
		reversibleDefinition(extensionID, "r1", "1.1.0", "1.0.0", DirectionReverse, "f1"),
		reversibleDefinition(extensionID, "f2", "1.1.0", "2.0.0", DirectionForward, "r2"),
		reversibleDefinition(extensionID, "r2", "2.0.0", "1.1.0", DirectionReverse, "f2"),
	}
}

func TestReversiblePreflightDeterministicAndReverseOrder(t *testing.T) {
	definitions := reversibleChain("ext.test")
	core := NewReversibleMigrationCore(nil)
	first, err := core.Preflight(context.Background(), ReversiblePreflightInput{ExtensionID: "ext.test", FromVersion: "1.0.0", ToVersion: "2.0.0", Definitions: definitions})
	if err != nil {
		t.Fatal(err)
	}
	reordered := []MigrationDefinition{definitions[3], definitions[1], definitions[2], definitions[0]}
	second, err := core.Preflight(context.Background(), ReversiblePreflightInput{ExtensionID: "ext.test", FromVersion: "1.0.0", ToVersion: "2.0.0", Definitions: reordered})
	if err != nil {
		t.Fatal(err)
	}
	if first.PlanHash != second.PlanHash {
		t.Fatalf("plan hash changed with definition order: %s != %s", first.PlanHash, second.PlanHash)
	}
	if len(first.ForwardPlan) != 2 || first.ForwardPlan[0].MigrationID != "f1" || first.ForwardPlan[1].MigrationID != "f2" {
		t.Fatalf("unexpected forward plan: %+v", first.ForwardPlan)
	}
	if len(first.ReversePlan) != 2 || first.ReversePlan[0].MigrationID != "r2" || first.ReversePlan[1].MigrationID != "r1" {
		t.Fatalf("unexpected reverse plan: %+v", first.ReversePlan)
	}
}

func TestReversiblePreflightValidation(t *testing.T) {
	tests := []struct {
		name        string
		definitions func() []MigrationDefinition
		want        string
	}{
		{
			name: "duplicate id",
			definitions: func() []MigrationDefinition {
				definitions := reversibleChain("ext.test")
				definitions = append(definitions, definitions[0])
				return definitions
			},
			want: "duplicate migration id",
		},
		{
			name: "unsafe path",
			definitions: func() []MigrationDefinition {
				definitions := reversibleChain("ext.test")
				definitions[0].Entry = "../escape.js"
				return definitions
			},
			want: "unsafe migration entry",
		},
		{
			name: "cycle",
			definitions: func() []MigrationDefinition {
				definitions := reversibleChain("ext.test")
				definitions = append(definitions, reversibleDefinition("ext.test", "f3", "2.0.0", "1.0.0", DirectionForward, "r3"))
				return definitions
			},
			want: "contains cycle",
		},
		{
			name: "reverse linkage",
			definitions: func() []MigrationDefinition {
				definitions := reversibleChain("ext.test")
				definitions[1].ForwardMigrationID = migrationString("f2")
				return definitions
			},
			want: "reverse linkage mismatch",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewReversibleMigrationCore(nil).Preflight(context.Background(), ReversiblePreflightInput{ExtensionID: "ext.test", FromVersion: "1.0.0", ToVersion: "2.0.0", Definitions: test.definitions()})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q, got %v", test.want, err)
			}
		})
	}
}

func TestReversiblePreflightMarksManualAndSnapshotRisk(t *testing.T) {
	definitions := reversibleChain("ext.test")[:2]
	definitions[0].DataDomains = []DataDomain{{Domain: "user", Storage: "sqlite", Namespace: "profile"}}
	definitions[0].ReverseMigrationID = nil
	preflight, err := NewReversibleMigrationCore(nil).Preflight(context.Background(), ReversiblePreflightInput{ExtensionID: "ext.test", FromVersion: "1.0.0", ToVersion: "1.1.0", Definitions: definitions})
	if err != nil {
		t.Fatal(err)
	}
	if !preflight.ManualRequired || !preflight.UserDataSnapshotRequired || len(preflight.SnapshotDomains) != 1 {
		t.Fatalf("unexpected risk result: %+v", preflight)
	}
	definitions[0].Reversibility = ReversibilityIrreversible
	preflight, err = NewReversibleMigrationCore(nil).Preflight(context.Background(), ReversiblePreflightInput{ExtensionID: "ext.test", FromVersion: "1.0.0", ToVersion: "1.1.0", Definitions: definitions})
	if err != nil {
		t.Fatal(err)
	}
	if !preflight.Irreversible || !preflight.ManualRequired {
		t.Fatalf("irreversible migration was not blocked: %+v", preflight)
	}
}
