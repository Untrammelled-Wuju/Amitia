package acquisition

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/agent_skill"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type LocalSkillSource struct{ roots []string }

func NewLocalSkillSource(roots ...string) *LocalSkillSource { return &LocalSkillSource{roots: roots} }
func (*LocalSkillSource) ID() string                        { return "local_skill_directories" }
func (*LocalSkillSource) Kind() CandidateKind               { return CandidateAgentSkill }

func (s *LocalSkillSource) Search(ctx context.Context, request AcquisitionRequest) ([]CapabilityCandidate, error) {
	result := []CapabilityCandidate{}
	query := strings.ToLower(discoveryQuery(request))
	for _, root := range s.roots {
		err := filepath.WalkDir(root, func(name string, entry os.DirEntry, walkErr error) error {
			if os.IsNotExist(walkErr) {
				return filepath.SkipDir
			}
			if walkErr != nil {
				return walkErr
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if entry.Type()&os.ModeSymlink != 0 {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.IsDir() {
				relative, _ := filepath.Rel(root, name)
				if len(strings.Split(relative, string(filepath.Separator))) > 3 {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.Name() != "SKILL.md" {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if info.Size() > 1<<20 {
				return nil
			}
			raw, err := os.ReadFile(name)
			if err != nil {
				return err
			}
			parsed, err := agent_skill.NewSkillParser().Parse(raw)
			if err != nil {
				return nil
			}
			skillName, _ := parsed.Fields["name"].(string)
			description, _ := parsed.Fields["description"].(string)
			if skillName == "" || !strings.Contains(strings.ToLower(skillName+" "+description), query) {
				return nil
			}
			result = append(result, CapabilityCandidate{ID: "local-skill:" + name, Name: skillName, Description: description,
				Kind: CandidateAgentSkill, Capabilities: []capability.CapabilityID{request.CapabilityID},
				Source: CandidateSource{Registry: "local", URI: filepath.Dir(name)}, Trust: CandidateTrust{Level: TrustUnverified},
				Install: CandidateInstallDescriptor{Method: InstallSkill, Skill: &SkillInstallDescriptor{SourceURI: filepath.Dir(name), SkillName: skillName}},
			})
			if len(result) >= 100 {
				return filepath.SkipAll
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

type packageDiscoveryManifest struct {
	ManifestVersion int `json:"manifestVersion"`
	Extension       struct {
		ID          string            `json:"id"`
		Version     string            `json:"version"`
		Name        map[string]string `json:"name"`
		Description map[string]string `json:"description"`
	} `json:"extension"`
}

func readPackageDiscoveryManifest(reader *zip.Reader) (packageDiscoveryManifest, error) {
	var manifest packageDiscoveryManifest
	for _, entry := range reader.File {
		if entry.Name != "manifest.json" {
			continue
		}
		if entry.UncompressedSize64 > 1<<20 {
			return manifest, fmt.Errorf("package manifest is too large")
		}
		file, err := entry.Open()
		if err != nil {
			return manifest, err
		}
		raw, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
		file.Close()
		if err != nil {
			return manifest, err
		}
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return manifest, err
		}
		if manifest.ManifestVersion < 1 || manifest.Extension.ID == "" || manifest.Extension.Version == "" {
			return manifest, fmt.Errorf("not an Amitia package manifest")
		}
		return manifest, nil
	}
	return manifest, fmt.Errorf("Amitia manifest.json is missing")
}

func packageDiscoveryCandidate(manifest packageDiscoveryManifest, uri string, request AcquisitionRequest) CapabilityCandidate {
	name := manifest.Extension.Name["default"]
	if name == "" {
		name = manifest.Extension.ID
	}
	return CapabilityCandidate{ID: "package:" + manifest.Extension.ID + "@" + manifest.Extension.Version, ExtensionID: manifest.Extension.ID,
		PackageName: manifest.Extension.ID, Name: name, Description: manifest.Extension.Description["default"], Version: manifest.Extension.Version,
		Kind: CandidateExtensionPackage, Capabilities: []capability.CapabilityID{request.CapabilityID}, Source: CandidateSource{URI: uri}, Trust: CandidateTrust{Level: TrustUnverified},
		Install: CandidateInstallDescriptor{Method: InstallExtension, ExtensionPackage: &ExtensionInstallDescriptor{PackageURI: uri}}, Metadata: map[string]any{"installOnly": true},
	}
}

type LocalPackageSource struct{ roots []string }

func NewLocalPackageSource(roots ...string) *LocalPackageSource {
	return &LocalPackageSource{roots: roots}
}
func (*LocalPackageSource) ID() string          { return "local_package_archives" }
func (*LocalPackageSource) Kind() CandidateKind { return CandidateExtensionPackage }

func (s *LocalPackageSource) Search(ctx context.Context, request AcquisitionRequest) ([]CapabilityCandidate, error) {
	result := []CapabilityCandidate{}
	for _, root := range s.roots {
		files, err := os.ReadDir(root)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if file.IsDir() || file.Type()&os.ModeSymlink != 0 || !strings.EqualFold(filepath.Ext(file.Name()), ".amitiax") {
				continue
			}
			archivePath := filepath.Join(root, file.Name())
			reader, err := zip.OpenReader(archivePath)
			if err != nil {
				continue
			}
			manifest, err := readPackageDiscoveryManifest(&reader.Reader)
			reader.Close()
			if err != nil {
				continue
			}
			candidate := packageDiscoveryCandidate(manifest, archivePath, request)
			if !strings.Contains(strings.ToLower(candidate.Name+" "+candidate.Description+" "+candidate.ExtensionID), strings.ToLower(discoveryQuery(request))) {
				continue
			}
			result = append(result, candidate)
		}
	}
	return result, nil
}
