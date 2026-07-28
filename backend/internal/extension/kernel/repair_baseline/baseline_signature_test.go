package repair_baseline

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBaseline_Install_NoTrustedKeysIteration(t *testing.T) {
	src := readInstallExecuteSource(t)

	if strings.Contains(src, "range sigVerifier.TrustedKeys()") {
		t.Fatalf("install_execute.go must resolve the signing key by (publisherId, keyId) instead of iterating all TrustedKeys; Phase 6 must replace the loop with a single precise lookup")
	}
}

func TestBaseline_Install_HandlesAllSignatureStates(t *testing.T) {
	src := readInstallExecuteSource(t)

	requiredStates := []string{
		"SignatureUnknownKey",
		"SignatureRevokedKey",
		"SignatureExpiredKey",
		"SignaturePublisherMismatch",
		"SignatureContentMismatch",
		"SignatureUnsupportedAlgorithm",
	}
	missing := []string{}
	for _, s := range requiredStates {
		if !strings.Contains(src, s) {
			missing = append(missing, s)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("install_execute.go must explicitly handle all signature failure states; missing=%v; Phase 6 must reject install for each of these states", missing)
	}
}

func readInstallExecuteSource(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	installPath := filepath.Join(filepath.Dir(file), "..", "install_execute.go")
	data, err := os.ReadFile(installPath)
	if err != nil {
		t.Fatalf("read install_execute.go: %v", err)
	}
	return string(data)
}
