//go:build !ios
// +build !ios

package main

func (b *runtimeBootstrap) registerProviderFactoriesIOS() error {
	if b == nil || b.providerRegistry == nil {
		return nil
	}

	return nil
}
