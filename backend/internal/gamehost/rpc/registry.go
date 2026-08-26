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

	UnregisterByConnection(
		ctx context.Context,
		runtimeID domain.RuntimeInstanceID,
		serviceID domain.ServiceID,
		connectionID string,
	) error

	ReconcileService(
		ctx context.Context,
		pluginID domain.PluginID,
		runtimeID domain.RuntimeInstanceID,
		serviceID domain.ServiceID,
		connectionID string,
		namespaces []Namespace,
	) (NamespaceReconcileResult, error)

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

type NamespaceReconcileResult struct {
	Registered []Namespace
	Reused     []Namespace
	Removed    []Namespace
}

type namespaceRegistry struct {
	mu                      sync.RWMutex
	routes                  map[RouteKey]Route
	byService               map[ServiceKey]map[RouteKey]struct{}
	validator               RuntimeValidator
	maxNamespacesPerRuntime int
}

type NamespaceRegistryConfig struct {
	Validator               RuntimeValidator
	MaxNamespacesPerRuntime int
}

func NewNamespaceRegistry(config NamespaceRegistryConfig) NamespaceRegistry {
	maxNS := config.MaxNamespacesPerRuntime
	if maxNS <= 0 {
		maxNS = 1024
	}
	return &namespaceRegistry{
		routes:                  make(map[RouteKey]Route),
		byService:               make(map[ServiceKey]map[RouteKey]struct{}),
		validator:               config.Validator,
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
	serviceKey := route.ServiceKey()
	if r.byService[serviceKey] == nil {
		r.byService[serviceKey] = make(map[RouteKey]struct{})
	}
	r.byService[serviceKey][key] = struct{}{}
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
	serviceKey := route.ServiceKey()
	if keys := r.byService[serviceKey]; keys != nil {
		delete(keys, key)
		if len(keys) == 0 {
			delete(r.byService, serviceKey)
		}
	}
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

	keys := r.byService[prefix]
	for rk := range keys {
		delete(r.routes, rk)
	}
	delete(r.byService, prefix)

	return nil
}

func (r *namespaceRegistry) UnregisterByRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	var toRemove []RouteKey
	for k := range r.routes {
		if k.RuntimeID == runtimeID {
			toRemove = append(toRemove, k)
		}
	}

	for _, k := range toRemove {
		route := r.routes[k]
		delete(r.routes, k)
		serviceKey := route.ServiceKey()
		if keys := r.byService[serviceKey]; keys != nil {
			delete(keys, k)
			if len(keys) == 0 {
				delete(r.byService, serviceKey)
			}
		}
	}

	return nil
}

