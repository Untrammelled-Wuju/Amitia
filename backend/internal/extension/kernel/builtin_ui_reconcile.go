package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/builtin"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
)

func reconcileBuiltinUIContributions(ctx context.Context, catalog *builtin.Catalog, repository *sqlite.SQLiteUIContributionRepository) error {
	if catalog == nil || repository == nil {
		return nil
	}
	active := make(map[ui_contribution.ContributionID]struct{})
	for _, definition := range catalog.List() {
		for _, contribution := range definition.Extension.AllContributions() {
			if contribution.EffectiveAdapter() != domain.ContributionAdapterUI {
				continue
			}
			active[ui_contribution.ContributionID(contribution.ID)] = struct{}{}
			encoded, err := json.Marshal(contribution.Definition)
			if err != nil {
				return fmt.Errorf("encode builtin ui contribution %s: %w", contribution.ID, err)
			}
			var uiDefinition ui_contribution.UIContributionDefinition
			if err := json.Unmarshal(encoded, &uiDefinition); err != nil {
				return fmt.Errorf("decode builtin ui contribution %s: %w", contribution.ID, err)
			}
			uiDefinition.Schema = normalizeJSONSchemaRaw(uiDefinition.Schema)
			uiDefinition.ContributionID = ui_contribution.ContributionID(contribution.ID)
			uiDefinition.ExtensionID = ui_contribution.ExtensionID(contribution.ExtensionID)
			uiDefinition.ModuleID = ui_contribution.ModuleID(contribution.ModuleID)
			uiDefinition.Integrity.Generation = 0
			if err := repository.PutContribution(ctx, &uiDefinition); err != nil {
				return fmt.Errorf("persist builtin ui contribution %s: %w", contribution.ID, err)
			}
		}
	}
	saved, err := repository.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("list saved builtin ui contributions: %w", err)
	}
	for _, contribution := range saved {
		if contribution == nil || !strings.HasPrefix(string(contribution.ExtensionID), builtin.PrefixBuiltin) {
			continue
		}
		if _, ok := active[contribution.ContributionID]; ok {
			continue
		}
		if err := repository.DeleteContribution(ctx, string(contribution.ContributionID)); err != nil {
			return fmt.Errorf("delete retired builtin ui contribution %s: %w", contribution.ContributionID, err)
		}
	}
	return nil
}
