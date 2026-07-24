// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package specs

const specVersion = 1

func GetSpec(actionKey string) (ActionGenerationSpec, bool) {
	spec, ok := defaultSpecs[actionKey]
	if !ok {
		return ActionGenerationSpec{}, false
	}
	return spec, true
}

func AllSpecs() []ActionGenerationSpec {
	out := make([]ActionGenerationSpec, 0, len(defaultSpecs))
	for _, spec := range defaultSpecs {
		out = append(out, spec)
	}
	return out
}

func SpecVersion() int {
	return specVersion
}
