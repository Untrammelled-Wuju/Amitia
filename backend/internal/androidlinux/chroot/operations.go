//go:build linux && !android

package chroot

const (
	OpChrootStatus  = "chroot.status"
	OpChrootInspect = "chroot.inspect"
	OpChrootExec    = "chroot.exec"
)
