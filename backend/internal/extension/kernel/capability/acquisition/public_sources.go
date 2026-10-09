package acquisition

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/timeoutpolicy"
)

func discoveryJSON(ctx context.Context, client *http.Client, uri string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Amitia-Capability-Discovery")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("discovery endpoint returned HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (8<<20)+1))
	if err != nil {
		return err
	}
	if len(raw) > 8<<20 {
		return fmt.Errorf("discovery response exceeds size limit")
	}
	return json.Unmarshal(raw, target)
}

func discoveryQuery(request AcquisitionRequest) string {
	if request.Query != "" {
		return strings.TrimSpace(request.Query)
	}
	query := string(request.CapabilityID)
	for _, prefix := range []string{"mcp.server.", "skill.", "plugin.", "extension."} {
		query = strings.TrimPrefix(query, prefix)
	}
	return strings.ReplaceAll(query, ".", " ")
}

type PublicMCPSource struct {
	endpoint string
	client   *http.Client
}

func NewPublicMCPSource(endpoint string) *PublicMCPSource {
	return &PublicMCPSource{endpoint: endpoint, client: timeoutpolicy.Client(&http.Client{Timeout: 20 * time.Second})}
}
func (*PublicMCPSource) ID() string          { return "public_mcp_registry" }
func (*PublicMCPSource) Kind() CandidateKind { return CandidateMCP }

func (s *PublicMCPSource) Search(ctx context.Context, request AcquisitionRequest) ([]CapabilityCandidate, error) {
	query := discoveryQuery(request)
	if query == "" {
		return nil, nil
	}
	uri, err := url.Parse(s.endpoint)
	if err != nil {
		return nil, err
	}
	params := uri.Query()
	params.Set("search", query)
	params.Set("limit", "50")
	params.Set("version", "latest")
	uri.RawQuery = params.Encode()
	var response struct {
		Servers []struct {
			Server struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Version     string `json:"version"`
				Packages    []struct {
					RegistryType         string `json:"registryType"`
					Identifier           string `json:"identifier"`
					Version              string `json:"version"`
					EnvironmentVariables []struct {
						Name       string `json:"name"`
						IsRequired bool   `json:"isRequired"`
						Value      string `json:"value"`
					} `json:"environmentVariables"`
					RuntimeArguments []json.RawMessage `json:"runtimeArguments"`
					PackageArguments []json.RawMessage `json:"packageArguments"`
				} `json:"packages"`
				Remotes []struct {
					Type    string `json:"type"`
					URL     string `json:"url"`
					Headers []struct {
						Name       string `json:"name"`
						IsRequired bool   `json:"isRequired"`
					} `json:"headers"`
				} `json:"remotes"`
			} `json:"server"`
			Meta map[string]struct {
				Status   string `json:"status"`
				IsLatest bool   `json:"isLatest"`
			} `json:"_meta"`
		} `json:"servers"`
	}
	if err := discoveryJSON(ctx, s.client, uri.String(), &response); err != nil {
		return nil, err
	}
	result := []CapabilityCandidate{}
	for _, item := range response.Servers {
		if metadata, exists := item.Meta["io.modelcontextprotocol.registry/official"]; exists && (metadata.Status != "active" || !metadata.IsLatest) {
			continue
		}
		server := item.Server
		if server.Name == "" {
			continue
		}
		base := CapabilityCandidate{Name: server.Name, Description: server.Description, Version: server.Version,
			Kind: CandidateMCP, Capabilities: []capability.CapabilityID{request.CapabilityID},
			Source: CandidateSource{Registry: "modelcontextprotocol", URI: uri.String()}, Trust: CandidateTrust{Level: TrustUnverified}}
		for index, remote := range server.Remotes {
			if remote.Type != "streamable-http" && remote.Type != "sse" {
				continue
			}
			parsed, err := url.Parse(remote.URL)
			if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || strings.Contains(remote.URL, "{") {
				continue
			}
			desc := &MCPInstallDescriptor{ServerName: server.Name, Transport: strings.ReplaceAll(remote.Type, "-", "_"), Command: remote.URL}
			for _, header := range remote.Headers {
				if header.IsRequired {
					desc.RequiredInputs = append(desc.RequiredInputs, "HTTP header "+header.Name)
				}
			}
			candidate := base
			candidate.ID = fmt.Sprintf("registry:%s@%s:remote:%d", server.Name, server.Version, index)
			candidate.Install = CandidateInstallDescriptor{Method: InstallMCP, MCP: desc}
			result = append(result, candidate)
		}
		for index, pkg := range server.Packages {
			desc := &MCPInstallDescriptor{ServerName: server.Name, Transport: "stdio", Env: map[string]string{}}
			if pkg.Version == "" || pkg.Identifier == "" {
				continue
			}
			switch pkg.RegistryType {
			case "npm":
				desc.Command = "npx"
				desc.Args = []string{"-y", pkg.Identifier + "@" + pkg.Version}
			case "pypi":
				desc.Command = "uvx"
				desc.Args = []string{pkg.Identifier + "==" + pkg.Version}
			default:
				continue
			}
			for _, entry := range pkg.EnvironmentVariables {
				if entry.IsRequired && entry.Value == "" {
					desc.RequiredInputs = append(desc.RequiredInputs, "environment "+entry.Name)
				}
				if entry.Value != "" {
					desc.Env[entry.Name] = entry.Value
				}
			}
			if len(pkg.RuntimeArguments) > 0 {
				desc.RequiredInputs = append(desc.RequiredInputs, "registry runtime arguments must be configured from server metadata")
			}
			for _, raw := range pkg.PackageArguments {
				var argument struct {
					Type       string `json:"type"`
					Name       string `json:"name"`
					Value      string `json:"value"`
					ValueHint  string `json:"valueHint"`
					IsRequired bool   `json:"isRequired"`
				}
				if err := json.Unmarshal(raw, &argument); err != nil {
					return nil, err
				}
				if argument.Value == "" || strings.Contains(argument.Value, "{") {
					if argument.IsRequired || argument.Value != "" {
						desc.RequiredInputs = append(desc.RequiredInputs, "package argument "+firstNonEmpty(argument.Name, argument.ValueHint, "value"))
					}
					continue
				}
				if argument.Type == "named" && argument.Name != "" {
					desc.Args = append(desc.Args, argument.Name)
				}
				desc.Args = append(desc.Args, argument.Value)
			}
			candidate := base
			candidate.ID = fmt.Sprintf("registry:%s@%s:package:%d", server.Name, server.Version, index)
			candidate.Install = CandidateInstallDescriptor{Method: InstallMCP, MCP: desc}
			result = append(result, candidate)
		}
	}
	return result, nil
}

