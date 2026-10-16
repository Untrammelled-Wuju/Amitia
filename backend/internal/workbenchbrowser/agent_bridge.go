package workbenchbrowser

import (
	"crypto/rand"
	"encoding/base64"
	"regexp"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/browser"
)

type exposedAgentTab struct {
	spaceID        string
	conversationID string
	session        browser.BrowserSessionID
	tab            browser.BrowserTabID
	expires        time.Time
}

var agentTabs = struct {
	sync.Mutex
	items map[string]exposedAgentTab
}{items: make(map[string]exposedAgentTab)}

const agentTabTTL = 6 * time.Hour
const maxExposedAgentTabs = 256

var browserReferencePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{32}$`)

func validBrowserRef(ref string) bool { return browserReferencePattern.MatchString(ref) }

func ExposeAgentTab(spaceID, conversationID string, session browser.BrowserSessionID, tab browser.BrowserTabID) string {
	if spaceID == "" || !validIdentifier.MatchString(conversationID) || session == "" || tab == "" {
		return ""
	}
	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		return ""
	}
	ref := base64.RawURLEncoding.EncodeToString(secret)
	agentTabs.Lock()
	defer agentTabs.Unlock()
	now := time.Now()
	for key, record := range agentTabs.items {
		if now.After(record.expires) {
			delete(agentTabs.items, key)
		}
	}
	if len(agentTabs.items) >= maxExposedAgentTabs {
		return ""
	}
	agentTabs.items[ref] = exposedAgentTab{spaceID: spaceID, conversationID: conversationID, session: session, tab: tab, expires: now.Add(agentTabTTL)}
	return ref
}

func ResolveAgentTab(spaceID, conversationID, ref string) (browser.BrowserSessionID, browser.BrowserTabID, bool) {
	agentTabs.Lock()
	defer agentTabs.Unlock()
	if !validBrowserRef(ref) {
		return "", "", false
	}
	record, ok := agentTabs.items[ref]
	if !ok || time.Now().After(record.expires) || record.spaceID != spaceID || record.conversationID != conversationID {
		return "", "", false
	}
	return record.session, record.tab, true
}

func RevokeAgentTab(ref string) {
	if ref == "" {
		return
	}
	agentTabs.Lock()
	delete(agentTabs.items, ref)
	agentTabs.Unlock()
	revokeAgentControl(ref)
}
