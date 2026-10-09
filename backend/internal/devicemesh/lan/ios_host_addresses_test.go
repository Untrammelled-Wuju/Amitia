package lan

import "testing"

func TestIOSHostAddressesArePrivateBoundedAndCanonical(t *testing.T) {
	addresses, err := validatedHostAddresses([]string{"192.168.1.3", "10.0.0.2", "192.168.1.3"})
	if err != nil || len(addresses) != 2 || addresses[0].String() != "10.0.0.2" {
		t.Fatalf("invalid canonical addresses: %v", err)
	}
	for _, value := range []string{"127.0.0.1", "8.8.8.8", "0.0.0.0", "169.254.1.1", "::1", "not-an-address", "192.168.1.3:18899"} {
		if _, err := validatedHostAddresses([]string{value}); err == nil {
			t.Fatalf("invalid host address accepted: %s", value)
		}
	}
	if _, err := validatedHostAddresses(make([]string, 17)); err == nil {
		t.Fatal("oversized address list accepted")
	}
	if addresses, err := validatedHostAddresses(nil); err != nil || len(addresses) != 0 {
		t.Fatal("offline host created a LAN endpoint")
	}
}

func TestIOSHostAddressesCannotFallBackToSpoofedGuestEnvironment(t *testing.T) {
	t.Setenv("AMITIA_RUNTIME_MODE", "ios-ish")
	t.Setenv("AMITIA_LAN_ADDRESSES", "192.168.1.99")
	for _, key := range []string{"READ_FD", "WRITE_FD", "GENERATION"} {
		t.Setenv("AMITIA_IOS_HOST_BRIDGE_"+key, "")
	}
	if _, err := PrivateAddresses(); err == nil {
		t.Fatal("guest environment substituted for host LAN authority")
	}
}
