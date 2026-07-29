package sandbox_webui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

type WebCircuitState string

const (
	CircuitClosed   WebCircuitState = "closed"
	CircuitOpen     WebCircuitState = "open"
	CircuitHalfOpen WebCircuitState = "half_open"
)

type CrashRecord struct {
	Timestamp time.Time
	Reason    string
	Count     int
}

type CircuitBreaker struct {
	mu               sync.Mutex
	state            WebCircuitState
	failureCount     int
	failureThreshold int
	windowDuration   time.Duration
	lastFailure      time.Time
	cooldown         time.Duration
	records          []CrashRecord
}

func NewCircuitBreaker(threshold int, window, cooldown time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            CircuitClosed,
		failureThreshold: threshold,
		windowDuration:   window,
		cooldown:         cooldown,
		records:          make([]CrashRecord, 0, 16),
	}
}

func (c *CircuitBreaker) RecordFailure(reason string) WebCircuitState {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now().UTC()
	c.records = append(c.records, CrashRecord{Timestamp: now, Reason: reason, Count: 1})
	cutoff := now.Add(-c.windowDuration)
	pruned := c.records[:0]
	for _, r := range c.records {
		if r.Timestamp.After(cutoff) {
			pruned = append(pruned, r)
		}
	}
	c.records = pruned
	c.failureCount = len(pruned)
	c.lastFailure = now
	if c.failureCount >= c.failureThreshold {
		c.state = CircuitOpen
	} else if c.state == CircuitHalfOpen {
		c.state = CircuitOpen
	}
	return c.state
}

func (c *CircuitBreaker) RecordSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == CircuitHalfOpen {
		c.state = CircuitClosed
		c.failureCount = 0
		c.records = c.records[:0]
	}
}

func (c *CircuitBreaker) Allow() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == CircuitClosed {
		return true
	}
	if c.state == CircuitOpen {
		if time.Since(c.lastFailure) > c.cooldown {
			c.state = CircuitHalfOpen
			return true
		}
		return false
	}
	return true
}

func (c *CircuitBreaker) State() WebCircuitState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

type LifecycleManager struct {
	mu             sync.RWMutex
	host           *Host
	breakers       map[string]*CircuitBreaker
	maxCrashes     int
	windowDuration time.Duration
	cooldown       time.Duration
}

func NewLifecycleManager(host *Host) *LifecycleManager {
	return &LifecycleManager{
		host:           host,
		breakers:       make(map[string]*CircuitBreaker),
		maxCrashes:     5,
		windowDuration: 10 * time.Minute,
		cooldown:       1 * time.Minute,
	}
}

func (m *LifecycleManager) getOrCreateBreaker(sessionID string) *CircuitBreaker {
	m.mu.Lock()
	defer m.mu.Unlock()
	if b, exists := m.breakers[sessionID]; exists {
		return b
	}
	b := NewCircuitBreaker(m.maxCrashes, m.windowDuration, m.cooldown)
	m.breakers[sessionID] = b
	return b
}

type CrashReport struct {
	SessionID string
	Reason    string
	Stack     string
}

func (m *LifecycleManager) HandleCrash(report CrashReport) error {
	session, err := m.host.GetSession(report.SessionID)
	if err != nil {
		return err
	}
	breaker := m.getOrCreateBreaker(report.SessionID)
	state := breaker.RecordFailure(report.Reason)
	if state == CircuitOpen {
		session.SetState(SessionStateFailed)
		_ = m.host.CloseSession(report.SessionID, "circuit_open")
		return ErrCircuitOpen
	}
	session.SetState(SessionStateSuspended)
	return nil
}

func (m *LifecycleManager) AllowReload(sessionID string) bool {
	breaker := m.getOrCreateBreaker(sessionID)
	return breaker.Allow()
}

func (m *LifecycleManager) RecordSuccess(sessionID string) {
	breaker := m.getOrCreateBreaker(sessionID)
	breaker.RecordSuccess()
}

type PerformanceMonitor struct {
	mu            sync.RWMutex
	snapshots     map[string][]PerformanceSnapshot
	maxPerSession int
}

type PerformanceSnapshot struct {
	Timestamp      time.Time
	CPUPercent     float64
	MemoryBytes    int64
	DOMNodes       int
	FrameRate      int
	BundleBytes    int64
	MessageRate    int
	BackendVisible bool
}

func NewPerformanceMonitor() *PerformanceMonitor {
	return &PerformanceMonitor{
		snapshots:     make(map[string][]PerformanceSnapshot),
		maxPerSession: 60,
	}
}

func (pm *PerformanceMonitor) Record(sessionID string, snapshot PerformanceSnapshot) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	list := pm.snapshots[sessionID]
	if len(list) >= pm.maxPerSession {
		list = list[1:]
	}
	list = append(list, snapshot)
	pm.snapshots[sessionID] = list
}

