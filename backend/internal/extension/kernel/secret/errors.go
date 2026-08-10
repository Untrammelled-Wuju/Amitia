package secret

import "fmt"

var (
	ErrSecretNotFound           = fmt.Errorf("secret: not found")
	ErrSecretStoreUnavailable   = fmt.Errorf("secret: store unavailable")
	ErrSecretLeaseExpired       = fmt.Errorf("secret: lease expired")
	ErrSecretLeaseRevoked       = fmt.Errorf("secret: lease revoked")
	ErrSecretLeaseExhausted     = fmt.Errorf("secret: lease exhausted")
	ErrSecretLeaseScopeMismatch = fmt.Errorf("secret: lease scope mismatch")
	ErrSecretRefInvalid         = fmt.Errorf("secret: invalid ref")
	ErrSecretStoreCorrupted     = fmt.Errorf("secret: store corrupted")
	ErrSecretDecryptionFailed   = fmt.Errorf("secret: decryption failed")

	ErrLeaseInvocationMismatch      = fmt.Errorf("secret: lease invocation mismatch")
	ErrLeaseExtensionMismatch       = fmt.Errorf("secret: lease extension mismatch")
	ErrLeaseModuleMismatch          = fmt.Errorf("secret: lease module mismatch")
	ErrLeaseRuntimeInstanceMismatch = fmt.Errorf("secret: lease runtime instance mismatch")
	ErrLeaseGenerationMismatch      = fmt.Errorf("secret: lease generation mismatch")
)
