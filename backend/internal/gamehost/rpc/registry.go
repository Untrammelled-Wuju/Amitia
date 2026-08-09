package rpc

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type NamespaceRegistry interface {
	Register(
		ctx context.Context,
		route Route,
	) error

	Unregister(
		ctx context.Context,
		runtimeID domain.RuntimeInstanceID,
		namespace Namespace,
	) error

	UnregisterByService(
		ctx context.Context,
		runtimeID domain.RuntimeInstanceID,
		serviceID domain.ServiceID,
	) error

	UnregisterByRuntime(
		ctx context.Context,
		runtimeID domain.RuntimeInstanceID,
	) error

	Resolve(
		ctx context.Context,
		runtimeID domain.RuntimeInstanceID,
		method Method,
	) (Route, error)

	List(
		ctx context.Context,
		runtimeID domain.RuntimeInstanceID,
	) ([]Route, error)

	ListByService(
		ctx context.Context,
		runtimeID domain.RuntimeInstanceID,
		serviceID domain.ServiceID,
	) ([]Route, error)
}

type RuntimeValidator interface {
	ValidateRuntime(
		ctx context.Context,
		runtimeID domain.RuntimeInstanceID,
	) error

	ValidateService(
		ctx context.Context,
		runtimeID domain.RuntimeInstanceID,
		serviceID domain.ServiceID,
		expectedPluginID domain.PluginID,
	) error
}

type namespaceRegistry struct {
	mu       sync.RWMutex
	routes   map[RouteKey]Route
	byService map[ServiceKey]RouteKey
	validator RuntimeValidator
	maxNamespacesPerRuntime int
}

type NamespaceRegistryConfig struct {
	Validator             RuntimeValidator
	MaxNamespacesPerRuntime int
}

func NewNamespaceRegistry(config NamespaceRegistryConfig) NamespaceRegistry {
	maxNS := config.MaxNamespacesPerRuntime
	if maxNS <= 0 {
		maxNS = 1024
	}
	return &namespaceRegistry{
		routes:   make(map[RouteKey]Route),
		byService: make(map[ServiceKey]RouteKey),
		validator: config.Validator,
		maxNamespacesPerRuntime: maxNS,
	}
}

func (r *namespaceRegistry) Register(ctx context.Context, route Route) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := route.Validate(); err != nil {
		return NewRPCErrorWithCause(RPCErrorMethodNotFound, domain.ErrInvalidArgument, "route validation failed", err)
	}

	if err := ValidateCustomNamespace(route.Namespace); err != nil {
		return NewRPCErrorWithCause(RPCErrorReservedNamespace, domain.ErrInvalidArgument, "invalid custom namespace", err)
	}

	if r.validator != nil {
		if err := r.validator.ValidateRuntime(ctx, route.RuntimeID); err != nil {
			return NewRPCErrorWithCause(RPCErrorMethodNotFound, domain.ErrRuntimeUnavailable, "runtime validation failed", err)
		}
		if err := r.validator.ValidateService(ctx, route.RuntimeID, route.ServiceID, route.PluginID); err != nil {
			return NewRPCErrorWithCause(RPCErrorMethodNotFound, domain.ErrNotFound, "service validation failed", err)
		}
	}

	key := route.OwnerKey()

	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, exists := r.routes[key]; exists {
		if existing.ServiceID == route.ServiceID && existing.PluginID == route.PluginID {
			return nil
		}
		return NewRPCErrorWithCause(
			RPCErrorNamespaceConflict,
			domain.ErrAlreadyExists,
			fmt.Sprintf("namespace %q already registered by service %s", route.Namespace, existing.ServiceID),
			nil,
		)
	}

	count := 0
	for k := range r.routes {
		if k.RuntimeID == route.RuntimeID {
			count++
		}
	}
	if count >= r.maxNamespacesPerRuntime {
		return NewRPCErrorWithCause(
			RPCErrorNamespaceConflict,
			domain.ErrResourceExhausted,
			fmt.Sprintf("runtime %s has reached max namespaces limit", route.RuntimeID),
			nil,
		)
	}

	r.routes[key] = route
	r.byService[route.ServiceKey()] = key
	return nil
}

func (r *namespaceRegistry) Unregister(ctx context.Context, runtimeID domain.RuntimeInstanceID, namespace Namespace) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	key := RouteKey{
		RuntimeID: runtimeID,
		Namespace: namespace,
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	route, exists := r.routes[key]
	if !exists {
		return nil
	}

	delete(r.routes, key)
	delete(r.byService, route.ServiceKey())
	return nil
}

func (r *namespaceRegistry) UnregisterByService(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	prefix := ServiceKey{
		RuntimeID: runtimeID,
		ServiceID: serviceID,
	}

	var toRemove []RouteKey
	for sk, rk := range r.byService {
		if sk == prefix {
			toRemove = append(toRemove, rk)
		}
	}

	for _, rk := range toRemove {
		if route, ok := r.routes[rk]; ok {
			delete(r.routes, rk)
			delete(r.byService, route.ServiceKey())
		}
	}

	return nil
}

func (r *namespaceRegistry) UnregisterByRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	var toRemove []RouteKey
	for k, route := range r.routes {
		if k.RuntimeID == runtimeID {
			toRemove = append(toRemove, k)
			delete(r.byService, route.ServiceKey())
		}
	}

	for _, k := range toRemove {
		delete(r.routes, k)
	}

	return nil
}

func (r *namespaceRegistry) Resolve(ctx context.Context, runtimeID domain.RuntimeInstanceID, method Method) (Route, error) {
	if err := ctx.Err(); err != nil {
		return Route{}, err
	}

	namespace := NamespaceOfMethod(method)

	r.mu.RLock()
	defer r.mu.RUnlock()

	key := RouteKey{
		RuntimeID: runtimeID,
		Namespace: namespace,
	}

	route, exists := r.routes[key]
	if !exists {
		return Route{}, NewRPCErrorWithCause(
			RPCErrorMethodNotFound,
			domain.ErrNotFound,
			fmt.Sprintf("namespace %q not found in runtime %s", namespace, runtimeID),
			nil,
		)
	}

	return route.Clone(), nil
}

func (r *namespaceRegistry) List(ctx context.Context, runtimeID domain.RuntimeInstanceID) ([]Route, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Route
	for k, route := range r.routes {
		if k.RuntimeID == runtimeID {
			result = append(result, route.Clone())
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Namespace != result[j].Namespace {
			return result[i].Namespace < result[j].Namespace
		}
		return result[i].ServiceID < result[j].ServiceID
	})

	return result, nil
}

func (r *namespaceRegistry) ListByService(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) ([]Route, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.Unlock()

	var result []Route
	prefix := ServiceKey{
		RuntimeID: runtimeID,
		ServiceID: serviceID,
	}
	for sk, rk := range r.byService {
		if sk == prefix {
			if route, ok := r.routes[rk]; ok {
				result = append(result, route.Clone())
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Namespace != result[j].Namespace {
			return result[i].Namespace < result[j].Namespace
		}
		return result[i].ServiceID < result[j].ServiceID
	})

	return result, nil
}

func (r Route) Clone() Route {
	return Route{
		RuntimeID: r.RuntimeID,
		PluginID:  r.PluginID,
		ServiceID: r.ServiceID,
		Namespace: r.Namespace,
	}
}
