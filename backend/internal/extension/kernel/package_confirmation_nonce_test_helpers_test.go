package kernel

import "time"

func validTestConfirmationNonceBinding(
	operation PackageOperationRecord,
	nonce string,
	now time.Time,
) PackageConfirmationNonceBinding {
	now = now.UTC().Truncate(time.Second)

	return PackageConfirmationNonceBinding{
		Nonce:         nonce,
		OperationType: operation.OperationType,
		ExtensionID:   operation.ExtensionID,
		UserID:        operation.UserID,
		IssuedAt:      confirmationTimestamp(now.Unix()),
		ExpiresAt:     confirmationTimestamp(now.Add(5 * time.Minute).Unix()),
	}
}
