package workbenchterminal

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"
)

const ticketLifetime = 20 * time.Second
const maxPendingTickets = 128

var ErrTicket = errors.New("terminal stream ticket is invalid or expired")

type streamTicket struct {
	owner          string
	sessionID      string
	origin         string
	conversationID string
	expires        time.Time
}

type ticketRegistry struct {
	mu      sync.Mutex
	tickets map[string]streamTicket
}

func validStreamOrigin(raw string) bool {
	if raw == "null" {
		return true
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "app" {
		return false
	}
	return strings.EqualFold(u.String(), raw)
}

func (r *ticketRegistry) issue(owner, sessionID, origin string, conversationIDs ...string) (string, error) {
	if owner == "" || sessionID == "" || !validStreamOrigin(origin) {
		return "", ErrInvalid
	}
	var bytes [24]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	id := hex.EncodeToString(bytes[:])
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tickets == nil {
		r.tickets = make(map[string]streamTicket)
	}
	now := time.Now()
	for key, entry := range r.tickets {
		if !now.Before(entry.expires) {
			delete(r.tickets, key)
		}
	}
	if len(r.tickets) >= maxPendingTickets {
		return "", ErrLimit
	}
	conversationID := ""
	if len(conversationIDs) > 0 {
		conversationID = conversationIDs[0]
	}
	r.tickets[id] = streamTicket{owner: owner, sessionID: sessionID, origin: origin, conversationID: conversationID, expires: now.Add(ticketLifetime)}
	return id, nil
}

func (r *ticketRegistry) consume(id, origin string) (streamTicket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, exists := r.tickets[id]
	delete(r.tickets, id)
	if !exists || !time.Now().Before(entry.expires) || entry.origin != origin {
		return streamTicket{}, ErrTicket
	}
	return entry, nil
}