type PublicSkillSource struct {
	endpoint string
	client   *http.Client
}

func NewPublicSkillSource() *PublicSkillSource {
	return &PublicSkillSource{endpoint: "https://api.github.com", client: timeoutpolicy.Client(&http.Client{Timeout: 20 * time.Second})}
}
func (*PublicSkillSource) ID() string          { return "github_skills" }
func (*PublicSkillSource) Kind() CandidateKind { return CandidateAgentSkill }

func (s *PublicSkillSource) Search(ctx context.Context, request AcquisitionRequest) ([]CapabilityCandidate, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	query := discoveryQuery(request)
	if query == "" {
		return nil, nil
	}
	var repos struct {
		Items []struct {
			FullName      string `json:"full_name"`
			DefaultBranch string `json:"default_branch"`
		} `json:"items"`
	}
	searchErr := discoveryJSON(ctx, s.client, s.endpoint+"/search/repositories?per_page=3&q="+url.QueryEscape(query+" agent skills"), &repos)
	repositories := []struct{ Name, Ref string }{{"openai/skills", "main"}}
	for _, repo := range repos.Items {
		repositories = append(repositories, struct{ Name, Ref string }{repo.FullName, repo.DefaultBranch})
	}
	result := []CapabilityCandidate{}
	var lastErr error
	for _, repo := range repositories {
		var commit struct {
			SHA string `json:"sha"`
		}
		if err := discoveryJSON(ctx, s.client, s.endpoint+"/repos/"+repo.Name+"/commits/"+url.PathEscape(repo.Ref), &commit); err != nil {
			lastErr = err
			continue
		}
		if commit.SHA == "" {
			lastErr = fmt.Errorf("GitHub skill commit is missing")
			continue
		}
		var tree struct {
			SHA       string `json:"sha"`
			Truncated bool   `json:"truncated"`
			Tree      []struct {
				Path string `json:"path"`
				Type string `json:"type"`
			} `json:"tree"`
		}
		if err := discoveryJSON(ctx, s.client, s.endpoint+"/repos/"+repo.Name+"/git/trees/"+url.PathEscape(commit.SHA)+"?recursive=1", &tree); err != nil {
			lastErr = err
			continue
		}
		count := 0
		for _, file := range tree.Tree {
			if file.Type != "blob" || !strings.HasSuffix(file.Path, "/SKILL.md") {
				continue
			}
			directory := strings.TrimSuffix(file.Path, "/SKILL.md")
			matches := false
			for _, word := range strings.Fields(strings.ToLower(query)) {
				if strings.Contains(strings.ToLower(directory), word) {
					matches = true
				}
			}
			if !matches {
				continue
			}
			name := directory[strings.LastIndex(directory, "/")+1:]
			sourceURI := "https://github.com/" + repo.Name + "/tree/" + commit.SHA + "/" + directory
			result = append(result, CapabilityCandidate{ID: "github:" + repo.Name + "/" + directory + "@" + commit.SHA,
				Kind: CandidateAgentSkill, Name: name, Description: "SKILL.md and supporting files from " + repo.Name + "/" + directory,
				Version: commit.SHA, Capabilities: []capability.CapabilityID{request.CapabilityID},
				Source: CandidateSource{Registry: "github", URI: sourceURI, Publisher: repo.Name}, Trust: CandidateTrust{Level: TrustUnverified},
				Install: CandidateInstallDescriptor{Method: InstallSkill, Skill: &SkillInstallDescriptor{SourceURI: sourceURI, SkillName: name}},
			})
			count++
			if count >= 5 || len(result) >= 15 {
				break
			}
		}
	}
	if len(result) == 0 {
		if searchErr != nil {
			return nil, searchErr
		}
		if lastErr != nil {
			return nil, lastErr
		}
	}
	return result, nil
}

