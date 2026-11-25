package plugincontract

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	UIHostWeb     = "web"
	UIHostFlutter = "flutter"
)

type UIHostCall struct {
	Host     string `json:"host"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Endpoint string `json:"endpoint"`
}

type UIHostCoverage struct {
	Host          string   `json:"host"`
	Files         int      `json:"files"`
	Calls         int      `json:"calls"`
	Linked        int      `json:"linked"`
	Prefixes      int      `json:"prefixes"`
	Unlinked      int      `json:"unlinked"`
	UnlinkedPaths []string `json:"unlinkedPaths"`
	Digest        string   `json:"digest"`
}

type uiHostRoot struct {
	host       string
	directory  string
	extensions map[string]bool
}

var uiHostRoots = []uiHostRoot{
	{host: UIHostWeb, directory: "front/src", extensions: map[string]bool{".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".vue": true}},
	{host: UIHostFlutter, directory: "mobile_app/lib", extensions: map[string]bool{".dart": true}},
}

var uiEndpointPattern = regexp.MustCompile("[\"'`](/(?:api|internal)/[^\"'`\n\\\\]*)")

var uiSkipDirs = map[string]bool{
	"node_modules": true,
	"dist":         true,
	"build":        true,
	"testdata":     true,
	".git":         true,
	"third_party":  true,
	"ephemeral":    true,
}

func LoadUIHostCalls(root string) ([]UIHostCall, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("plugincontract: resolve ui host root %q: %w", root, err)
	}
	calls := make([]UIHostCall, 0, 256)
	for _, hostRoot := range uiHostRoots {
		directory := filepath.Join(absolute, filepath.FromSlash(hostRoot.directory))
		info, err := os.Stat(directory)
		if err != nil {
			return nil, fmt.Errorf("plugincontract: stat ui host %s root %q: %w", hostRoot.host, directory, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("plugincontract: ui host %s root %q is not a directory", hostRoot.host, directory)
		}
		err = filepath.WalkDir(directory, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if path != directory && (uiSkipDirs[entry.Name()] || strings.HasPrefix(entry.Name(), ".")) {
					return filepath.SkipDir
				}
				return nil
			}
			if !hostRoot.extensions[strings.ToLower(filepath.Ext(entry.Name()))] {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(absolute, path)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			lines := strings.Split(string(raw), "\n")
			for index, line := range lines {
				for _, match := range uiEndpointPattern.FindAllStringSubmatch(line, -1) {
					endpoint := normalizeUIEndpoint(match[1])
					if endpoint == "" {
						continue
					}
					calls = append(calls, UIHostCall{Host: hostRoot.host, File: relative, Line: index + 1, Endpoint: endpoint})
				}
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("plugincontract: scan ui host %s: %w", hostRoot.host, err)
		}
	}
	return calls, nil
}

func BuildUIHostCoverage(calls []UIHostCall, routes []RouteRef) []UIHostCoverage {
	byHost := map[string][]UIHostCall{}
	for _, call := range calls {
		byHost[call.Host] = append(byHost[call.Host], call)
	}
	coverage := make([]UIHostCoverage, 0, len(uiHostRoots))
	for _, hostRoot := range uiHostRoots {
		hostCalls := byHost[hostRoot.host]
		entry := UIHostCoverage{Host: hostRoot.host, Calls: len(hostCalls)}
		files := map[string]bool{}
		unlinked := map[string]bool{}
		lines := make([]string, 0, len(hostCalls))
		for _, call := range hostCalls {
			files[call.File] = true
			lines = append(lines, call.File+"|"+strconv.Itoa(call.Line)+"|"+call.Endpoint)
			switch {
			case matchesAnyRoute(call.Endpoint, routes):
				entry.Linked++
			case prefixesAnyRoute(call.Endpoint, routes):
				entry.Prefixes++
			default:
				entry.Unlinked++
				unlinked[call.Endpoint] = true
			}
		}
		entry.Files = len(files)
		entry.UnlinkedPaths = make([]string, 0, len(unlinked))
		for path := range unlinked {
			entry.UnlinkedPaths = append(entry.UnlinkedPaths, path)
		}
		sort.Strings(entry.UnlinkedPaths)
		sort.Strings(lines)
		sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
		entry.Digest = hex.EncodeToString(sum[:])
		coverage = append(coverage, entry)
	}
	return coverage
}

func normalizeUIEndpoint(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if index := strings.IndexAny(trimmed, "?#"); index >= 0 {
		trimmed = trimmed[:index]
	}
	for pass := 0; pass < 16; pass++ {
		start := strings.Index(trimmed, "${")
		if start >= 0 && !strings.Contains(trimmed[start:], "}") {
			trimmed = trimmed[:start]
			break
		}
		break
	}
	for passes := 0; passes < 16; passes++ {
		start := strings.Index(trimmed, "${")
		if start < 0 {
			break
		}
		end := strings.Index(trimmed[start:], "}")
		if end < 0 {
			break
		}
		trimmed = trimmed[:start] + ":param" + trimmed[start+end+1:]
	}
	for passes := 0; passes < 16; passes++ {
		position := strings.Index(trimmed, "$")
		if position < 0 || position == len(trimmed)-1 {
			break
		}
		end := position + 1
		for end < len(trimmed) && isIdentifierByte(trimmed[end]) {
			end++
		}
		if end == position+1 {
			break
		}
		trimmed = trimmed[:position] + ":param" + trimmed[end:]
	}
	trimmed = strings.TrimSpace(strings.TrimRight(trimmed, "."))
	for strings.Contains(trimmed, "//") {
		trimmed = strings.ReplaceAll(trimmed, "//", "/")
	}
	if len(trimmed) > 1 {
		trimmed = strings.TrimSuffix(trimmed, "/")
	}
	if trimmed == "/api" || trimmed == "/internal" {
		return ""
	}
	if strings.ContainsAny(trimmed, "[]()<>|\\") {
		return ""
	}
	for _, segment := range strings.Split(trimmed, "/") {
		if strings.HasPrefix(segment, "^") || strings.Contains(segment, "+") {
			return ""
		}
	}
	return trimmed
}

func isIdentifierByte(value byte) bool {
	return value == '_' || (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') || (value >= '0' && value <= '9')
}

func prefixesAnyRoute(endpoint string, routes []RouteRef) bool {
	trimmed := strings.Trim(endpoint, "/")
	for _, route := range routes {
		if strings.HasPrefix(strings.Trim(route.Path, "/"), trimmed+"/") {
			return true
		}
	}
	return false
}

func matchesAnyRoute(endpoint string, routes []RouteRef) bool {
	for _, route := range routes {
		if routeMatchesEndpoint(route.Path, endpoint) {
			return true
		}
	}
	return false
}

func routeMatchesEndpoint(routePath, endpoint string) bool {
	routeSegments := splitPathSegments(routePath)
	endpointSegments := splitPathSegments(endpoint)
	if len(routeSegments) == 0 || len(endpointSegments) == 0 {
		return false
	}
	for index := 0; index < len(routeSegments); index++ {
		if index >= len(endpointSegments) {
			return false
		}
		segment := routeSegments[index]
		if strings.HasPrefix(segment, "*") {
			return true
		}
		if isWildcardSegment(segment) || isWildcardSegment(endpointSegments[index]) {
			continue
		}
		if segment != endpointSegments[index] {
			return false
		}
	}
	return len(routeSegments) == len(endpointSegments)
}

func splitPathSegments(path string) []string {
	trimmed := strings.TrimSpace(path)
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func isWildcardSegment(segment string) bool {
	if segment == "" {
		return false
	}
	if strings.HasPrefix(segment, ":") || strings.HasPrefix(segment, "*") {
		return true
	}
	return strings.Contains(segment, ":param")
}
