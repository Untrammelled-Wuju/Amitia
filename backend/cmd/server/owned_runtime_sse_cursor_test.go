package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestRound90RuntimeSSECursorValidation(t *testing.T) {
	for _, tc := range []struct {
		sequence int64
		header   string
		valid    bool
	}{
		{0, "", true},
		{1, "", true},
		{7, "7", true},
		{7, "6", false},
		{0, "1", false},
		{7, "07", false},
		{7, "-7", false},
		{7, "not-a-number", false},
		{7, "9223372036854775808", false},
		{-1, "", false},
	} {
		err := validateOwnedRuntimeEventCursor(tc.sequence, tc.header)
		if (err == nil) != tc.valid {
			t.Fatalf("cursor %d / %q valid=%v error=%v", tc.sequence, tc.header, tc.valid, err)
		}
	}
}

func TestRound90RuntimeSSERecoverableSequenceGap(t *testing.T) {
	var stream bytes.Buffer
	if !ownedRuntimeEventIsNext(4, 5) || ownedRuntimeEventIsNext(4, 6) || ownedRuntimeEventIsNext(4, 4) || ownedRuntimeEventIsNext(0, 0) {
		t.Fatal("event sequence adjacency not enforced")
	}
	if err := writeOwnedRuntimeEventFrame(&stream, 5, []byte(`{"eventSequence":5}`)); err != nil {
		t.Fatal(err)
	}
	if err := writeOwnedRuntimeSnapshotRequired(&stream); err != nil {
		t.Fatal(err)
	}
	if got := stream.String(); got != "id: 5\nevent: agent_event\ndata: {\"eventSequence\":5}\n\nevent: snapshot_required\ndata: {}\n\n" {
		t.Fatalf("invalid SSE wire frames: %q", got)
	}
	if !errors.Is(errOwnedRuntimeSnapshotRequired, errOwnedRuntimeSnapshotRequired) {
		t.Fatal("snapshot sentinel cannot be recognized")
	}
}

type round90FailWriter struct{}

func (round90FailWriter) Write([]byte) (int, error) { return 0, errors.New("closed stream") }

func TestRound90RuntimeSSEWriteFailureIsNotConfirmation(t *testing.T) {
	if err := writeOwnedRuntimeSnapshotRequired(round90FailWriter{}); err == nil {
		t.Fatal("snapshot write failure was lost")
	}
	if err := writeOwnedRuntimeEventFrame(round90FailWriter{}, 1, []byte(strings.Repeat("x", 1))); err == nil {
		t.Fatal("event write failure was lost")
	}
}

func TestRound90RuntimeSSEGapYieldsSnapshotAndStopsReplay(t *testing.T) {
	for _, tc := range []struct {
		previous   int64
		incoming   int64
		wantResync bool
		wantFrame  string
	}{
		{0, 1, false, "id: 1\nevent: agent_event\ndata: {}\n\n"},
		{3, 4, false, "id: 4\nevent: agent_event\ndata: {}\n\n"},
		{3, 5, true, "event: snapshot_required\ndata: {}\n\n"},
		{3, 3, true, "event: snapshot_required\ndata: {}\n\n"},
		{3, 2, true, "event: snapshot_required\ndata: {}\n\n"},
	} {
		var output bytes.Buffer
		resync, err := writeOwnedRuntimeSequencedFrame(&output, tc.previous, tc.incoming, []byte(`{}`))
		if err != nil || resync != tc.wantResync || output.String() != tc.wantFrame {
			t.Fatalf("previous=%d incoming=%d resync=%v err=%v frame=%q", tc.previous, tc.incoming, resync, err, output.String())
		}
	}
}
