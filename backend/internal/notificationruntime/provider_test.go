package notificationruntime

import "testing"

func TestFCMProviderWithoutConfigurationIsUnavailable(t *testing.T) {
	t.Setenv("AMITIA_FCM_SERVICE_ACCOUNT_JSON", "")
	t.Setenv("AMITIA_FCM_SERVICE_ACCOUNT_FILE", "")

	provider := NewFCMProviderFromEnvironment()
	if provider.Available() {
		t.Fatal("unconfigured FCM provider must be unavailable")
	}
}
