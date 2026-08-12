// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package modelprotocol

type ProviderConfig struct {
	ModelName        string
	BaseURL          string
	APIKey           string
	Temperature      float64
	TopP             float64
	TimeoutSeconds   int
	MaxTokens        int
	MaxOutputTokens  int
	ContextWindow    int
	Protocol         string
	CapabilitiesJSON string
}
