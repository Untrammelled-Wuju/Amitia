package rpc

import (
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/ipc"
)

func TestCorrelationMap_Bidirectional(t *testing.T) {
	cm := NewCorrelationMap()

	upstream := RequestKey{RuntimeID: "r1", ServiceID: "s1", RequestID: "req-1"}
	downstreamPeer := ipc.Peer{RuntimeID: "r2", ServiceID: "s2"}

	corr := &Correlation{
		Upstream:        upstream,
		DownstreamPeer:  downstreamPeer,
		DownstreamReqID: "host-xyz",
		CreatedAt:       time.Now().UTC(),
	}

	cm.Add(corr)

	c1, ok := cm.ByUpstream(upstream)
	if !ok {
		t.Fatal("ByUpstream should find correlation")
	}
	if c1.DownstreamReqID != "host-xyz" {
		t.Error("downstream request ID mismatch")
	}

	c2, ok := cm.ByDownstream(downstreamPeer, "host-xyz")
	if !ok {
		t.Fatal("ByDownstream should find correlation")
	}
	if c2.Upstream != upstream {
		t.Error("upstream key mismatch")
	}
}

func TestCorrelationMap_Remove(t *testing.T) {
	cm := NewCorrelationMap()
	upstream := RequestKey{RuntimeID: "r1", ServiceID: "s1", RequestID: "req-1"}
	peer := ipc.Peer{RuntimeID: "r2", ServiceID: "s2"}

	cm.Add(&Correlation{
		Upstream:        upstream,
		DownstreamPeer:  peer,
		DownstreamReqID: "host-xyz",
	})

	cm.Remove(upstream)

	_, ok := cm.ByUpstream(upstream)
	if ok {
		t.Error("removed upstream should not be found")
	}

	_, ok = cm.ByDownstream(peer, "host-xyz")
	if ok {
		t.Error("removed downstream should not be found")
	}
}

func TestCorrelationMap_RemoveByDownstream(t *testing.T) {
	cm := NewCorrelationMap()
	upstream := RequestKey{RuntimeID: "r1", ServiceID: "s1", RequestID: "req-1"}
	peer := ipc.Peer{RuntimeID: "r2", ServiceID: "s2"}

	cm.Add(&Correlation{
		Upstream:        upstream,
		DownstreamPeer:  peer,
		DownstreamReqID: "host-xyz",
	})

	cm.RemoveByDownstream(peer, "host-xyz")

	_, ok := cm.ByUpstream(upstream)
	if ok {
		t.Error("removed correlation should not be found by upstream")
	}
}

func TestCorrelationMap_RemoveByRuntime(t *testing.T) {
	cm := NewCorrelationMap()

	cm.Add(&Correlation{
		Upstream:        RequestKey{RuntimeID: "r1", ServiceID: "s1", RequestID: "req-1"},
		DownstreamPeer:  ipc.Peer{RuntimeID: "r3", ServiceID: "s3"},
		DownstreamReqID: "host-1",
	})

	cm.Add(&Correlation{
		Upstream:        RequestKey{RuntimeID: "r2", ServiceID: "s2", RequestID: "req-2"},
		DownstreamPeer:  ipc.Peer{RuntimeID: "r3", ServiceID: "s3"},
		DownstreamReqID: "host-2",
	})

	count := cm.RemoveByRuntime("r1")
	if count != 1 {
		t.Errorf("expected 1 removal, got %d", count)
	}

	if cm.Len() != 1 {
		t.Errorf("expected len 1 after removal, got %d", cm.Len())
	}
}

func TestCorrelationMap_RemoveByService(t *testing.T) {
	cm := NewCorrelationMap()

	cm.Add(&Correlation{
		Upstream:        RequestKey{RuntimeID: "r1", ServiceID: "s1", RequestID: "req-1"},
		DownstreamPeer:  ipc.Peer{RuntimeID: "r3", ServiceID: "s3"},
		DownstreamReqID: "host-1",
	})

	cm.Add(&Correlation{
		Upstream:        RequestKey{RuntimeID: "r1", ServiceID: "s2", RequestID: "req-2"},
		DownstreamPeer:  ipc.Peer{RuntimeID: "r3", ServiceID: "s3"},
		DownstreamReqID: "host-2",
	})

	count := cm.RemoveByService("r1", "s1")
	if count != 1 {
		t.Errorf("expected 1 removal, got %d", count)
	}

	if cm.Len() != 1 {
		t.Errorf("expected len 1, got %d", cm.Len())
	}
}

func TestCorrelationMap_DownstreamIDIsolation(t *testing.T) {
	cm := NewCorrelationMap()

	cm.Add(&Correlation{
		Upstream:        RequestKey{RuntimeID: "r1", ServiceID: "s1", RequestID: "req-1"},
		DownstreamPeer:  ipc.Peer{RuntimeID: "r3", ServiceID: "s3"},
		DownstreamReqID: "req-1",
	})

	cm.Add(&Correlation{
		Upstream:        RequestKey{RuntimeID: "r2", ServiceID: "s2", RequestID: "req-1"},
		DownstreamPeer:  ipc.Peer{RuntimeID: "r3", ServiceID: "s3"},
		DownstreamReqID: "req-1",
	})

	if cm.Len() != 2 {
		t.Errorf("expected 2 correlations, got %d", cm.Len())
	}

	peer3 := ipc.Peer{RuntimeID: "r3", ServiceID: "s3"}

	results := []*Correlation{}
	if c, ok := cm.ByDownstream(peer3, "req-1"); ok {
		results = append(results, c)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 downstream match, got %d", len(results))
	}
}

func TestCorrelationMap_UnknownLookup(t *testing.T) {
	cm := NewCorrelationMap()

	_, ok := cm.ByUpstream(RequestKey{RuntimeID: "r1", ServiceID: "s1", RequestID: "req"})
	if ok {
		t.Error("unknown upstream should not be found")
	}

	_, ok = cm.ByDownstream(ipc.Peer{RuntimeID: "r1", ServiceID: "s1"}, "unknown")
	if ok {
		t.Error("unknown downstream should not be found")
	}
}
