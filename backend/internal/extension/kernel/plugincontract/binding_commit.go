package plugincontract

import (
	"errors"
	"fmt"
)

var ErrBindingCommitConflict = errors.New("plugincontract: binding commit conflict")

type BindingCommitConflictError struct {
	Epoch    string
	Expected uint64
	Actual   uint64
}

func (e *BindingCommitConflictError) Error() string {
	return fmt.Sprintf("plugincontract: binding commit expected revision %d at epoch %q but the installed revision is %d", e.Expected, e.Epoch, e.Actual)
}

func (e *BindingCommitConflictError) Unwrap() error { return ErrBindingCommitConflict }

type BindingVersion struct {
	Epoch     string
	Revision  uint64
	Digest    string
	Installed bool
}

func (s *BindingSnapshotStore) Version() BindingVersion {
	s.mu.Lock()
	defer s.mu.Unlock()
	version := BindingVersion{Epoch: s.epoch}
	if s.current != nil {
		version.Revision = s.current.Revision()
		version.Digest = s.current.Digest()
		version.Installed = true
	}
	return version
}

func (s *BindingSnapshotStore) Commit(expectedRevision uint64, bindings []Binding) (*BindingSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	actual := uint64(0)
	if s.current != nil {
		actual = s.current.Revision()
	}
	if expectedRevision != actual {
		return nil, &BindingCommitConflictError{Epoch: s.epoch, Expected: expectedRevision, Actual: actual}
	}
	revision := actual + 1
	snapshot, err := NewBindingSnapshot(s.epoch, revision, bindings)
	if err != nil {
		return nil, err
	}
	s.current = snapshot
	s.next = revision
	return snapshot, nil
}
