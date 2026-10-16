package workbenchbrowser

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/browser"
)

// A control lease is a short-lived, in-memory grant to send inputs to an Agent's
// existing tab. It never conveys ownership of the BrowserProvider session.
// BrowserAgent consults the same lease registry before dispatching further
// mutating tool calls, so manual control and automation cannot race at a tool
// boundary. An already-running browser command cannot be forcibly cancelled.
const agentControlLeaseTTL = 45 * time.Second

type agentControlLease struct {
	ownerKey   string
	leaseID    string
	browserRef string
	deadline   time.Time
}

var agentControlLeases = struct {
	sync.Mutex
	items          map[string]agentControlLease
	agentInFlight  map[string]int
	manualInFlight map[string]int
}{items: make(map[string]agentControlLease), agentInFlight: make(map[string]int), manualInFlight: make(map[string]int)}

func agentControlTabKey(session browser.BrowserSessionID, tab browser.BrowserTabID) string {
	return string(session) + "\x00" + string(tab)
}

func acquireAgentControl(ref, owner string, session browser.BrowserSessionID, tab browser.BrowserTabID) (time.Time, bool) {
	expires, _, allowed := acquireAgentControlWithToken(ref, owner, session, tab)
	return expires, allowed
}

func acquireAgentControlWithToken(ref, owner string, session browser.BrowserSessionID, tab browser.BrowserTabID) (time.Time, string, bool) {
	if ref == "" || owner == "" || session == "" || tab == "" {
		return time.Time{}, "", false
	}
	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		return time.Time{}, "", false
	}
	token := base64.RawURLEncoding.EncodeToString(secret)
	now := time.Now()
	key := agentControlTabKey(session, tab)
	agentControlLeases.Lock()
	defer agentControlLeases.Unlock()
	if agentControlLeases.agentInFlight[key] > 0 {
		return time.Time{}, "", false
	}
	if agentControlLeases.manualInFlight[key] > 0 {
		return time.Time{}, "", false
	}
	if current, ok := agentControlLeases.items[key]; ok && current.deadline.After(now) &&
		(current.ownerKey != owner || current.browserRef != ref) {
		return time.Time{}, "", false
	}
	expires := now.Add(agentControlLeaseTTL)
	agentControlLeases.items[key] = agentControlLease{ownerKey: owner, browserRef: ref, leaseID: token, deadline: expires}
	return expires, token, true
}

func renewAgentControl(ref, owner, token string, session browser.BrowserSessionID, tab browser.BrowserTabID) (time.Time, bool) {
	if token == "" {
		return time.Time{}, false
	}
	key := agentControlTabKey(session, tab)
	agentControlLeases.Lock()
	defer agentControlLeases.Unlock()
	current, ok := agentControlLeases.items[key]
	if !ok || current.ownerKey != owner || current.browserRef != ref || current.leaseID != token || !current.deadline.After(time.Now()) {
		return time.Time{}, false
	}
	current.deadline = time.Now().Add(agentControlLeaseTTL)
	agentControlLeases.items[key] = current
	return current.deadline, true
}

func releaseAgentControlWithToken(ref, owner, token string, session browser.BrowserSessionID, tab browser.BrowserTabID) bool {
	if token == "" {
		return false
	}
	key := agentControlTabKey(session, tab)
	agentControlLeases.Lock()
	defer agentControlLeases.Unlock()
	current, ok := agentControlLeases.items[key]
	if !ok || current.ownerKey != owner || current.browserRef != ref || current.leaseID != token {
		return false
	}
	delete(agentControlLeases.items, key)
	return true
}

func agentControlLeaseToken(ref, owner string, session browser.BrowserSessionID, tab browser.BrowserTabID) string {
	key := agentControlTabKey(session, tab)
	agentControlLeases.Lock()
	defer agentControlLeases.Unlock()
	current, ok := agentControlLeases.items[key]
	if !ok || current.ownerKey != owner || current.browserRef != ref || !current.deadline.After(time.Now()) {
		return ""
	}
	return current.leaseID
}

