package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

type leaseExecutor struct {
	calls   atomic.Int32
	started chan struct{}
	stopped chan struct{}
}

func (e *leaseExecutor) Execute(ctx context.Context, _ string, _ map[string]interface{}) (json.RawMessage, error) {
	e.calls.Add(1)
	close(e.started)
	<-ctx.Done()
	close(e.stopped)
	return nil, ctx.Err()
}

func TestDeviceTaskWaitsForCoreLeaseAndStopsWhenConnectionIsLost(t *testing.T) {
	for _, accepted := range []bool{false, true} {
		t.Run(map[bool]string{false: "rejected", true: "accepted"}[accepted], func(t *testing.T) {
			claimSeen := make(chan struct{})
			approve := make(chan struct{})
			claimResponded := make(chan struct{})
			serverSocket := make(chan *websocket.Conn, 1)
			handler := NewHandler(nil, nil)
			handler.SetOnTaskClaimPayload(func(protocol.TaskClaimPayload) bool {
				close(claimSeen)
				<-approve
				close(claimResponded)
				return accepted
			})
			upgrader := websocket.Upgrader{}
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				connection, err := upgrader.Upgrade(writer, request, nil)
				if err != nil {
					return
				}
				defer connection.Close()
				serverSocket <- connection
				var hello protocol.Envelope
				if err := connection.ReadJSON(&hello); err != nil {
					return
				}
				mesh := NewMeshConnection(connection, "session", 1, hello.SpaceID, hello.DeviceID, hello.RuntimeID)
				if err := handler.sendHelloAck(mesh, "session", 1, protocol.ResumeModeFresh); err != nil {
					return
				}
				if err := handler.sendEnvelope(mesh, protocol.MessageTypeTaskDispatch, protocol.TaskDispatchPayload{TaskRunID: "task", AttemptID: "attempt", LeaseID: "lease", TaskDefinitionID: "action", Input: json.RawMessage(`{}`), DeviceID: hello.DeviceID, RuntimeID: hello.RuntimeID, RuntimeSessionID: "session", ConnectionGeneration: 1}); err != nil {
					return
				}
				_ = handler.messageLoop(request.Context(), mesh, "session", 1, hello.Sequence)
			}))
			defer server.Close()
			client := agent.NewMeshClient(agent.MeshClientConfig{CloudBaseURL: server.URL, Credential: "secret", SpaceID: "core", Identity: &agent.LocalIdentity{DeviceID: "device", RuntimeID: "runtime"}})
			executor := &leaseExecutor{started: make(chan struct{}), stopped: make(chan struct{})}
			worker := agent.NewTaskWorker(client)
			worker.SetTaskRuntime(executor)
			client.SetTaskWorker(worker)
			client.Start()
			defer client.Stop()
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			if err := client.WaitReady(ctx); err != nil {
				t.Fatal(err)
			}
			select {
			case <-claimSeen:
			case <-ctx.Done():
				t.Fatal("Core did not receive task lease claim")
			}
			if executor.calls.Load() != 0 {
				t.Fatal("device action ran before Core accepted lease")
			}
			close(approve)
			select {
			case <-claimResponded:
			case <-ctx.Done():
				t.Fatal("Core did not respond")
			}
			if accepted {
				select {
				case <-executor.started:
				case <-ctx.Done():
					t.Fatal("accepted task did not start")
				}
				connection := <-serverSocket
				connection.Close()
				select {
				case <-executor.stopped:
				case <-time.After(time.Second):
					t.Fatal("device continued acting after disconnection")
				}
			} else {
				select {
				case <-executor.started:
					t.Fatal("rejected task executed")
				case <-time.After(100 * time.Millisecond):
				}
			}
			client.Stop()
			if err := client.WaitStopped(ctx); err != nil {
				t.Fatal(err)
			}
		})
	}
}
