package acquisition

import "testing"

func TestM07ApprovalFingerprintChangesOnCandidatePermissionAndSource(t *testing.T) {
	original := CapabilityCandidate{
		ID: "skill-a", Kind: CandidateGeneratedSkill, Version: "1",
		Source:      CandidateSource{Publisher: "publisher", URI: "source-a"},
		Trust:       CandidateTrust{PublisherVerified: true, SignatureVerified: true},
		Install:     CandidateInstallDescriptor{Method: InstallGeneratedSkill, GeneratedSkill: &GeneratedSkillDescriptor{PromptStub: "original", ToolsNeeded: []string{"read"}}},
		Permissions: []string{"memory.read"},
	}
	initial := acquisitionCandidateFingerprint(original)
	if initial == "" || initial != acquisitionCandidateFingerprint(original) {
		t.Fatal("candidate evidence hash must be stable")
	}
	mutated := original
	mutated.Permissions = []string{"shell.execute"}
	if initial == acquisitionCandidateFingerprint(mutated) {
		t.Fatal("new permission must invalidate prior approval")
	}
	mutated = original
	mutated.Source.URI = "different-source"
	if initial == acquisitionCandidateFingerprint(mutated) {
		t.Fatal("a changed installation source must invalidate prior approval")
	}
	mutated = original
	mutated.Install.GeneratedSkill = &GeneratedSkillDescriptor{PromptStub: "new code", ToolsNeeded: []string{"read"}}
	if initial == acquisitionCandidateFingerprint(mutated) {
		t.Fatal("regenerated skill contents must require review")
	}
}
