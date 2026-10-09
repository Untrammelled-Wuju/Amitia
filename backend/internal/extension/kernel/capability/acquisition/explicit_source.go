package acquisition

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type ExplicitSource struct{}

func NewExplicitSource() *ExplicitSource    { return &ExplicitSource{} }
func (*ExplicitSource) ID() string          { return "explicit_source" }
func (*ExplicitSource) Kind() CandidateKind { return CandidateBuiltin }

func normalizeCandidateKind(value string) CandidateKind {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "skill", "skills", "agent_skill":
		return CandidateAgentSkill
	case "plugin", "extension", "extension_package":
		return CandidateExtensionPackage
	default:
		return CandidateKind(value)
	}
}

func (*ExplicitSource) Search(ctx context.Context, request AcquisitionRequest) ([]CapabilityCandidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if request.SourceURI == "" && request.Install == nil {
		return nil, nil
	}
	install := CandidateInstallDescriptor{}
	if request.Install != nil {
		install = *request.Install
	} else {
		kind := CandidateAgentSkill
		if len(request.PreferredKinds) > 0 {
			kind = request.PreferredKinds[0]
		}
		switch kind {
		case CandidateAgentSkill:
			install.Method = InstallSkill
			install.Skill = &SkillInstallDescriptor{SourceURI: request.SourceURI}
		case CandidateExtensionPackage:
			install.Method = InstallExtension
			install.ExtensionPackage = &ExtensionInstallDescriptor{PackageURI: request.SourceURI}
		default:
			return nil, fmt.Errorf("explicit MCP source requires a verified mcp install descriptor")
		}
	}
	candidate := CapabilityCandidate{ExtensionID: request.ExtensionID, Version: request.Version, Name: request.Query,
		Capabilities: []capability.CapabilityID{request.CapabilityID}, Install: install,
		Source: CandidateSource{URI: request.SourceURI, Registry: "explicit"}, Trust: CandidateTrust{Level: TrustUnverified}}
	switch install.Method {
	case InstallSkill:
		if install.Skill == nil || install.Skill.SourceURI == "" {
			return nil, fmt.Errorf("skill sourceUri is required")
		}
		candidate.Kind = CandidateAgentSkill
		candidate.Source.URI = install.Skill.SourceURI
		if candidate.Name == "" {
			candidate.Name = install.Skill.SkillName
		}
	case InstallMCP:
		if install.MCP == nil || install.MCP.ServerName == "" || install.MCP.Command == "" {
			return nil, fmt.Errorf("MCP serverName, transport and command or remote endpoint are required")
		}
		switch install.MCP.Transport {
		case "stdio", "executable", "streamable_http", "sse", "remote":
		default:
			return nil, fmt.Errorf("unsupported MCP transport %q", install.MCP.Transport)
		}
		candidate.Kind = CandidateMCP
		candidate.Name = install.MCP.ServerName
	case InstallExtension:
		if install.ExtensionPackage == nil || install.ExtensionPackage.PackageURI == "" || request.ExtensionID == "" {
			return nil, fmt.Errorf("Amitia package URI and manifest extensionId are required; other products' plugins are not Amitia packages")
		}
		candidate.Kind = CandidateExtensionPackage
		candidate.PackageName = request.ExtensionID
		candidate.Source.URI = install.ExtensionPackage.PackageURI
		candidate.Metadata = map[string]any{"installOnly": true}
	default:
		return nil, fmt.Errorf("unsupported explicit install method %q", install.Method)
	}
	raw, _ := json.Marshal(install)
	digest := sha256.Sum256(raw)
	candidate.ID = "source:" + hex.EncodeToString(digest[:12])
	if request.RequestedCandidateID != "" {
		candidate.ID = request.RequestedCandidateID
	}
	return []CapabilityCandidate{candidate}, nil
}
