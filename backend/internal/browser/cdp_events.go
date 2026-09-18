package browser

import (
	"encoding/json"
	"sync"
	"sync/atomic"
)

type cdpEventDispatcher struct {
	mu             sync.RWMutex
	handlers       map[string][]cdpEventHandler
	general        []cdpGeneralHandler
	sessionGeneral []cdpSessionGeneralHandler
	closed         int32
	nextID         uint64
}

type cdpEventHandler struct {
	id      uint64
	handler func(json.RawMessage)
}

type cdpGeneralHandler struct {
	id      uint64
	handler func(string, json.RawMessage)
}

type cdpSessionGeneralHandler struct {
	id      uint64
	handler func(string, string, json.RawMessage)
}

func newCDPEventDispatcher() *cdpEventDispatcher {
	return &cdpEventDispatcher{handlers: make(map[string][]cdpEventHandler)}
}

func (d *cdpEventDispatcher) subscribe(method string, handler func(json.RawMessage)) func() {
	id := atomic.AddUint64(&d.nextID, 1)
	d.mu.Lock()
	d.handlers[method] = append(d.handlers[method], cdpEventHandler{id: id, handler: handler})
	d.mu.Unlock()
	return func() { d.unsubscribeMethod(method, id) }
}

func (d *cdpEventDispatcher) subscribeAll(handler func(string, json.RawMessage)) func() {
	id := atomic.AddUint64(&d.nextID, 1)
	d.mu.Lock()
	d.general = append(d.general, cdpGeneralHandler{id: id, handler: handler})
	d.mu.Unlock()
	return func() { d.unsubscribeGeneral(id) }
}

func (d *cdpEventDispatcher) subscribeAllWithSession(handler func(string, string, json.RawMessage)) func() {
	id := atomic.AddUint64(&d.nextID, 1)
	d.mu.Lock()
	d.sessionGeneral = append(d.sessionGeneral, cdpSessionGeneralHandler{id: id, handler: handler})
	d.mu.Unlock()
	return func() { d.unsubscribeSessionGeneral(id) }
}

func (d *cdpEventDispatcher) unsubscribeMethod(method string, id uint64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	hs := d.handlers[method]
	for i, h := range hs {
		if h.id == id {
			d.handlers[method] = append(hs[:i], hs[i+1:]...)
			break
		}
	}
	if len(d.handlers[method]) == 0 {
		delete(d.handlers, method)
	}
}

func (d *cdpEventDispatcher) unsubscribeGeneral(id uint64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, h := range d.general {
		if h.id == id {
			d.general = append(d.general[:i], d.general[i+1:]...)
			break
		}
	}
}

func (d *cdpEventDispatcher) unsubscribeSessionGeneral(id uint64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, h := range d.sessionGeneral {
		if h.id == id {
			d.sessionGeneral = append(d.sessionGeneral[:i], d.sessionGeneral[i+1:]...)
			break
		}
	}
}

func (d *cdpEventDispatcher) dispatch(method, sessionID string, params json.RawMessage) {
	if atomic.LoadInt32(&d.closed) == 1 {
		return
	}
	d.mu.RLock()
	handlers := make([]func(json.RawMessage), 0, len(d.handlers[method]))
	for _, h := range d.handlers[method] {
		handlers = append(handlers, h.handler)
	}
	generals := make([]func(string, json.RawMessage), 0, len(d.general))
	for _, h := range d.general {
		generals = append(generals, h.handler)
	}
	sessionGenerals := make([]func(string, string, json.RawMessage), 0, len(d.sessionGeneral))
	for _, h := range d.sessionGeneral {
		sessionGenerals = append(sessionGenerals, h.handler)
	}
	d.mu.RUnlock()
	for _, h := range handlers {
		safeCallEvent(h, params)
	}
	for _, g := range generals {
		safeCallGeneral(g, method, params)
	}
	for _, g := range sessionGenerals {
		safeCallSessionGeneral(g, method, sessionID, params)
	}
}

func (d *cdpEventDispatcher) close() {
	atomic.StoreInt32(&d.closed, 1)
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers = make(map[string][]cdpEventHandler)
	d.general = nil
	d.sessionGeneral = nil
}

func safeCallEvent(h func(json.RawMessage), p json.RawMessage) {
	defer func() { _ = recover() }()
	h(p)
}

func safeCallGeneral(h func(string, json.RawMessage), m string, p json.RawMessage) {
	defer func() { _ = recover() }()
	h(m, p)
}

func safeCallSessionGeneral(h func(string, string, json.RawMessage), m, sessionID string, p json.RawMessage) {
	defer func() { _ = recover() }()
	h(m, sessionID, p)
}
