//go:build ios
// +build ios

package main

import (
	"github.com/u-ai/backend/internal/nativebridge"
)

func (r *nativeBridgeRelay) RegisterIOSBridge(bridge *nativebridge.IOSBridge) {
	r.handler.RegisterBridge("ios", bridge)
}

func tryRegisterIOSBridge(relay *nativeBridgeRelay, bootstrap *runtimeBootstrap) {
	if relay == nil || bootstrap == nil {
		return
	}
	iosBridge := bootstrap.IOSNativeBridge()
	if iosBridge == nil {
		return
	}
	if b, ok := iosBridge.(*nativebridge.IOSBridge); ok {
		relay.RegisterIOSBridge(b)
	}
}

func tryRegisterAndroidBridge(relay *nativeBridgeRelay, bootstrap *runtimeBootstrap) {
}
