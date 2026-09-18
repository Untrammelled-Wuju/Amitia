//go:build !linux && !windows

package platform

import (
	"encoding/json"
	"fmt"
)

func ProbeClient() ClientStatus { return ClientStatus{Message: "unsupported platform"} }
func StartClient(StartOptions) (ClientStatus, error) {
	return ProbeClient(), fmt.Errorf("unsupported platform")
}
func HideClient(int) error                  { return fmt.Errorf("unsupported platform") }
func ProbeDriver(ClientStatus) DriverStatus { return DriverStatus{Message: "unsupported platform"} }
func callDriver(ClientStatus, DriverRequest) (json.RawMessage, error) {
	return nil, fmt.Errorf("unsupported platform")
}