type PerformanceBudget struct {
	MaxBundleBytes  int64
	MaxMemoryBytes  int64
	MaxCPUPercent   float64
	MaxDOMNodes     int
	MinFrameRate    int
	MaxMessageRate  int
	MaxHiddenPeriod time.Duration
}

func DefaultPerformanceBudget() PerformanceBudget {
	return PerformanceBudget{
		MaxBundleBytes:  MaxBundleBytes,
		MaxMemoryBytes:  256 * 1024 * 1024,
		MaxCPUPercent:   80,
		MaxDOMNodes:     5000,
		MinFrameRate:    30,
		MaxMessageRate:  100,
		MaxHiddenPeriod: 30 * time.Second,
	}
}

func (pm *PerformanceMonitor) CheckBudget(sessionID string, budget PerformanceBudget) (violations []string) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	list := pm.snapshots[sessionID]
	if len(list) == 0 {
		return nil
	}
	latest := list[len(list)-1]
	if latest.BundleBytes > budget.MaxBundleBytes {
		violations = append(violations, "bundle_size_exceeded")
	}
	if latest.MemoryBytes > budget.MaxMemoryBytes {
		violations = append(violations, "memory_exceeded")
	}
	if latest.CPUPercent > budget.MaxCPUPercent {
		violations = append(violations, "cpu_exceeded")
	}
	if latest.DOMNodes > budget.MaxDOMNodes {
		violations = append(violations, "dom_nodes_exceeded")
	}
	if latest.FrameRate > 0 && latest.FrameRate < budget.MinFrameRate {
		violations = append(violations, "frame_rate_low")
	}
	if latest.MessageRate > budget.MaxMessageRate {
		violations = append(violations, "message_rate_exceeded")
	}
	return violations
}

type PreloadBuilder struct {
	allowedMethods map[BridgeMethod]bool
}

func NewPreloadBuilder() *PreloadBuilder {
	return &PreloadBuilder{allowedMethods: allowedMethods}
}

func (pb *PreloadBuilder) Build(session *WebSession) (string, error) {
	preload := `
window.amitiaUI = (function() {
  const sessionId = "` + session.SessionID + `";
  const origin = "` + session.Origin + `";
  const nonce = "` + session.Nonce + `";
  const token = "` + session.Token + `";
  const generation = ` + fmt.Sprintf("%d", session.Generation) + `;
  const contributionId = "` + session.ContributionID + `";
  const protocolVersion = "` + ProtocolVersion + `";
  const allowedMethods = ["ready","context.get","action.invoke","data.query","data.subscribe","navigation.request","resize.request","dialog.request","resource.open","resource.read","artifact.create","log","session.ping","clipboard.read","clipboard.write","network.request","storage"];
  let port = null;
  let pendingMessages = [];
  let pendingCallbacks = new Map();
  let readyCallbacks = [];
  let isReady = false;

  function validateMethod(name) {
    if (!allowedMethods.includes(name)) {
      throw new Error("Method not allowed: " + name);
    }
  }

  function flushPending() {
    if (port) {
      for (const msg of pendingMessages) {
        port.postMessage(msg);
      }
      pendingMessages = [];
    }
  }

  function postMessage(method, input, callback) {
    validateMethod(method);
    const id = crypto.randomUUID();
    const msg = {
      method: "ui." + method,
      version: 1,
      id: id,
      origin: origin,
      nonce: nonce,
      token: token,
      generation: generation,
      session: sessionId,
      input: input || null,
      timestamp: Date.now()
    };
    if (callback) {
      pendingCallbacks.set(id, callback);
    }
    if (port) {
      port.postMessage(msg);
    } else {
      pendingMessages.push(msg);
    }
    return id;
  }

  function call(method, input) {
    return new Promise(function(resolve, reject) {
      postMessage(method, input, function(err, result) {
        if (err) { reject(err); }
        else { resolve(result); }
      });
    });
  }

  function handlePortMessage(event) {
    if (!event.data || typeof event.data !== "object") return;
    var data = event.data;
    if (data.type === "bridge.response") {
      var cb = pendingCallbacks.get(data.id);
      if (cb) {
        pendingCallbacks.delete(data.id);
        if (data.error) { cb(new Error(data.error), null); }
        else { cb(null, data.output || data); }
      }
      return;
    }
  }

  function acceptPort(event) {
    if (event.source !== window.parent || port || !event.data || event.data.type !== "amitia.extension.init") return;
    if (event.data.session !== sessionId || event.data.nonce !== nonce || event.data.token !== token || event.data.generation !== generation) return;
    if (!event.ports || event.ports.length !== 1) return;
    port = event.ports[0];
    port.onmessage = handlePortMessage;
    port.start();
    window.removeEventListener("message", acceptPort);
    flushPending();
  }

  window.addEventListener("message", acceptPort);
  window.parent.postMessage({type:"amitia.extension.ready",protocolVersion:protocolVersion,session:sessionId,nonce:nonce,generation:generation,contributionId:contributionId}, "*");

  var api = {
    ready: function() {
      return new Promise(function(resolve) {
        if (isReady) { resolve(); return; }
        readyCallbacks.push(resolve);
        postMessage("ready", null, function(err, result) {
          isReady = true;
          for (var i = 0; i < readyCallbacks.length; i++) {
            readyCallbacks[i]();
          }
          readyCallbacks = [];
        });
      });
    },
    getContext: function() { return call("context.get", null); },
    invokeAction: function(actionId, input) { return call("action.invoke", {actionId: actionId, input: input}); },
    queryData: function(sourceId, params) { return call("data.query", {sourceId: sourceId, params: params}); },
    subscribeData: function(sourceId, params, rate) { return call("data.subscribe", {sourceId: sourceId, params: params, ratePerMinute: rate}); },
    navigate: function(target, type) { return call("navigation.request", {target: target, type: type}); },
    requestResize: function(width, height) { return call("resize.request", {width: width, height: height}); },
    openResource: function(handleId) { return call("resource.open", {handleId: handleId}); },
    readResource: function(handleId) { return call("resource.read", {handleId: handleId}); },
    createArtifact: function(contentType, data, filename) { return call("artifact.create", {contentType: contentType, data: data, filename: filename}); },
    log: function(level, message) { return call("log", {level: level, message: message}); },
    ping: function() { return call("session.ping", null); },
    onReady: function(cb) {
      if (isReady) { cb(); return; }
      readyCallbacks.push(cb);
    },
    sessionId: sessionId,
    origin: origin,
    generation: generation
  };

  Object.freeze(api);
  return api;
})();
Object.defineProperty(window, 'amitiaUI', { configurable: false, writable: false });
`
	return preload, nil
}