func (r *namespaceRegistry) UnregisterByConnection(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, connectionID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if connectionID == "" {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	serviceKey := ServiceKey{RuntimeID: runtimeID, ServiceID: serviceID}
	for key := range r.byService[serviceKey] {
		route, ok := r.routes[key]
		if !ok || route.ConnectionID != connectionID {
			continue
		}
		delete(r.routes, key)
		delete(r.byService[serviceKey], key)
	}
	if len(r.byService[serviceKey]) == 0 {
		delete(r.byService, serviceKey)
	}
	return nil
}

// ReconcileService atomically converges one service's custom namespaces to the
// hello advertisement and transfers route ownership to the current connection.
// Conflict detection, stale removal and registration are performed under one
// lock so a failed handshake cannot leave a partially-mutated routing table.
func (r *namespaceRegistry) ReconcileService(
	ctx context.Context,
	pluginID domain.PluginID,
	runtimeID domain.RuntimeInstanceID,
	serviceID domain.ServiceID,
	connectionID string,
	namespaces []Namespace,
) (NamespaceReconcileResult, error) {
	result := NamespaceReconcileResult{
		Registered: make([]Namespace, 0),
		Reused:     make([]Namespace, 0),
		Removed:    make([]Namespace, 0),
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if pluginID == "" || runtimeID == "" || serviceID == "" || connectionID == "" {
		return result, NewRPCError(RPCErrorNamespaceConflict, domain.ErrInvalidArgument, "plugin, runtime, service and connection ids are required")
	}

	desired := make(map[Namespace]struct{}, len(namespaces))
	for _, namespace := range namespaces {
		if err := ValidateCustomNamespace(namespace); err != nil {
			return result, NewRPCErrorWithCause(RPCErrorReservedNamespace, domain.ErrInvalidArgument, "invalid custom namespace", err)
		}
		if _, duplicate := desired[namespace]; duplicate {
			return result, NewRPCError(RPCErrorNamespaceConflict, domain.ErrInvalidArgument, fmt.Sprintf("duplicate namespace %q", namespace))
		}
		desired[namespace] = struct{}{}
	}

	if r.validator != nil {
		if err := r.validator.ValidateRuntime(ctx, runtimeID); err != nil {
			return result, NewRPCErrorWithCause(RPCErrorMethodNotFound, domain.ErrRuntimeUnavailable, "runtime validation failed", err)
		}
		if err := r.validator.ValidateService(ctx, runtimeID, serviceID, pluginID); err != nil {
			return result, NewRPCErrorWithCause(RPCErrorMethodNotFound, domain.ErrNotFound, "service validation failed", err)
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for namespace := range desired {
		key := RouteKey{RuntimeID: runtimeID, Namespace: namespace}
		if existing, exists := r.routes[key]; exists && existing.ServiceID != serviceID {
			return result, NewRPCErrorWithCause(
				RPCErrorNamespaceConflict,
				domain.ErrAlreadyExists,
				fmt.Sprintf("namespace %q already registered by service %s", namespace, existing.ServiceID),
				nil,
			)
		}
	}

	serviceKey := ServiceKey{RuntimeID: runtimeID, ServiceID: serviceID}
	currentKeys := r.byService[serviceKey]
	currentRuntimeCount := 0
	for key := range r.routes {
		if key.RuntimeID == runtimeID {
			currentRuntimeCount++
		}
	}
	staleCount := 0
	for key := range currentKeys {
		if _, keep := desired[key.Namespace]; !keep {
			staleCount++
		}
	}
	missingCount := 0
	for namespace := range desired {
		if _, exists := r.routes[RouteKey{RuntimeID: runtimeID, Namespace: namespace}]; !exists {
			missingCount++
		}
	}
	if currentRuntimeCount-staleCount+missingCount > r.maxNamespacesPerRuntime {
		return result, NewRPCErrorWithCause(
			RPCErrorNamespaceConflict,
			domain.ErrResourceExhausted,
			fmt.Sprintf("runtime %s has reached max namespaces limit", runtimeID),
			nil,
		)
	}

	for key := range currentKeys {
		if _, keep := desired[key.Namespace]; keep {
			continue
		}
		delete(r.routes, key)
		delete(currentKeys, key)
		result.Removed = append(result.Removed, key.Namespace)
	}
	if len(currentKeys) == 0 {
		delete(r.byService, serviceKey)
	}

	for namespace := range desired {
		key := RouteKey{RuntimeID: runtimeID, Namespace: namespace}
		if existing, exists := r.routes[key]; exists {
			existing.PluginID = pluginID
			existing.ServiceID = serviceID
			existing.ConnectionID = connectionID
			r.routes[key] = existing
			if r.byService[serviceKey] == nil {
				r.byService[serviceKey] = make(map[RouteKey]struct{})
			}
			r.byService[serviceKey][key] = struct{}{}
			result.Reused = append(result.Reused, namespace)
			continue
		}
		route := Route{
			RuntimeID: runtimeID, PluginID: pluginID, ServiceID: serviceID,
			Namespace: namespace, ConnectionID: connectionID,
		}
		r.routes[key] = route
		if r.byService[serviceKey] == nil {
			r.byService[serviceKey] = make(map[RouteKey]struct{})
		}
		r.byService[serviceKey][key] = struct{}{}
		result.Registered = append(result.Registered, namespace)
	}

	sort.Slice(result.Registered, func(i, j int) bool { return result.Registered[i] < result.Registered[j] })
	sort.Slice(result.Reused, func(i, j int) bool { return result.Reused[i] < result.Reused[j] })
	sort.Slice(result.Removed, func(i, j int) bool { return result.Removed[i] < result.Removed[j] })
	return result, nil
}

func (r *namespaceRegistry) Resolve(ctx context.Context, runtimeID domain.RuntimeInstanceID, method Method) (Route, error) {
	if err := ctx.Err(); err != nil {
		return Route{}, err
	}

	candidates := NamespaceCandidatesOfMethod(method)

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, namespace := range candidates {
		key := RouteKey{
			RuntimeID: runtimeID,
			Namespace: namespace,
		}
		if route, exists := r.routes[key]; exists {
			return route.Clone(), nil
		}
	}

	return Route{}, NewRPCErrorWithCause(
		RPCErrorMethodNotFound,
		domain.ErrNotFound,
		fmt.Sprintf("no registered namespace matches method %q in runtime %s", method, runtimeID),
		nil,
	)
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
	defer r.mu.RUnlock()

	var result []Route
	prefix := ServiceKey{
		RuntimeID: runtimeID,
		ServiceID: serviceID,
	}
	for rk := range r.byService[prefix] {
		if route, ok := r.routes[rk]; ok {
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

func (r Route) Clone() Route {
	return Route{
		RuntimeID:    r.RuntimeID,
		PluginID:     r.PluginID,
		ServiceID:    r.ServiceID,
		Namespace:    r.Namespace,
		ConnectionID: r.ConnectionID,
	}
}