func releaseAgentControl(ref, owner string, session browser.BrowserSessionID, tab browser.BrowserTabID) bool {
	key := agentControlTabKey(session, tab)
	agentControlLeases.Lock()
	defer agentControlLeases.Unlock()
	current, ok := agentControlLeases.items[key]
	if !ok || current.browserRef != ref || current.ownerKey != owner {
		return false
	}
	delete(agentControlLeases.items, key)
	return true
}

func revokeAgentControl(ref string) {
	agentControlLeases.Lock()
	defer agentControlLeases.Unlock()
	for key, lease := range agentControlLeases.items {
		if lease.browserRef == ref {
			delete(agentControlLeases.items, key)
		}
	}
}

func hasAgentControl(ref, owner string, session browser.BrowserSessionID, tab browser.BrowserTabID) bool {
	key := agentControlTabKey(session, tab)
	agentControlLeases.Lock()
	defer agentControlLeases.Unlock()
	current, ok := agentControlLeases.items[key]
	if !ok {
		return false
	}
	if !current.deadline.After(time.Now()) {
		delete(agentControlLeases.items, key)
		return false
	}
	return current.browserRef == ref && current.ownerKey == owner
}

func BeginManualTabMutation(ref, owner, token string, session browser.BrowserSessionID, tab browser.BrowserTabID) (func(), bool) {
	if ref == "" || owner == "" || token == "" || session == "" || tab == "" {
		return nil, false
	}
	key := agentControlTabKey(session, tab)
	agentControlLeases.Lock()
	lease, ok := agentControlLeases.items[key]
	if !ok || !lease.deadline.After(time.Now()) || lease.browserRef != ref || lease.ownerKey != owner || lease.leaseID != token || agentControlLeases.agentInFlight[key] > 0 || agentControlLeases.manualInFlight[key] > 0 {
		agentControlLeases.Unlock()
		return nil, false
	}
	agentControlLeases.manualInFlight[key]++
	agentControlLeases.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			agentControlLeases.Lock()
			if agentControlLeases.manualInFlight[key] <= 1 {
				delete(agentControlLeases.manualInFlight, key)
			} else {
				agentControlLeases.manualInFlight[key]--
			}
			agentControlLeases.Unlock()
		})
	}, true
}

func currentAgentControl(session browser.BrowserSessionID, tab browser.BrowserTabID) bool {
	key := agentControlTabKey(session, tab)
	agentControlLeases.Lock()
	defer agentControlLeases.Unlock()
	current, ok := agentControlLeases.items[key]
	if agentControlLeases.manualInFlight[key] > 0 {
		return true
	}
	if !ok {
		return false
	}
	if !current.deadline.After(time.Now()) {
		delete(agentControlLeases.items, key)
		return false
	}
	return true
}

// WaitForAgentTabControl waits only at a browser tool boundary. Agents do not
// acquire user control and cannot prolong it. Cancellation is respected.
func WaitForAgentTabControl(ctx context.Context, session browser.BrowserSessionID, tab browser.BrowserTabID) error {
	if session == "" || tab == "" {
		return nil
	}
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	for currentAgentControl(session, tab) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
	return nil
}

// BeginAgentTabMutation atomically enters a mutating tool call once there is
// no live manual lease. A user cannot acquire a lease while a tool is in flight.
// The caller MUST invoke finish, including if the tool returns an error.
func BeginAgentTabMutation(ctx context.Context, session browser.BrowserSessionID, tab browser.BrowserTabID) (func(), error) {
	if session == "" || tab == "" {
		return func() {}, nil
	}
	key := agentControlTabKey(session, tab)
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		agentControlLeases.Lock()
		lease, exists := agentControlLeases.items[key]
		if agentControlLeases.agentInFlight[key] == 0 && agentControlLeases.manualInFlight[key] == 0 && (!exists || !lease.deadline.After(time.Now())) {
			if exists {
				delete(agentControlLeases.items, key)
			}
			agentControlLeases.agentInFlight[key]++
			agentControlLeases.Unlock()
			var once sync.Once
			return func() {
				once.Do(func() {
					agentControlLeases.Lock()
					if agentControlLeases.agentInFlight[key] <= 1 {
						delete(agentControlLeases.agentInFlight, key)
					} else {
						agentControlLeases.agentInFlight[key]--
					}
					agentControlLeases.Unlock()
				})
			}, nil
		}
		agentControlLeases.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
