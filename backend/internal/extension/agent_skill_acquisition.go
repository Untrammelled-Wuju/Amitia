package extension

import (
	"context"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/capability/acquisition"
)

type AgentSkillAcquisitionAdapter struct{ service *AgentSkillService }

func NewAgentSkillAcquisitionAdapter(service *AgentSkillService) *AgentSkillAcquisitionAdapter {
	return &AgentSkillAcquisitionAdapter{service: service}
}

func (a *AgentSkillAcquisitionAdapter) ImportSkill(context.Context, string, string, string) (string, error) {
	return "", fmt.Errorf("skill installation requires an owning space")
}

func (a *AgentSkillAcquisitionAdapter) RemoveSkill(context.Context, string) error {
	return fmt.Errorf("skill removal requires an owning space")
}

func (a *AgentSkillAcquisitionAdapter) ImportSkillForSpace(ctx context.Context, spaceID, sourceURI, name, hash string) (string, error) {
	if spaceID == "" {
		return "", fmt.Errorf("skill installation requires an owning space")
	}
	if strings.HasPrefix(sourceURI, "amitia-skill:") {
		id := strings.TrimPrefix(sourceURI, "amitia-skill:")
		if err := a.service.Enable(ctx, ExecutionScope{SpaceID: spaceID}, id); err != nil {
			return "", err
		}
		return id, nil
	}
	bundle, err := acquisition.LoadSkillBundle(ctx, sourceURI, name, hash)
	if err != nil {
		return "", err
	}
	var preview AgentSkillImportPreview
	if bundle.Archive != nil {
		preview, err = a.service.PreviewZIP(ctx, spaceID, bundle.Archive)
	} else {
		preview, err = a.service.PreviewDirectory(ctx, spaceID, bundle.Root, bundle.Files)
	}
	if err != nil {
		return "", err
	}
	if name != "" && preview.Definition.Name != name {
		return "", fmt.Errorf("skill name does not match selected source")
	}
	installed, err := a.service.Install(ctx, InstallAgentSkillRequest{SpaceID: spaceID, PreviewID: preview.PreviewID, Enable: true})
	if err != nil {
		return "", err
	}
	return installed.ExtensionID, nil
}

func (a *AgentSkillAcquisitionAdapter) RemoveSkillForSpace(ctx context.Context, spaceID, id string) error {
	return a.service.Remove(ctx, ExecutionScope{SpaceID: spaceID}, id)
}

func (a *AgentSkillAcquisitionAdapter) VerifyInstalledCapability(ctx context.Context, installed acquisition.InstalledCapability) (bool, error) {
	id, _ := installed.Candidate.Metadata["skillId"].(string)
	if id == "" {
		return false, fmt.Errorf("installed skill identifier is missing")
	}
	definition, _, err := a.service.Get(ctx, ExecutionScope{SpaceID: string(installed.Target.SpaceID)}, id)
	if err != nil {
		return false, err
	}
	return definition.Enabled && definition.CompatibilityStatus != AgentSkillBlocked, nil
}

func (*AgentSkillAcquisitionAdapter) ID() string { return "persistent_skills" }
func (*AgentSkillAcquisitionAdapter) Kind() acquisition.CandidateKind {
	return acquisition.CandidateAgentSkill
}

func (a *AgentSkillAcquisitionAdapter) Search(ctx context.Context, request acquisition.AcquisitionRequest) ([]acquisition.CapabilityCandidate, error) {
	catalog := PagedAgentSkills{}
	for page := 1; ; page++ {
		batch, err := a.service.List(ctx, ExecutionScope{SpaceID: string(request.SpaceID)}, AgentSkillFilter{Page: page, PageSize: 100})
		if err != nil {
			return nil, err
		}
		catalog.Items = append(catalog.Items, batch.Items...)
		if int64(len(catalog.Items)) >= batch.Total || len(batch.Items) == 0 {
			break
		}
	}
	query := request.Query
	if query == "" {
		query = strings.TrimPrefix(string(request.CapabilityID), "skill.")
	}
	result := []acquisition.CapabilityCandidate{}
	for _, item := range catalog.Items {
		if item.CompatibilityStatus == AgentSkillBlocked {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(item.Name+" "+item.Description), strings.ToLower(query)) {
			continue
		}
		result = append(result, acquisition.CapabilityCandidate{ID: item.ExtensionID, ExtensionID: item.ExtensionID,
			Name: item.Name, Description: item.Description, Kind: acquisition.CandidateAgentSkill,
			Capabilities: []capability.CapabilityID{request.CapabilityID}, Trust: acquisition.CandidateTrust{Level: acquisition.TrustUnverified},
			Source:  acquisition.CandidateSource{URI: "amitia-skill:" + item.ExtensionID, Registry: "persistent_skills"},
			Install: acquisition.CandidateInstallDescriptor{Method: acquisition.InstallSkill, Skill: &acquisition.SkillInstallDescriptor{SourceURI: "amitia-skill:" + item.ExtensionID, SkillName: item.Name}},
		})
	}
	return result, nil
}
