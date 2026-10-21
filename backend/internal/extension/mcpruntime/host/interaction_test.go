package host

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type samplingExecutorStub struct{}

func (samplingExecutorStub) GenerateMCPSampling(context.Context, json.RawMessage) (any, error) {
	return map[string]any{"role": "assistant", "content": map[string]any{"type": "text", "text": "result"}}, nil
}

func waitPending(t *testing.T, broker *Broker, kind string) PendingInteraction {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, item := range broker.List() {
			if item.Kind == kind {
				return item
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("pending interaction %s not found", kind)
	return PendingInteraction{}
}

func TestBrokerRequiresRequestAndResultApproval(t *testing.T) {
	broker := NewBroker(samplingExecutorStub{})
	type outcome struct {
		result any
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := broker.CreateMessage(context.Background(), "server-1", json.RawMessage(`{"messages":[{"role":"user","content":"hello"}]}`))
		done <- outcome{result: result, err: err}
	}()
	request := waitPending(t, broker, "sampling")
	if err := broker.Resolve(request.ID, InteractionDecision{Action: "accept"}); err != nil {
		t.Fatal(err)
	}
	result := waitPending(t, broker, "sampling_result")
	if err := broker.Resolve(result.ID, InteractionDecision{Action: "accept"}); err != nil {
		t.Fatal(err)
	}
	select {
	case value := <-done:
		if value.err != nil || value.result == nil {
			t.Fatalf("unexpected outcome: %#v", value)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("sampling approval timed out")
	}
}

func TestBrokerDeclineAndSingleResolution(t *testing.T) {
	broker := NewBroker(samplingExecutorStub{})
	done := make(chan error, 1)
	go func() {
		_, err := broker.CreateMessage(context.Background(), "server-1", json.RawMessage(`{"messages":[]}`))
		done <- err
	}()
	request := waitPending(t, broker, "sampling")
	if err := broker.Resolve(request.ID, InteractionDecision{Action: "decline"}); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err == nil {
		t.Fatal("expected declined sampling error")
	}
	if err := broker.Resolve(request.ID, InteractionDecision{Action: "accept"}); err == nil {
		t.Fatal("expected resolved interaction error")
	}
}

func TestBrokerValidatesElicitationFormDecision(t *testing.T) {
	broker := NewBroker(nil)
	done := make(chan error, 1)
	go func() {
		result, err := broker.Elicit(context.Background(), "server-1", json.RawMessage(`{"mode":"form","requestedSchema":{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer"}},"required":["name"]}}`))
		if err == nil && result.(map[string]any)["action"] != "accept" {
			err = context.Canceled
		}
		done <- err
	}()
	request := waitPending(t, broker, "elicitation")
	if err := broker.Resolve(request.ID, InteractionDecision{Action: "accept", Content: map[string]any{"unknown": "value"}}); err == nil {
		t.Fatal("expected unknown field rejection")
	}
	if err := broker.Resolve(request.ID, InteractionDecision{Action: "accept", Content: map[string]any{"name": "Ada", "age": float64(36)}}); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
