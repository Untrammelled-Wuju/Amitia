//go:build !windows

package trusted_service

func recoverPlatformSandboxResidue(string) error { return nil }
