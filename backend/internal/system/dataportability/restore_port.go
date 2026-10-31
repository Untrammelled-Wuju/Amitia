package dataportability

type RestorePurpose string

const (
	RestorePurposeOrdinary RestorePurpose = "ordinary"
	RestorePurposeFull     RestorePurpose = "full_restore"
)

type RestoreOptions struct {
	OperationID        string
	Purpose            RestorePurpose
	CharacterPolicy    CollisionPolicy
	DefaultCharacterID string
	ActivateImported   bool
	IncludeSecrets     bool
	AllowLegacySecrets bool
	IdentityMap        *ImportIdentityMap
	SecretProvider     SecretProvider
}
