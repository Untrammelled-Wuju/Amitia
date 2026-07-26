//go:build !windows && !linux && !android

package platform

func Detect() RuntimePlatform {
	return &ServerRuntime{}
}
