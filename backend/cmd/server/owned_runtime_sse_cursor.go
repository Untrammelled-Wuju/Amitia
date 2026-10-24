package main

import (
	"errors"
	"fmt"
	"io"
	"strconv"
)

var errOwnedRuntimeSnapshotRequired = errors.New("runtime stream requires snapshot")

func validateOwnedRuntimeEventCursor(sequence int64, header string) error {
	if sequence < 0 {
		return errors.New("invalid runtime event sequence")
	}
	if header == "" {
		return nil
	}
	parsed, err := strconv.ParseInt(header, 10, 64)
	if err != nil || parsed <= 0 || strconv.FormatInt(parsed, 10) != header || parsed != sequence {
		return errors.New("runtime event cursor mismatch")
	}
	return nil
}

func writeOwnedRuntimeSnapshotRequired(writer io.Writer) error {
	_, err := io.WriteString(writer, "event: snapshot_required\ndata: {}\n\n")
	return err
}

func writeOwnedRuntimeEventFrame(writer io.Writer, sequence int64, payload []byte) error {
	_, err := fmt.Fprintf(writer, "id: %d\nevent: agent_event\ndata: %s\n\n", sequence, payload)
	return err
}

func ownedRuntimeEventIsNext(previous, incoming int64) bool {
	return previous >= 0 && incoming > 0 && incoming-previous == 1
}

func ownedRuntimeDurableCursorAdvanced(delivered, durableLatest int64) bool {
	return durableLatest > delivered
}

func writeOwnedRuntimeSequencedFrame(writer io.Writer, previous, incoming int64, payload []byte) (bool, error) {
	if !ownedRuntimeEventIsNext(previous, incoming) {
		return true, writeOwnedRuntimeSnapshotRequired(writer)
	}
	return false, writeOwnedRuntimeEventFrame(writer, incoming, payload)
}
