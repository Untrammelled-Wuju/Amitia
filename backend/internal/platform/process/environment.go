package process

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type EnvironmentBuilder struct {
	vars map[string]string
}

func NewEnvironmentBuilder() *EnvironmentBuilder {
	b := &EnvironmentBuilder{
		vars: make(map[string]string),
	}
	b.addSystemMinimum()
	return b
}

func (b *EnvironmentBuilder) addSystemMinimum() {
	b.vars["PATH"] = os.Getenv("PATH")
	b.vars["HOME"] = os.Getenv("HOME")
	b.vars["TMP"] = os.Getenv("TMP")
	b.vars["TEMP"] = os.Getenv("TEMP")
	if runtime.GOOS == "windows" {
		b.vars["SystemRoot"] = os.Getenv("SystemRoot")
		b.vars["USERPROFILE"] = os.Getenv("USERPROFILE")
		b.vars["PATHEXT"] = os.Getenv("PATHEXT")
	}
}

func (b *EnvironmentBuilder) Set(key, value string) *EnvironmentBuilder {
	b.vars[key] = value
	return b
}

func (b *EnvironmentBuilder) SetRuntimeInstance(instanceID string) *EnvironmentBuilder {
	b.vars["AMITIA_RUNTIME_INSTANCE_ID"] = instanceID
	return b
}

func (b *EnvironmentBuilder) SetExtensionID(extensionID string) *EnvironmentBuilder {
	b.vars["AMITIA_EXTENSION_ID"] = extensionID
	return b
}

func (b *EnvironmentBuilder) SetModuleID(moduleID string) *EnvironmentBuilder {
	b.vars["AMITIA_MODULE_ID"] = moduleID
	return b
}

func (b *EnvironmentBuilder) SetGeneration(generation int64) *EnvironmentBuilder {
	b.vars["AMITIA_RUNTIME_GENERATION"] = fmt.Sprintf("%d", generation)
	return b
}

func (b *EnvironmentBuilder) SetRPCEndpoint(endpoint string) *EnvironmentBuilder {
	b.vars["AMITIA_RPC_ENDPOINT"] = endpoint
	return b
}

func (b *EnvironmentBuilder) SetSessionNonce(nonce string) *EnvironmentBuilder {
	b.vars["AMITIA_SESSION_NONCE"] = nonce
	return b
}

func (b *EnvironmentBuilder) SetLogLevel(level string) *EnvironmentBuilder {
	b.vars["AMITIA_LOG_LEVEL"] = level
	return b
}

func (b *EnvironmentBuilder) SetTempDir(dir string) *EnvironmentBuilder {
	b.vars["AMITIA_TEMP_DIR"] = dir
	return b
}

func (b *EnvironmentBuilder) SetDataHandle(handle string) *EnvironmentBuilder {
	b.vars["AMITIA_DATA_HANDLE"] = handle
	return b
}

func (b *EnvironmentBuilder) SetConfigHandle(handle string) *EnvironmentBuilder {
	b.vars["AMITIA_CONFIG_HANDLE"] = handle
	return b
}

func (b *EnvironmentBuilder) Build() []string {
	out := make([]string, 0, len(b.vars))
	for k, v := range b.vars {
		out = append(out, k+"="+v)
	}
	return out
}

func (b *EnvironmentBuilder) BuildFiltered() []string {
	allowed := allowedEnvKeys()
	out := make([]string, 0, len(b.vars))
	for k, v := range b.vars {
		if !allowed[k] {
			continue
		}
		out = append(out, k+"="+v)
	}
	return out
}

func allowedEnvKeys() map[string]bool {
	return map[string]bool{
		"PATH":                       true,
		"HOME":                       true,
		"TMP":                        true,
		"TEMP":                       true,
		"SystemRoot":                 true,
		"USERPROFILE":                true,
		"PATHEXT":                    true,
		"AMITIA_RUNTIME_INSTANCE_ID": true,
		"AMITIA_EXTENSION_ID":        true,
		"AMITIA_MODULE_ID":           true,
		"AMITIA_RUNTIME_GENERATION":  true,
		"AMITIA_RPC_ENDPOINT":        true,
		"AMITIA_SESSION_NONCE":       true,
		"AMITIA_LOG_LEVEL":           true,
		"AMITIA_TEMP_DIR":            true,
		"AMITIA_DATA_HANDLE":         true,
		"AMITIA_CONFIG_HANDLE":       true,
		"AMITIA_SESSION":             true,
		"AMITIA_INSTANCE":            true,
		"AMITIA_GENERATION":          true,
		"AMITIA_SECRET_LEASE":        true,
		"AMITIA_HOST_API":            true,
		"AMITIA_PROTOCOL":            true,
		"AMITIA_PLATFORM":            true,
	}
}

func ValidatePath(path string) error {
	if path == "" {
		return fmt.Errorf("process: empty path")
	}
	if filepath.IsAbs(path) {
		return fmt.Errorf("process: absolute path not allowed: %s", path)
	}
	if strings.HasPrefix(path, "../") || strings.Contains(path, "/../") || strings.Contains(path, "\\..\\") {
		return fmt.Errorf("process: path traversal not allowed: %s", path)
	}
	return nil
}

func ResolveExecutable(basePath, relPath string) (string, error) {
	if err := ValidatePath(relPath); err != nil {
		return "", err
	}
	if basePath == "" {
		return relPath, nil
	}
	cleaned := filepath.Clean(filepath.Join(basePath, relPath))
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return "", fmt.Errorf("process: resolve base path: %w", err)
	}
	absResolved, err := filepath.Abs(cleaned)
	if err != nil {
		return "", fmt.Errorf("process: resolve executable path: %w", err)
	}
	if !strings.HasPrefix(absResolved, absBase) {
		return "", fmt.Errorf("process: executable path escapes base directory")
	}
	return absResolved, nil
}