type PublicPackageSource struct {
	endpoint string
	client   *http.Client
}

func NewPublicPackageSource() *PublicPackageSource {
	return &PublicPackageSource{endpoint: "https://api.github.com", client: timeoutpolicy.Client(&http.Client{Timeout: 15 * time.Second})}
}
func (*PublicPackageSource) ID() string          { return "github_amitia_packages" }
func (*PublicPackageSource) Kind() CandidateKind { return CandidateExtensionPackage }

func (s *PublicPackageSource) Search(ctx context.Context, request AcquisitionRequest) ([]CapabilityCandidate, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	query := discoveryQuery(request)
	if query == "" {
		return nil, nil
	}
	var repositories struct {
		Items []struct {
			FullName string `json:"full_name"`
		} `json:"items"`
	}
	if err := discoveryJSON(ctx, s.client, s.endpoint+"/search/repositories?per_page=3&q="+url.QueryEscape("amitia "+query), &repositories); err != nil {
		return nil, err
	}
	result := []CapabilityCandidate{}
	for _, repository := range repositories.Items {
		var release struct {
			TagName string `json:"tag_name"`
			Assets  []struct {
				Name string `json:"name"`
				URL  string `json:"browser_download_url"`
			} `json:"assets"`
		}
		if err := discoveryJSON(ctx, s.client, s.endpoint+"/repos/"+repository.FullName+"/releases/latest", &release); err != nil {
			continue
		}
		var document struct {
			Encoding string `json:"encoding"`
			Content  string `json:"content"`
		}
		if err := discoveryJSON(ctx, s.client, s.endpoint+"/repos/"+repository.FullName+"/contents/manifest.json?ref="+url.QueryEscape(release.TagName), &document); err != nil {
			continue
		}
		if document.Encoding != "base64" {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(document.Content, "\n", ""))
		if err != nil {
			continue
		}
		var manifest packageDiscoveryManifest
		if err := json.Unmarshal(raw, &manifest); err != nil || manifest.ManifestVersion < 1 || manifest.Extension.ID == "" || manifest.Extension.Version == "" {
			continue
		}
		for _, asset := range release.Assets {
			if !strings.HasSuffix(strings.ToLower(asset.Name), ".amitiax") {
				continue
			}
			uri, err := url.Parse(asset.URL)
			if err != nil || uri.Scheme != "https" || uri.Hostname() != "github.com" {
				continue
			}
			candidate := packageDiscoveryCandidate(manifest, asset.URL, request)
			candidate.Source.Registry = "github"
			candidate.Source.Publisher = repository.FullName
			result = append(result, candidate)
		}
	}
	return result, nil
}
