package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/workspace"
)

func TestGitRootResolverHonorsLocalWorkspaceMount(t *testing.T) {
	dataRoot := t.TempDir()
	repositoryRoot := t.TempDir()
	resolver := NewAmitiaDataRootResolver(dataRoot)
	canonical, err := filepath.EvalSymlinks(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := resolver.ResolveRoot(workspace.WorkspaceMount{
		Kind: workspace.WorkspaceKindLocal, LocalRoot: canonical,
	})
	if err != nil || filepath.Clean(actual) != filepath.Clean(canonical) {
		t.Fatalf("local Git operations resolved wrong root: %s %v", actual, err)
	}
	if _, err := resolver.ResolveRoot(workspace.WorkspaceMount{Kind: workspace.WorkspaceKindLocal}); err == nil {
		t.Fatal("empty local root was accepted")
	}
	if _, err := resolver.ResolveRoot(workspace.WorkspaceMount{Kind: workspace.WorkspaceKindRemote}); err == nil {
		t.Fatal("remote workspace mapped to a local repository")
	}
}

func TestGitRootResolverRejectsRetargetedLocalWorkspace(t *testing.T) {
	base := t.TempDir()
	first := filepath.Join(base, "first")
	second := filepath.Join(base, "second")
	link := filepath.Join(base, "linked")
	for _, path := range []string{first, second} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(first, link); err != nil {
		t.Skipf("symlinks are not available in this environment: %v", err)
	}
	resolver := NewAmitiaDataRootResolver(t.TempDir())
	if _, err := resolver.ResolveRoot(workspace.WorkspaceMount{Kind: workspace.WorkspaceKindLocal, LocalRoot: link}); err == nil {
		t.Fatal("uncanonicalized local mount accepted")
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(second, link); err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.ResolveRoot(workspace.WorkspaceMount{Kind: workspace.WorkspaceKindLocal, LocalRoot: link}); err == nil {
		t.Fatal("retargeted local mount accepted")
	}
}
