//go:build linux && !android

package fileops

type Policy struct {
	MaxReadBytes         int64
	MaxWriteBytes        int64
	MaxListEntries       int
	MaxSearchResults     int
	MaxSearchDepth       int
	MaxCopyDepth         int
	MaxCopyFiles         int
	AllowAbsolutePaths   bool
	AllowSymlinkCreate   bool
	AllowChmod           bool
	DeniedMutationRoots  []string
}

func DefaultPolicy(workspaceDir, tempDir string) Policy {
	denied := []string{"/proc", "/sys", "/dev"}
	if workspaceDir != "" {
		denied = append(denied, workspaceDir+"/.amitia-core")
	}
	return Policy{
		MaxReadBytes:        10 * 1024 * 1024,
		MaxWriteBytes:       10 * 1024 * 1024,
		MaxListEntries:      1000,
		MaxSearchResults:    500,
		MaxSearchDepth:      20,
		MaxCopyDepth:        20,
		MaxCopyFiles:        10000,
		AllowAbsolutePaths:  true,
		AllowSymlinkCreate:  false,
		AllowChmod:          false,
		DeniedMutationRoots: denied,
	}
}
