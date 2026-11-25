package plugincontract

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const bindingFilterGoldenPath = "testdata/binding-filter-matrix.json"

type bindingFilterCase struct {
	Name    string        `json:"name"`
	Profile TargetProfile `json:"profile"`
}

type bindingFilterCaseResult struct {
	Name       string        `json:"name"`
	Profile    TargetProfile `json:"profile"`
	Decision   string        `json:"decision"`
	Candidates []string      `json:"candidates"`
	Gate       string        `json:"gate,omitempty"`
	Detail     string        `json:"detail,omitempty"`
}

type bindingFilterMatrix struct {
	SchemaVersion int                       `json:"schemaVersion"`
	ProviderID    string                    `json:"providerId"`
	Cases         []bindingFilterCaseResult `json:"cases"`
}

func emoteProviderBindings(t *testing.T) []ProviderBinding {
	t.Helper()
	manifest := loadEmoteManifest(t)
	bindings := manifest.ProviderBindings()
	if len(bindings) != 1 {
		t.Fatalf("emote manifest produced %d bindings, want 1", len(bindings))
	}
	return bindings
}

func fullEmotePermissions() []string {
	return []string{
		"message.append",
		"resource.read",
		"resource.write",
		"storage.state.read",
		"storage.state.write",
		"tool.invoke",
	}
}

func emoteFilterCases() []bindingFilterCase {
	all := fullEmotePermissions()
	return []bindingFilterCase{
		{
			Name:    "macos-device-fully-granted",
			Profile: TargetProfile{Platform: "macos", Host: HostKindDevice, GrantedPerms: all, ContractVersion: "0.1.0"},
		},
		{
			Name:    "windows-device-fully-granted",
			Profile: TargetProfile{Platform: "windows", Host: HostKindDevice, GrantedPerms: all, ContractVersion: "0.2.5"},
		},
		{
			Name:    "android-device-higher-contract",
			Profile: TargetProfile{Platform: "android", Host: HostKindDevice, GrantedPerms: all, ContractVersion: "9.9.9"},
		},
		{
			Name:    "ios-device-placement-mismatch",
			Profile: TargetProfile{Platform: "ios", Host: HostKindCore, GrantedPerms: all, ContractVersion: "0.1.0"},
		},
		{
			Name:    "linux-cloud-placement-mismatch",
			Profile: TargetProfile{Platform: "linux", Host: HostKindCloud, GrantedPerms: all, ContractVersion: "0.1.0"},
		},
		{
			Name:    "macos-device-partial-permissions",
			Profile: TargetProfile{Platform: "macos", Host: HostKindDevice, GrantedPerms: []string{"tool.invoke", "resource.read"}, ContractVersion: "0.1.0"},
		},
		{
			Name:    "linux-device-no-permissions",
			Profile: TargetProfile{Platform: "linux", Host: HostKindDevice, ContractVersion: "0.1.0"},
		},
		{
			Name:    "macos-device-contract-too-low",
			Profile: TargetProfile{Platform: "macos", Host: HostKindDevice, GrantedPerms: all, ContractVersion: "0.0.9"},
		},
		{
			Name:    "macos-device-contract-missing",
			Profile: TargetProfile{Platform: "macos", Host: HostKindDevice, GrantedPerms: all},
		},
		{
			Name:    "ios-device-permissions-before-contract",
			Profile: TargetProfile{Platform: "ios", Host: HostKindDevice, ContractVersion: "0.0.1"},
		},
	}
}

func buildBindingFilterMatrix(t *testing.T, bindings []ProviderBinding, cases []bindingFilterCase) bindingFilterMatrix {
	t.Helper()
	matrix := bindingFilterMatrix{SchemaVersion: 1, ProviderID: bindings[0].ProviderID}
	for _, testCase := range cases {
		result, err := FilterProviderBindings(bindings, testCase.Profile)
		if err != nil {
			t.Fatalf("case %s: FilterProviderBindings: %v", testCase.Name, err)
		}
		outcome := bindingFilterCaseResult{
			Name:       testCase.Name,
			Profile:    testCase.Profile,
			Decision:   "candidate",
			Candidates: result.CandidateIDs(),
		}
		if len(result.Rejected) > 0 {
			if len(result.Rejected) != 1 {
				t.Fatalf("case %s: %d rejections, want at most 1", testCase.Name, len(result.Rejected))
			}
			outcome.Decision = "rejected"
			outcome.Gate = string(result.Rejected[0].Gate)
			outcome.Detail = result.Rejected[0].Detail
		}
		matrix.Cases = append(matrix.Cases, outcome)
	}
	return matrix
}

func encodeBindingFilterMatrix(t *testing.T, matrix bindingFilterMatrix) []byte {
	t.Helper()
	encoded, err := json.MarshalIndent(matrix, "", "  ")
	if err != nil {
		t.Fatalf("marshal matrix: %v", err)
	}
	return append(encoded, '\n')
}

