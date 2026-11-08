package pairing

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"
)

var ErrApprovalNotFound = errors.New("配对审批请求不存在")

type approvalEventHub struct {
	mu        sync.Mutex
	requests  map[string]map[chan Approval]struct{}
	observers map[chan struct{}]struct{}
}

func (h *approvalEventHub) subscribeRequest(requestID string) (<-chan Approval, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.requests == nil {
		h.requests = make(map[string]map[chan Approval]struct{})
	}
	ch := make(chan Approval, 4)
	if h.requests[requestID] == nil {
		h.requests[requestID] = make(map[chan Approval]struct{})
	}
	h.requests[requestID][ch] = struct{}{}
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if subscribers := h.requests[requestID]; subscribers != nil {
			if _, ok := subscribers[ch]; ok {
				delete(subscribers, ch)
				close(ch)
			}
			if len(subscribers) == 0 {
				delete(h.requests, requestID)
			}
		}
	}
}

func (h *approvalEventHub) observe() (<-chan struct{}, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.observers == nil {
		h.observers = make(map[chan struct{}]struct{})
	}
	ch := make(chan struct{}, 1)
	h.observers[ch] = struct{}{}
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, ok := h.observers[ch]; ok {
			delete(h.observers, ch)
			close(ch)
		}
	}
}

func (s *Service) publishApproval(approval Approval) {
	s.approvals.mu.Lock()
	defer s.approvals.mu.Unlock()
	for ch := range s.approvals.requests[approval.RequestID] {
		select {
		case ch <- approval:
		default:
		}
	}
	for ch := range s.approvals.observers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (s *Service) ApprovalState(ctx context.Context, requestID string) (Approval, time.Time, error) {
	if requestID == "" {
		return Approval{}, time.Time{}, ErrApprovalNotFound
	}
	var approval Approval
	var expiresAt string
	err := s.db.QueryRowContext(ctx, `SELECT a.request_id,a.offer_id,a.device_id,a.runtime_id,a.platform,a.label,a.public_key,a.state,a.revision,a.created_at,o.expires_at
		FROM kernel_device_pairing_approvals a
		JOIN kernel_device_pairing_offers o ON o.offer_id=a.offer_id
		WHERE a.space_id=? AND a.request_id=?`, s.spaceID.String(), requestID).Scan(
		&approval.RequestID,
		&approval.OfferID,
		&approval.DeviceID,
		&approval.RuntimeID,
		&approval.Platform,
		&approval.Label,
		&approval.PublicKey,
		&approval.State,
		&approval.Revision,
		&approval.CreatedAt,
		&expiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Approval{}, time.Time{}, ErrApprovalNotFound
	}
	if err != nil {
		return Approval{}, time.Time{}, err
	}
	expiry, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil {
		return Approval{}, time.Time{}, err
	}
	return approval, expiry, nil
}

func (s *Service) WatchApproval(ctx context.Context, requestID string) (Approval, time.Time, <-chan Approval, func(), error) {
	events, cancel := s.approvals.subscribeRequest(requestID)
	approval, expiry, err := s.ApprovalState(ctx, requestID)
	if err != nil {
		cancel()
		return Approval{}, time.Time{}, nil, nil, err
	}
	return approval, expiry, events, cancel, nil
}

func (s *Service) WatchApprovalRequests() (<-chan struct{}, func()) {
	return s.approvals.observe()
}
