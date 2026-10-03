//go:build ios
// +build ios

package main

import (
	"github.com/u-ai/backend/internal/nativebridge"
)

func registerIOSBridgeWithRelay(relay *nativeBridgeRelay, bridge nativebridge.Bridge) {
	if relay == nil || bridge == nil {
		return
	}
	if iosBridge, ok := bridge.(*nativebridge.IOSBridge); ok {
		relay.RegisterIOSBridge(iosBridge)
	}
}