func TestFilterProviderBindingsMatchesGoldenMatrix(t *testing.T) {
	bindings := emoteProviderBindings(t)
	matrix := buildBindingFilterMatrix(t, bindings, emoteFilterCases())
	encoded := encodeBindingFilterMatrix(t, matrix)
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(bindingFilterGoldenPath), 0o755); err != nil {
			t.Fatalf("create golden dir: %v", err)
		}
		if err := os.WriteFile(bindingFilterGoldenPath, encoded, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}
	want, err := os.ReadFile(bindingFilterGoldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v", bindingFilterGoldenPath, err)
	}
	if !bytes.Equal(bytes.TrimSpace(want), bytes.TrimSpace(encoded)) {
		t.Fatalf("binding filter matrix drifted from %s\nwant:\n%s\ngot:\n%s", bindingFilterGoldenPath, want, encoded)
	}
}

func TestFilterProviderBindingsReportsEarliestFailingGate(t *testing.T) {
	bindings := emoteProviderBindings(t)
	result, err := FilterProviderBindings(bindings, TargetProfile{
		Platform:        "ios",
		Host:            HostKindCore,
		ContractVersion: "0.0.1",
	})
	if err != nil {
		t.Fatalf("FilterProviderBindings: %v", err)
	}
	if len(result.Candidates) != 0 {
		t.Fatalf("candidates = %v, want none", result.CandidateIDs())
	}
	if len(result.Rejected) != 1 || result.Rejected[0].Gate != BindingGatePlacement {
		t.Fatalf("rejected = %+v, want single placement rejection", result.Rejected)
	}
	if !strings.Contains(result.Rejected[0].Detail, "placement device does not run on host core") {
		t.Fatalf("detail = %q, want placement reason", result.Rejected[0].Detail)
	}
}

func TestFilterProviderBindingsRejectsInvalidProfile(t *testing.T) {
	bindings := emoteProviderBindings(t)
	cases := []struct {
		name    string
		profile TargetProfile
		want    string
	}{
		{name: "empty-platform", profile: TargetProfile{Host: HostKindDevice}, want: "platform is empty"},
		{name: "unknown-platform", profile: TargetProfile{Platform: "darwin", Host: HostKindDevice}, want: "unsupported target platform"},
		{name: "unknown-host", profile: TargetProfile{Platform: "macos", Host: HostKind("browser")}, want: "unsupported target host kind"},
		{name: "bad-contract", profile: TargetProfile{Platform: "macos", Host: HostKindDevice, ContractVersion: "0.x.0"}, want: "non numeric version segment"},
	}
	for _, testCase := range cases {
		if _, err := FilterProviderBindings(bindings, testCase.profile); err == nil {
			t.Fatalf("case %s: expected error", testCase.name)
		} else if !strings.Contains(err.Error(), testCase.want) {
			t.Fatalf("case %s: error %q does not contain %q", testCase.name, err.Error(), testCase.want)
		}
	}
}

func TestFilterProviderBindingsGatePrecedenceOrder(t *testing.T) {
	constrained := newProviderBindingWithRequirements(
		ModuleRef{PluginID: "com.amitia", ModuleID: "strict"},
		ServiceOffer{ServiceID: "com.amitia/strict/ai", Version: "1.0.0", Placement: ServicePlacementDevice},
		true,
		bindingRequirements{
			Platforms:           []string{"linux"},
			RequiredPermissions: []string{"tool.invoke"},
			MinContractVersion:  "2.0.0",
		},
	)
	cases := []struct {
		name    string
		profile TargetProfile
		gate    BindingGate
	}{
		{name: "platform-before-permission", profile: TargetProfile{Platform: "macos", Host: HostKindDevice, ContractVersion: "0.1.0"}, gate: BindingGatePlatform},
		{name: "permission-before-contract", profile: TargetProfile{Platform: "linux", Host: HostKindDevice, ContractVersion: "0.1.0"}, gate: BindingGatePermission},
		{name: "contract-last", profile: TargetProfile{Platform: "linux", Host: HostKindDevice, GrantedPerms: []string{"tool.invoke"}, ContractVersion: "1.9.9"}, gate: BindingGateContract},
	}
	for _, testCase := range cases {
		result, err := FilterProviderBindings([]ProviderBinding{constrained}, testCase.profile)
		if err != nil {
			t.Fatalf("case %s: %v", testCase.name, err)
		}
		if len(result.Candidates) != 0 {
			t.Fatalf("case %s: candidates = %v, want none", testCase.name, result.CandidateIDs())
		}
		if len(result.Rejected) != 1 || result.Rejected[0].Gate != testCase.gate {
			t.Fatalf("case %s: rejected = %+v, want gate %s", testCase.name, result.Rejected, testCase.gate)
		}
	}
	pass, err := FilterProviderBindings([]ProviderBinding{constrained}, TargetProfile{Platform: "linux", Host: HostKindDevice, GrantedPerms: []string{"tool.invoke"}, ContractVersion: "2.0.0"})
	if err != nil {
		t.Fatalf("pass case: %v", err)
	}
	if len(pass.Candidates) != 1 {
		t.Fatalf("pass case: candidates = %v, want 1", pass.CandidateIDs())
	}
}

