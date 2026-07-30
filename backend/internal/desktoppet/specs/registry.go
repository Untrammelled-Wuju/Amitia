// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package specs

const specVersion = 1

func GetSpec(actionKey string) (ActionGenerationSpec, bool) {
	cs, ok := CatalogGet(actionKey)
	if !ok {
		return ActionGenerationSpec{}, false
	}
	return fromContracts(cs), true
}

func AllSpecs() []ActionGenerationSpec {
	all := CatalogAll()
	out := make([]ActionGenerationSpec, 0, len(all))
	for _, cs := range all {
		out = append(out, fromContracts(cs))
	}
	return out
}

func SpecVersion() int {
	return specVersion
}
