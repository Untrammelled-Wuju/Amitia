package secret

type RevokeOutcome struct {
	RevokedCount int
	RequestedBy  string
	Reason       string
}

type AcquireOutcome struct {
	Requested  SecretAcquireRequest
	Result     SecretAcquireResult
	LeaseError error
}
