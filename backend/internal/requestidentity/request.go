package requestidentity

const DefaultUserID = "default"

func ResolveGin(c interface{}, envelopeUserID string) string {
	_ = c
	_ = envelopeUserID
	return DefaultUserID
}
