package workbenchterminal

func (s *session) broadcastLocked() {
	if s.changed != nil {
		close(s.changed)
	}
	s.changed = make(chan struct{})
}

func (s *session) snapshotAndListen(cursor int64) (Snapshot, <-chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.changed == nil {
		s.changed = make(chan struct{})
	}
	return s.snapshotLocked(cursor), s.changed
}
