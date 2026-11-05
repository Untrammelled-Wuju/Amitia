package kernel

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/builtin"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
)

func TestReconcileBuiltinUIContributionsReplacesMaterializedSchema(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "ui-reconcile.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := sqlite.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	repository := sqlite.NewSQLiteUIContributionRepository(db)
	builtinDefinition := builtin.BuildEmotionExtension("1.0.0")
	builtinContribution := builtinDefinition.Extension.AllContributions()[0]
	stale := &ui_contribution.UIContributionDefinition{
		ContributionID: ui_contribution.ContributionID(builtinContribution.ID),
		ExtensionID:    ui_contribution.ExtensionID(builtinDefinition.Extension.ID),
		ModuleID:       ui_contribution.ModuleID(builtinContribution.ModuleID),
		Integrity:      ui_contribution.ContributionIntegrity{DefinitionHash: "stale"},
	}
	if err := repository.PutContribution(context.Background(), stale); err != nil {
		t.Fatal(err)
	}
	catalog := builtin.NewCatalog()
	if err := catalog.Register(builtinDefinition); err != nil {
		t.Fatal(err)
	}
	if err := reconcileBuiltinUIContributions(context.Background(), catalog, repository); err != nil {
		t.Fatal(err)
	}
	definition, err := repository.GetContribution(context.Background(), string(builtinContribution.ID))
	if err != nil {
		t.Fatal(err)
	}
	if definition.Integrity.DefinitionHash != "builtin-"+string(builtinDefinition.Extension.ID)+"-v1" {
		t.Fatalf("unexpected materialized contribution: %+v", definition)
	}
}
