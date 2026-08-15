package artifact

import (
	"fmt"
	"strings"
)

type Reference struct {
	ArtifactID ID
	URI        string
}

func URI(id ID) string {
	return fmt.Sprintf("amitia://artifacts/%s", id)
}

func ParseURI(raw string) (ID, error) {
	if raw == "" {
		return "", fmt.Errorf("artifact: empty uri")
	}
	if strings.Contains(raw, "?") || strings.Contains(raw, "#") {
		return "", fmt.Errorf("artifact: uri must not contain query or fragment")
	}
	const prefix = "amitia://artifacts/"
	if !strings.HasPrefix(raw, prefix) {
		return "", fmt.Errorf("artifact: invalid uri scheme")
	}
	id := strings.TrimPrefix(raw, prefix)
	if id == "" {
		return "", fmt.Errorf("artifact: empty artifact id")
	}
	if strings.Contains(id, "/") || strings.Contains(id, "..") {
		return "", fmt.Errorf("artifact: invalid artifact id")
	}
	return ID(id), nil
}
