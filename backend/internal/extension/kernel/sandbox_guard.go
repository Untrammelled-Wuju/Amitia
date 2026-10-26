package kernel

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/internal/execution"
)

var ErrSandboxEscape = errors.New("SANDBOX_ESCAPE")

func invocationSandbox(execCtx *execution.ExecutionContext) *execution.SandboxConstraint {
	if execCtx == nil || execCtx.Sandbox == nil {
		return nil
	}
	if strings.TrimSpace(execCtx.Sandbox.Root) == "" {
		return nil
	}
	return execCtx.Sandbox
}

func ensureSandboxPath(execCtx *execution.ExecutionContext, target string) error {
	sandbox := invocationSandbox(execCtx)
	if sandbox == nil {
		return nil
	}
	root, err := resolveExistingPath(sandbox.Root)
	if err != nil {
		return fmt.Errorf("%w: sandbox root is unavailable: %v", ErrSandboxEscape, err)
	}
	resolved, err := resolveExistingPath(target)
	if err != nil {
		return err
	}
	if !withinDirectory(root, resolved) {
		log.Printf("[sandbox-audit] refuse path=%q root=%q", target, sandbox.Root)
		return fmt.Errorf("%w: %s is outside the channel sandbox", ErrSandboxEscape, target)
	}
	return nil
}

// resolveExistingPath canonicalises a path by resolving its deepest existing
// ancestor, so a target that does not exist yet is still verified through its
// real parent chain instead of an unresolved ".." string.
func resolveExistingPath(target string) (string, error) {
	current, err := filepath.Abs(strings.TrimSpace(target))
	if err != nil {
		return "", err
	}
	remainder := make([]string, 0, 4)
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for index := len(remainder) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, remainder[index])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		remainder = append(remainder, filepath.Base(current))
		current = parent
	}
}

func withinDirectory(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
