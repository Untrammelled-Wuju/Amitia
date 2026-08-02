//go:build legacy_migration

package extension

import "context"

func (r *Runtime) AttachPackageKernelProxy(proxy *KernelLifecycleProxy) error {
	if proxy == nil {
		return nil
	}
	return proxy.kernel.RecoverPackageOperations(context.Background())
}
