package registry

type RegistrationState string

const (
	RegistrationStateRegistered   RegistrationState = "registered"
	RegistrationStateUnregistered RegistrationState = "unregistered"
)
