package agent

import (
	"testing"

	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

type changingSessionDispatcher struct {
	client   *MeshClient
	change   func()
	executed bool
}

func (d *changingSessionDispatcher) Resolve(string) RuntimeInvokeHandler {
	d.change()
	return func(protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
		d.executed = true
		return &protocol.RuntimeResultPayload{}, nil
	}
}

func TestRuntimeInvokeRejectsSessionChangeDuringResolution(t *testing.T) {
	for _, scenario := range []string{"disconnect", "new-generation", "new-session"} {
		t.Run(scenario, func(t *testing.T) {
			dispatcher := &changingSessionDispatcher{}
			client := NewMeshClient(MeshClientConfig{SpaceID: "core", Identity: &LocalIdentity{DeviceID: "device", RuntimeID: "runtime"}, RuntimeDispatcher: dispatcher})
			client.sessionID = "session"
			client.connectionGen = 1
			client.setState(StateReady)
			dispatcher.client = client
			dispatcher.change = func() {
				switch scenario {
				case "disconnect":
					client.setState(StateDegraded)
				case "new-generation":
					client.sessionMu.Lock()
					client.connectionGen++
					client.sessionMu.Unlock()
				case "new-session":
					client.sessionMu.Lock()
					client.sessionID = "new-session"
					client.sessionMu.Unlock()
				}
			}
			_, err := client.executeRuntimeInvoke(protocol.RuntimeInvokePayload{InvocationID: "invoke", SpaceID: "core", DeviceID: "device", RuntimeID: "runtime", RuntimeSessionID: "session", ConnectionGeneration: 1, Handler: "coordination.data"})
			if err == nil || dispatcher.executed {
				t.Fatalf("expired invoke executed=%v err=%v", dispatcher.executed, err)
			}
		})
	}
}
