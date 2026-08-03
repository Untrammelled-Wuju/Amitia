//go:build windows

package kernel

func syscallNofollow() int {
	return 0
}
