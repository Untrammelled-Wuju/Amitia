package mcp

import (
	"context"
	"errors"
	"sync"
)

type managedConnection interface {
	State() MCPStdioServerState
	Start(context.Context) error
	Close(context.Context) error
}

type connectionOperation struct {
	done        chan struct{}
	cancelStart context.CancelFunc
}

type connectionRegistry[T managedConnection] struct {
	mu          sync.Mutex
	connections map[string]T
	operations  map[string]connectionOperation
	draining    chan struct{}
}

func newConnectionRegistry[T managedConnection]() *connectionRegistry[T] {
	return &connectionRegistry[T]{connections: make(map[string]T), operations: make(map[string]connectionOperation)}
}

func (r *connectionRegistry[T]) claim(ctx context.Context, id string, drain chan struct{}, cancelStart context.CancelFunc) (func(), error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		r.mu.Lock()
		wait := r.operations[id].done
		if r.draining != nil && r.draining != drain {
			wait = r.draining
		}
		if wait == nil {
			done := make(chan struct{})
			r.operations[id] = connectionOperation{done: done, cancelStart: cancelStart}
			r.mu.Unlock()
			return func() {
				r.mu.Lock()
				delete(r.operations, id)
				close(done)
				r.mu.Unlock()
			}, nil
		}
		r.mu.Unlock()
		select {
		case <-wait:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (r *connectionRegistry[T]) start(ctx context.Context, id string, create func(context.Context) (T, error)) (T, error) {
	var zero T
	startCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	release, err := r.claim(startCtx, id, nil, cancel)
	if err != nil {
		return zero, err
	}
	defer release()
	if existing, ok := r.get(id); ok {
		if existing.State() == MCPStdioStateReady {
			return existing, nil
		}
		if err := r.remove(startCtx, id); err != nil {
			return zero, err
		}
	}
	conn, err := create(startCtx)
	if err != nil {
		return zero, err
	}
	if err := conn.Start(startCtx); err != nil {
		return zero, err
	}
	r.mu.Lock()
	err = startCtx.Err()
	if err == nil {
		r.connections[id] = conn
	}
	r.mu.Unlock()
	if err != nil {
		return zero, errors.Join(err, conn.Close(startCtx))
	}
	return conn, nil
}

func (r *connectionRegistry[T]) get(id string) (T, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	conn, ok := r.connections[id]
	return conn, ok
}

func (r *connectionRegistry[T]) close(ctx context.Context, id string, drain chan struct{}) error {
	release, err := r.claim(ctx, id, drain, nil)
	if err != nil {
		return err
	}
	defer release()
	return r.remove(ctx, id)
}

func (r *connectionRegistry[T]) remove(ctx context.Context, id string) error {
	r.mu.Lock()
	conn, ok := r.connections[id]
	delete(r.connections, id)
	r.mu.Unlock()
	if !ok {
		return nil
	}
	return conn.Close(ctx)
}

func (r *connectionRegistry[T]) closeAll(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	for r.draining != nil {
		drain := r.draining
		r.mu.Unlock()
		select {
		case <-drain:
		case <-ctx.Done():
			return ctx.Err()
		}
		r.mu.Lock()
	}
	drain := make(chan struct{})
	r.draining = drain
	ids := make(map[string]struct{}, len(r.connections)+len(r.operations))
	for id := range r.connections {
		ids[id] = struct{}{}
	}
	for id, operation := range r.operations {
		ids[id] = struct{}{}
		if operation.cancelStart != nil {
			operation.cancelStart()
		}
	}
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.draining = nil
		close(drain)
		r.mu.Unlock()
	}()

	var closeErrors []error
	for id := range ids {
		if err := r.close(ctx, id, drain); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	return errors.Join(closeErrors...)
}

func (r *connectionRegistry[T]) list() map[string]T {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make(map[string]T, len(r.connections))
	for id, conn := range r.connections {
		result[id] = conn
	}
	return result
}
