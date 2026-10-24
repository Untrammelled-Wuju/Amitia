package acquisition

import (
	"strings"
	"testing"
)

func TestM07AcquisitionSensitivePermissionsNeverAutoApprove(t *testing.T) {
	engine := NewPolicyEngine()
	engine.trustedRegistries["verified.example"] = true
	cases := []struct {
		name        string
		kind        CandidateKind
		method      InstallMethod
		permissions []string
	}{
		{"signed publisher shell", CandidateAgentSkill, InstallSkill, []string{"shell.execute"}},
		{"signed publisher secret", CandidateAgentSkill, InstallSkill, []string{"secret.access"}},
		{"trusted registry device", CandidateMCP, InstallMCP, []string{"device.control.camera"}},
		{"installed skill sensitive", CandidateInstalledExtension, InstallEnableExisting, []string{"filesystem.write"}},
		{"built in sensitive", CandidateBuiltin, InstallEnableExisting, []string{"process.spawn"}},
		{"generated skill no permissions", CandidateGeneratedSkill, InstallGeneratedSkill, nil},
		{"generated skill with permissions", CandidateGeneratedSkill, InstallGeneratedSkill, []string{"network.listen"}},
		{"installed memory deletion", CandidateInstalledExtension, InstallEnableExisting, []string{"memory.delete"}},
		{"installed runtime shell", CandidateInstalledExtension, InstallEnableExisting, []string{"runtime.linux.shell.execute"}},
		{"built in external network", CandidateBuiltin, InstallEnableExisting, []string{"network.request"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			candidate := CapabilityCandidate{Kind: tc.kind, Install: CandidateInstallDescriptor{Method: tc.method}, Permissions: tc.permissions,
				Source: CandidateSource{Publisher: "verified.example", Registry: "verified.example"}, Trust: CandidateTrust{SignatureVerified: true, PublisherVerified: true, Level: TrustTrusted}}
			decision := engine.Evaluate(candidate, AcquisitionRequest{})
			if decision.Action != ActionRequireApproval || len(decision.RequiredApprovals) == 0 {
				t.Fatalf("expected explicit approval, got %#v", decision)
			}
			if tc.kind == CandidateGeneratedSkill && !strings.Contains(decision.RequiredApprovals[0].Reason, "generated skill") {
				t.Fatalf("missing generated skill review reason: %#v", decision)
			}
		})
	}
}

func TestM07AcquisitionKeepsSafeAndDeniedPolicy(t *testing.T) {
	engine := NewPolicyEngine()
	engine.trustedRegistries["verified.example"] = true
	candidate := CapabilityCandidate{Kind: CandidateAgentSkill, Install: CandidateInstallDescriptor{Method: InstallSkill},
		Source: CandidateSource{Publisher: "verified.example"}, Trust: CandidateTrust{SignatureVerified: true, PublisherVerified: true, Level: TrustTrusted}}
	if decision := engine.Evaluate(candidate, AcquisitionRequest{}); decision.Action != ActionAllowAuto {
		t.Fatalf("low risk trusted candidate unexpectedly blocked: %#v", decision)
	}
	candidate.Trust.PublisherVerified = false
	if decision := engine.Evaluate(candidate, AcquisitionRequest{}); decision.Action != ActionDeny {
		t.Fatalf("signature/publisher mismatch must be denied: %#v", decision)
	}
	candidate.Trust.PublisherVerified = true
	candidate.Permissions = []string{"shell.execute"}
	candidate.Trust.Level = TrustBlocked
	if decision := engine.Evaluate(candidate, AcquisitionRequest{}); decision.Action != ActionDeny {
		t.Fatalf("blocked publisher still denied before sensitive approval: %#v", decision)
	}
}