func TestFilterProviderBindingsKeepsUnconstrainedBinding(t *testing.T) {
	binding := newProviderBinding(
		ModuleRef{PluginID: "com.amitia", ModuleID: "loose"},
		ServiceOffer{ServiceID: "com.amitia/loose/ai", Version: "1.0.0", Placement: ServicePlacementHybrid},
		true,
	)
	for _, profile := range []TargetProfile{
		{Platform: "macos", Host: HostKindCore},
		{Platform: "android", Host: HostKindDevice},
		{Platform: "linux", Host: HostKindCloud, ContractVersion: "0.0.1"},
	} {
		result, err := FilterProviderBindings([]ProviderBinding{binding}, profile)
		if err != nil {
			t.Fatalf("profile %+v: %v", profile, err)
		}
		if len(result.Candidates) != 1 || len(result.Rejected) != 0 {
			t.Fatalf("profile %+v: candidates=%v rejected=%+v, want hybrid binding kept", profile, result.CandidateIDs(), result.Rejected)
		}
	}
}

func TestModuleBindingRequirementsProjectDeclarations(t *testing.T) {
	manifest := loadEmoteManifest(t)
	bindings := manifest.ProviderBindings()
	binding := bindings[0]
	wantPlatforms := []string{"windows", "linux", "macos", "android", "ios"}
	if !reflect.DeepEqual(binding.Platforms, wantPlatforms) {
		t.Fatalf("platforms = %v, want %v", binding.Platforms, wantPlatforms)
	}
	if !reflect.DeepEqual(binding.RequiredPermissions, fullEmotePermissions()) {
		t.Fatalf("required permissions = %v, want %v", binding.RequiredPermissions, fullEmotePermissions())
	}
	if binding.MinContractVersion != "0.1.0" {
		t.Fatalf("min contract version = %q, want 0.1.0", binding.MinContractVersion)
	}

	synthetic := parseSyntheticManifest(t, `{
		"schemaVersion": 1,
		"plugin": {"id": "com.amitia", "name": {"default": "Synthetic"}, "version": "1.0.0", "publisher": {"id": "com.amitia", "displayName": "Amitia"}},
		"compatibility": {"minHostVersion": "0.4.0", "platforms": ["windows"]},
		"modules": [{
			"id": "runtime",
			"name": {"default": "Runtime"},
			"compatibility": {"minHostVersion": "0.7.1", "platforms": ["macos", "macos", " "]},
			"services": {"provides": [{"serviceId": "com.amitia/synthetic/ai", "version": "1.0.0", "placement": "device", "scopes": ["global"]}]}
		}],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "0000000000000000000000000000000000000000000000000000000000000000"}
	}`)
	syntheticBinding := synthetic.ProviderBindings()[0]
	if !reflect.DeepEqual(syntheticBinding.Platforms, []string{"macos"}) {
		t.Fatalf("module platforms = %v, want module level [macos]", syntheticBinding.Platforms)
	}
	if syntheticBinding.MinContractVersion != "0.7.1" {
		t.Fatalf("module min contract version = %q, want module level 0.7.1", syntheticBinding.MinContractVersion)
	}
	if len(syntheticBinding.RequiredPermissions) != 0 {
		t.Fatalf("required permissions = %v, want none", syntheticBinding.RequiredPermissions)
	}
}

func TestFilterProviderBindingsIsolatesInput(t *testing.T) {
	bindings := emoteProviderBindings(t)
	profile := TargetProfile{Platform: "macos", Host: HostKindDevice, GrantedPerms: fullEmotePermissions(), ContractVersion: "0.1.0"}
	result, err := FilterProviderBindings(bindings, profile)
	if err != nil {
		t.Fatalf("FilterProviderBindings: %v", err)
	}
	result.Candidates[0].Platforms[0] = "tampered"
	result.Candidates[0].RequiredPermissions[0] = "tampered"
	if bindings[0].Platforms[0] != "windows" || bindings[0].RequiredPermissions[0] != "message.append" {
		t.Fatalf("source binding mutated: %+v", bindings[0])
	}

	rejected := []ProviderBinding{
		newProviderBinding(ModuleRef{PluginID: "com.amitia", ModuleID: "b"}, ServiceOffer{ServiceID: "com.amitia/b/ai", Placement: ServicePlacementDevice}, true),
		newProviderBinding(ModuleRef{PluginID: "com.amitia", ModuleID: "a"}, ServiceOffer{ServiceID: "com.amitia/a/ai", Placement: ServicePlacementDevice}, true),
	}
	rejected[0].Platforms = []string{"ios"}
	rejected[1].Platforms = []string{"ios"}
	ordered, err := FilterProviderBindings(rejected, TargetProfile{Platform: "macos", Host: HostKindDevice})
	if err != nil {
		t.Fatalf("FilterProviderBindings: %v", err)
	}
	if ordered.Rejected[0].ProviderID >= ordered.Rejected[1].ProviderID {
		t.Fatalf("rejections not sorted by provider id: %+v", ordered.Rejected)
	}
}
