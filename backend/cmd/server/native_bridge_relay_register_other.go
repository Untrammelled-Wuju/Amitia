//go:build !ios

package main

import (
	"github.com/u-ai/backend/internal/nativebridge"
)

func tryRegisterAndroidBridge(relay *nativeBridgeRelay, bootstrap *runtimeBootstrap) {
	if relay == nil || bootstrap == nil {
		return
	}
	androidBridge := bootstrap.AndroidNativeBridge()
	if androidBridge == nil {
		return
	}
	if b, ok := androidBridge.(*nativebridge.AndroidTransportBridge); ok {
		relay.RegisterAndroidBridge(b)
	}
}

func tryRegisterIOSBridge(relay *nativeBridgeRelay, bootstrap *runtimeBootstrap) {
	if relay == nil || bootstrap == nil {
		return
	}
	if bridge, ok := bootstrap.IOSNativeBridge().(*nativebridge.IOSBridge); ok {
		relay.handler.RegisterBridge("ios", bridge)
	}
}
