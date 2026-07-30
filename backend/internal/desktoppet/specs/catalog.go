// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package specs

import (
	"sort"
	"sync"

	"github.com/u-ai/backend/internal/desktoppet/contracts"
)

var (
	catalogOnce  sync.Once
	catalogSpecs []contracts.ActionSpec
	catalogIndex map[string]contracts.ActionSpec
)

func initCatalog() {
	catalogOnce.Do(func() {
		all := make([]contracts.ActionSpec, 0, 59)
		all = append(all, idleSpecs()...)
		all = append(all, movementSpecs()...)
		all = append(all, interactionSpecs()...)
		all = append(all, emotionSpecs()...)
		all = append(all, lifeSpecs()...)
		all = append(all, desktopSpecs()...)
		all = append(all, dialogueSpecs()...)

		for i := range all {
			all[i] = contracts.NormalizeSpec(all[i])
		}

		sort.Slice(all, func(i, j int) bool {
			if all[i].Identity.CategorySortOrder != all[j].Identity.CategorySortOrder {
				return all[i].Identity.CategorySortOrder < all[j].Identity.CategorySortOrder
			}
			return all[i].Identity.ActionSortOrder < all[j].Identity.ActionSortOrder
		})

		catalogSpecs = all
		catalogIndex = make(map[string]contracts.ActionSpec, len(all))
		for _, s := range all {
			catalogIndex[s.Identity.Key] = s
		}
	})
}

func CatalogAll() []contracts.ActionSpec {
	initCatalog()
	return catalogSpecs
}

func CatalogGet(key string) (contracts.ActionSpec, bool) {
	initCatalog()
	s, ok := catalogIndex[key]
	return s, ok
}

func CatalogVersion() int {
	return contracts.CatalogVersion
}

func CatalogKeys() []string {
	initCatalog()
	keys := make([]string, 0, len(catalogSpecs))
	for _, s := range catalogSpecs {
		keys = append(keys, s.Identity.Key)
	}
	return keys
}

func CatalogEnabledKeys() []string {
	initCatalog()
	keys := make([]string, 0, len(catalogSpecs))
	for _, s := range catalogSpecs {
		if s.Identity.Enabled {
			keys = append(keys, s.Identity.Key)
		}
	}
	return keys
}

func CatalogValidate() []contracts.ValidationError {
	initCatalog()
	return contracts.ValidateCatalog(catalogSpecs)
}

func CatalogDefaultIdleKeys() []string {
	initCatalog()
	keys := make([]string, 0)
	for _, s := range catalogSpecs {
		if s.Identity.SupportsDefaultIdle && s.Identity.Enabled {
			keys = append(keys, s.Identity.Key)
		}
	}
	return keys
}

func makePhases(descriptions []string) []contracts.FramePhase {
	phases := make([]contracts.FramePhase, len(descriptions))
	for i, d := range descriptions {
		phases[i] = contracts.FramePhase{Index: i, Description: d}
	}
	return phases
}