type ResourceHandleFactory struct{}

func NewResourceHandleFactory() *ResourceHandleFactory {
	return &ResourceHandleFactory{}
}

func (f *ResourceHandleFactory) Create(path, mime string, size int64) *ResourceHandle {
	return &ResourceHandle{
		HandleID:  newResourceHandleID(),
		Path:      path,
		MIME:      mime,
		Size:      size,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(MaxResourceHandleTTL),
		ReadOnly:  true,
	}
}

func newResourceHandleID() string {
	b := make([]byte, 12)
	_, _ = readRandom(b)
	return "rh_" + bytesToHex(b)
}

func bytesToHex(b []byte) string {
	const hexChars = "0123456789abcdef"
	result := make([]byte, len(b)*2)
	for i, v := range b {
		result[i*2] = hexChars[v>>4]
		result[i*2+1] = hexChars[v&0x0f]
	}
	return string(result)
}

func readRandom(b []byte) (int, error) {
	return cryptoRandRead(b)
}

type SuspendRequest struct {
	SessionID string
	Reason    string
}

func (m *LifecycleManager) Suspend(ctx context.Context, req SuspendRequest) error {
	session, err := m.host.GetSession(req.SessionID)
	if err != nil {
		return err
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	for _, sub := range session.subscriptions {
		sub.mu.Lock()
		sub.Active = false
		sub.mu.Unlock()
	}
	session.State = SessionStateSuspended
	return nil
}

type ResumeRequest struct {
	SessionID string
}

func (m *LifecycleManager) Resume(ctx context.Context, req ResumeRequest) error {
	session, err := m.host.GetSession(req.SessionID)
	if err != nil {
		return err
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	for _, sub := range session.subscriptions {
		sub.mu.Lock()
		sub.Active = true
		sub.mu.Unlock()
	}
	session.State = SessionStateReady
	m.RecordSuccess(req.SessionID)
	return nil
}

type FailureRecord struct {
	SessionID    string
	Timestamp    time.Time
	Reason       string
	CircuitState WebCircuitState
	Recovered    bool
}

type FailureStore struct {
	mu      sync.RWMutex
	records []FailureRecord
	maxSize int
}

func NewFailureStore() *FailureStore {
	return &FailureStore{
		records: make([]FailureRecord, 0, 128),
		maxSize: 1024,
	}
}

func (s *FailureStore) Record(record FailureRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.records) >= s.maxSize {
		s.records = s.records[1:]
	}
	s.records = append(s.records, record)
}

func (s *FailureStore) List(sessionID string) []FailureRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]FailureRecord, 0)
	for _, r := range s.records {
		if sessionID == "" || r.SessionID == sessionID {
			out = append(out, r)
		}
	}
	return out
}

var (
	ErrCircuitOpen = errors.New("sandbox_webui: circuit open")
)

var _ = json.Marshal
var _ = context.Background
