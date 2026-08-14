package capability

import (
	"fmt"
	"regexp"
	"strings"
)

var asciiOnlyPattern = regexp.MustCompile(`[^a-z0-9/._-]`)

func ParseCapabilityID(raw string) CapabilityID {
	return CapabilityID(strings.TrimSpace(raw))
}

func (id CapabilityID) IsEmpty() bool {
	return strings.TrimSpace(string(id)) == ""
}

func BuildCapabilityID(source CapabilitySource, namespace, name string) string {
	raw := strings.ToLower(string(source) + "/" + namespace + "/" + name)
	raw = asciiOnlyPattern.ReplaceAllString(raw, "-")
	raw = strings.Trim(raw, "-")
	return raw
}

func ModelNameFromCapabilityID(id CapabilityID) string {
	parts := strings.SplitN(string(id), "/", 3)
	if len(parts) != 3 {
		raw := strings.ToLower(string(id))
		raw = strings.ReplaceAll(raw, ".", "_")
		raw = strings.ReplaceAll(raw, "-", "_")
		raw = asciiOnlyPattern.ReplaceAllString(raw, "_")
		return strings.Trim(raw, "_")
	}
	source := parts[0]
	namespace := parts[1]
	name := parts[2]

	namespace = strings.ReplaceAll(namespace, ".", "_")
	namespace = strings.ReplaceAll(namespace, "-", "_")
	namespace = asciiOnlyPattern.ReplaceAllString(namespace, "_")
	namespace = strings.Trim(namespace, "_")

	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, "-", "_")
	name = asciiOnlyPattern.ReplaceAllString(name, "_")
	name = strings.Trim(name, "_")

	prefix := source
	if namespace != "" {
		prefix = source + "_" + namespace
	}

	return prefix + "__" + name
}

func ResolveModelNameConflicts(modelNames map[string]string, id CapabilityID) string {
	base := ModelNameFromCapabilityID(id)
	resolved := base
	suffix := 2
	for {
		existing, ok := modelNames[resolved]
		if !ok || existing == string(id) {
			return resolved
		}
		resolved = fmt.Sprintf("%s_%d", base, suffix)
		suffix++
		if suffix > 100 {
			resolved = string(id)
			break
		}
	}
	return resolved
}
