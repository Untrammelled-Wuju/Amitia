//go:build !ios

package main

import "github.com/u-ai/backend/internal/nativebridge"

func (b *runtimeBootstrap) buildIOSNativeBridge() nativebridge.Bridge {
	return nil
}

func (b *runtimeBootstrap) buildPlatformProvidersIOS() error {
	return nil
}
