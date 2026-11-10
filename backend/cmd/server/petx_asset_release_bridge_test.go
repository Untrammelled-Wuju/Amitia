package main

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func TestPetxInstallationIdempotencyKeyUsesPackageOperationScope(t *testing.T) {
	installed := domain.ExtensionInstallation{
		InstalledVersion: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 7},
		Metadata: map[string]interface{}{
			"lastOperationId": "package-operation-one",
			"generationId":    "generation-one",
		},
	}
	first := petxInstallationIdempotencyKey("com.example/pet", installed, "content-hash")
	installed.Metadata["lastOperationId"] = "package-operation-two"
	second := petxInstallationIdempotencyKey("com.example/pet", installed, "content-hash")
	if first == second {
		t.Fatal("new package operation reused the previous pet installation idempotency key")
	}
	delete(installed.Metadata, "lastOperationId")
	installed.Metadata["generationId"] = "generation-two"
	third := petxInstallationIdempotencyKey("com.example/pet", installed, "content-hash")
	if third == second {
		t.Fatal("new generation reused the previous pet installation idempotency key")
	}
}
