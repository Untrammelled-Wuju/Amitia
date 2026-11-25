package plugincontract

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type HostKind string

const (
	HostKindCore   HostKind = "core"
	HostKindDevice HostKind = "device"
	HostKindCloud  HostKind = "cloud"
)

type TargetProfile struct {
	Platform        string   `json:"platform"`
	Host            HostKind `json:"host"`
	GrantedPerms    []string `json:"grantedPermissions"`
	ContractVersion string   `json:"contractVersion"`
}

var targetPlatforms = map[string]bool{
	"windows": true,
	"linux":   true,
	"macos":   true,
	"android": true,
	"ios":     true,
}

func (p TargetProfile) Validate() error {
	platform := strings.TrimSpace(p.Platform)
	if platform == "" {
		return fmt.Errorf("plugincontract: target profile platform is empty")
	}
	if !targetPlatforms[platform] {
		return fmt.Errorf("plugincontract: unsupported target platform %q, want one of android, ios, linux, macos, windows", p.Platform)
	}
	switch p.Host {
	case HostKindCore, HostKindDevice, HostKindCloud:
	default:
		return fmt.Errorf("plugincontract: unsupported target host kind %q", p.Host)
	}
	if version := strings.TrimSpace(p.ContractVersion); version != "" {
		if _, err := parseDottedVersion(version); err != nil {
			return fmt.Errorf("plugincontract: target contract version %q: %w", p.ContractVersion, err)
		}
	}
	return nil
}

type BindingGate string

const (
	BindingGatePlacement  BindingGate = "placement"
	BindingGatePlatform   BindingGate = "platform"
	BindingGatePermission BindingGate = "permission"
	BindingGateContract   BindingGate = "contractVersion"
)

var bindingGateOrder = []BindingGate{BindingGatePlacement, BindingGatePlatform, BindingGatePermission, BindingGateContract}

type BindingRejection struct {
	ProviderID string      `json:"providerId"`
	Gate       BindingGate `json:"gate"`
	Detail     string      `json:"detail"`
}

type BindingFilterResult struct {
	Candidates []ProviderBinding  `json:"candidates"`
	Rejected   []BindingRejection `json:"rejected"`
}

func (r BindingFilterResult) CandidateIDs() []string {
	ids := make([]string, 0, len(r.Candidates))
	for _, candidate := range r.Candidates {
		ids = append(ids, candidate.ProviderID)
	}
	return ids
}

func FilterProviderBindings(bindings []ProviderBinding, profile TargetProfile) (BindingFilterResult, error) {
	if err := profile.Validate(); err != nil {
		return BindingFilterResult{}, err
	}
	granted := make(map[string]bool, len(profile.GrantedPerms))
	for _, permission := range profile.GrantedPerms {
		granted[strings.TrimSpace(permission)] = true
	}
	result := BindingFilterResult{Candidates: make([]ProviderBinding, 0, len(bindings)), Rejected: make([]BindingRejection, 0)}
	for _, binding := range bindings {
		if gate, detail := bindingGateFailure(binding, profile, granted); gate != "" {
			result.Rejected = append(result.Rejected, BindingRejection{
				ProviderID: binding.ProviderID,
				Gate:       gate,
				Detail:     detail,
			})
			continue
		}
		result.Candidates = append(result.Candidates, binding.clone())
	}
	sort.SliceStable(result.Rejected, func(i, j int) bool {
		if result.Rejected[i].ProviderID != result.Rejected[j].ProviderID {
			return result.Rejected[i].ProviderID < result.Rejected[j].ProviderID
		}
		return bindingGateRank(result.Rejected[i].Gate) < bindingGateRank(result.Rejected[j].Gate)
	})
	return result, nil
}

func bindingGateRank(gate BindingGate) int {
	for index, candidate := range bindingGateOrder {
		if candidate == gate {
			return index
		}
	}
	return len(bindingGateOrder)
}

func bindingGateFailure(binding ProviderBinding, profile TargetProfile, granted map[string]bool) (BindingGate, string) {
	if gate, detail := placementGate(binding, profile.Host); gate != "" {
		return gate, detail
	}
	if gate, detail := platformGate(binding, profile.Platform); gate != "" {
		return gate, detail
	}
	if gate, detail := permissionGate(binding, granted); gate != "" {
		return gate, detail
	}
	return contractGate(binding, profile.ContractVersion)
}

func placementGate(binding ProviderBinding, host HostKind) (BindingGate, string) {
	if binding.Placement == "" {
		return "", ""
	}
	allowed := false
	switch binding.Placement {
	case ServicePlacementCore:
		allowed = host == HostKindCore
	case ServicePlacementDevice:
		allowed = host == HostKindDevice
	case ServicePlacementCloud:
		allowed = host == HostKindCloud
	default:
		allowed = true
	}
	if allowed {
		return "", ""
	}
	return BindingGatePlacement, fmt.Sprintf("placement %s does not run on host %s", binding.Placement, host)
}

func platformGate(binding ProviderBinding, platform string) (BindingGate, string) {
	if len(binding.Platforms) == 0 {
		return "", ""
	}
	wanted := strings.TrimSpace(platform)
	for _, supported := range binding.Platforms {
		if strings.TrimSpace(supported) == wanted {
			return "", ""
		}
	}
	return BindingGatePlatform, fmt.Sprintf("platform %s not in %s", wanted, strings.Join(binding.Platforms, ","))
}

func permissionGate(binding ProviderBinding, granted map[string]bool) (BindingGate, string) {
	missing := make([]string, 0, len(binding.RequiredPermissions))
	for _, permission := range binding.RequiredPermissions {
		name := strings.TrimSpace(permission)
		if name == "" || granted[name] {
			continue
		}
		missing = append(missing, name)
	}
	if len(missing) == 0 {
		return "", ""
	}
	sort.Strings(missing)
	return BindingGatePermission, fmt.Sprintf("missing required permissions: %s", strings.Join(missing, ","))
}

func contractGate(binding ProviderBinding, hostVersion string) (BindingGate, string) {
	required := strings.TrimSpace(binding.MinContractVersion)
	if required == "" {
		return "", ""
	}
	provided := strings.TrimSpace(hostVersion)
	if provided == "" {
		return BindingGateContract, fmt.Sprintf("host contract version is empty, provider requires %s", required)
	}
	if compareDotted(provided, required) < 0 {
		return BindingGateContract, fmt.Sprintf("host contract version %s is below required %s", provided, required)
	}
	return "", ""
}

func parseDottedVersion(value string) ([]int, error) {
	parts := strings.Split(value, ".")
	numbers := make([]int, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return nil, fmt.Errorf("empty version segment in %q", value)
		}
		number, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("non numeric version segment %q in %q", part, value)
		}
		if number < 0 {
			return nil, fmt.Errorf("negative version segment %q in %q", part, value)
		}
		numbers = append(numbers, number)
	}
	return numbers, nil
}

func compareDotted(left, right string) int {
	leftParts, leftErr := parseDottedVersion(strings.TrimSpace(left))
	rightParts, rightErr := parseDottedVersion(strings.TrimSpace(right))
	if leftErr != nil || rightErr != nil {
		return strings.Compare(strings.TrimSpace(left), strings.TrimSpace(right))
	}
	for index := 0; index < len(leftParts) || index < len(rightParts); index++ {
		leftValue := 0
		if index < len(leftParts) {
			leftValue = leftParts[index]
		}
		rightValue := 0
		if index < len(rightParts) {
			rightValue = rightParts[index]
		}
		if leftValue != rightValue {
			if leftValue < rightValue {
				return -1
			}
			return 1
		}
	}
	return 0
}
