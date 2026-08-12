// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package embedding

import (
	"fmt"

	"github.com/u-ai/backend/internal/localmodel/llamacpp"
)

func init() {
	registerLocalEmbeddingProvider("llama_cpp", func(configJSON string) (localEmbeddingProvider, error) {
		factory, ok := llamacpp.GetLocalEmbeddingFactory("llama_cpp")
		if !ok {
			return nil, fmt.Errorf("llama_cpp embedding factory not registered")
		}
		return factory(configJSON)
	})
}
