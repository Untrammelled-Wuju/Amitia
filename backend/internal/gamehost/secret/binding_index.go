package secret

import (
	"fmt"
	"sync"

	kernelsecret "github.com/u-ai/backend/internal/extension/kernel/secret"
)

type bindingKey struct {
	runtimeID  string
	serviceID  string
	generation int64
	ref        string
}

type bindingEntry struct {
	KernelLease  kernelsecret.LeaseID
	RuntimeID    string
	ServiceID    string
	Ref          string
	ExtensionID  string
	Generation   int64
	Purpose      Purpose
	Revoked      bool
	RevokedAt    int64
}

type LeaseBindingIndex struct {
	mu           sync.RWMutex
	byKey        map[bindingKey]*bindingEntry
	byLease      map[kernelsecret.LeaseID]*bindingEntry
	byRuntime    map[string]map[bindingKey]struct{}
	byService    map[string]map[kernelsecret.LeaseID]struct{}
	byGeneration map[string]map[kernelsecret.LeaseID]struct{}
	clock        func() int64
}

func NewLeaseBindingIndex(clock func() int64) *LeaseBindingIndex {
	if clock == nil {
		clock = defaultClock
	}
	return &LeaseBindingIndex{
		byKey:        make(map[bindingKey]*bindingEntry),
		byLease:      make(map[kernelsecret.LeaseID]*bindingEntry),
		byRuntime:    make(map[string]map[bindingKey]struct{}),
		byService:    make(map[string]map[kernelsecret.LeaseID]struct{}),
		byGeneration: make(map[string]map[kernelsecret.LeaseID]struct{}),
		clock:        clock,
	}
}

func (i *LeaseBindingIndex) Record(kernelLease kernelsecret.LeaseID, runtimeID, serviceID, ref, extensionID string, generation int64, purpose Purpose) *bindingEntry {
	i.mu.Lock()
	defer i.mu.Unlock()

	key := bindingKey{runtimeID: runtimeID, serviceID: serviceID, generation: generation, ref: ref}
	entry := &bindingEntry{
		KernelLease: kernelLease,
		RuntimeID:   runtimeID,
		ServiceID:   serviceID,
		Ref:         ref,
		ExtensionID: extensionID,
		Generation:  generation,
		Purpose:     purpose,
	}
	i.byKey[key] = entry
	i.byLease[kernelLease] = entry

	if i.byRuntime[runtimeID] == nil {
		i.byRuntime[runtimeID] = make(map[bindingKey]struct{})
	}
	i.byRuntime[runtimeID][key] = struct{}{}

	svcKey := runtimeID + "/" + serviceID
	if i.byService[svcKey] == nil {
		i.byService[svcKey] = make(map[kernelsecret.LeaseID]struct{})
	}
	i.byService[svcKey][kernelLease] = struct{}{}

	genKey := fmt.Sprintf("%s/%d", runtimeID, generation)
	if i.byGeneration[genKey] == nil {
		i.byGeneration[genKey] = make(map[kernelsecret.LeaseID]struct{})
	}
	i.byGeneration[genKey][kernelLease] = struct{}{}

	return entry
}

func (i *LeaseBindingIndex) LookupByService(runtimeID, serviceID, ref string, generation int64) (*bindingEntry, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	key := bindingKey{runtimeID: runtimeID, serviceID: serviceID, generation: generation, ref: ref}
	entry, ok := i.byKey[key]
	if !ok {
		return nil, false
	}
	return entry, true
}

func (i *LeaseBindingIndex) LookupByLease(leaseID kernelsecret.LeaseID) (*bindingEntry, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	entry, ok := i.byLease[leaseID]
	return entry, ok
}

func (i *LeaseBindingIndex) ActiveLeasesByService(runtimeID, serviceID string) []kernelsecret.LeaseID {
	i.mu.RLock()
	defer i.mu.RUnlock()

	svcKey := runtimeID + "/" + serviceID
	leases := make([]kernelsecret.LeaseID, 0, len(i.byService[svcKey]))
	for l := range i.byService[svcKey] {
		if entry, ok := i.byLease[l]; ok && !entry.Revoked {
			leases = append(leases, l)
		}
	}
	return leases
}

func (i *LeaseBindingIndex) ActiveLeasesByRuntime(runtimeID string) []kernelsecret.LeaseID {
	i.mu.RLock()
	defer i.mu.RUnlock()

	seen := make(map[kernelsecret.LeaseID]struct{})
	leases := make([]kernelsecret.LeaseID, 0)
	for key := range i.byRuntime[runtimeID] {
		if entry, ok := i.byKey[key]; ok && !entry.Revoked {
			if _, dup := seen[entry.KernelLease]; !dup {
				seen[entry.KernelLease] = struct{}{}
				leases = append(leases, entry.KernelLease)
			}
		}
	}
	return leases
}

func (i *LeaseBindingIndex) ActiveLeasesByExtension(extensionID string) []kernelsecret.LeaseID {
	i.mu.RLock()
	defer i.mu.RUnlock()

	seen := make(map[kernelsecret.LeaseID]struct{})
	leases := make([]kernelsecret.LeaseID, 0)
	for _, entry := range i.byLease {
		if entry.ExtensionID == extensionID && !entry.Revoked {
			if _, dup := seen[entry.KernelLease]; !dup {
				seen[entry.KernelLease] = struct{}{}
				leases = append(leases, entry.KernelLease)
			}
		}
	}
	return leases
}

func (i *LeaseBindingIndex) ActiveLeasesByGeneration(runtimeID string, generation int64) []kernelsecret.LeaseID {
	i.mu.RLock()
	defer i.mu.RUnlock()

	genKey := fmt.Sprintf("%s/%d", runtimeID, generation)
	leases := make([]kernelsecret.LeaseID, 0, len(i.byGeneration[genKey]))
	for l := range i.byGeneration[genKey] {
		if entry, ok := i.byLease[l]; ok && !entry.Revoked {
			leases = append(leases, l)
		}
	}
	return leases
}

func (i *LeaseBindingIndex) MarkRevoked(leaseID kernelsecret.LeaseID) bool {
	i.mu.Lock()
	defer i.mu.Unlock()

	entry, ok := i.byLease[leaseID]
	if !ok || entry.Revoked {
		return false
	}
	entry.Revoked = true
	entry.RevokedAt = i.clock()
	return true
}

func (i *LeaseBindingIndex) Clear() {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.byKey = make(map[bindingKey]*bindingEntry)
	i.byLease = make(map[kernelsecret.LeaseID]*bindingEntry)
	i.byRuntime = make(map[string]map[bindingKey]struct{})
	i.byService = make(map[string]map[kernelsecret.LeaseID]struct{})
	i.byGeneration = make(map[string]map[kernelsecret.LeaseID]struct{})
}

func defaultClock() int64 {
	return 0
}

type TopologyServiceView interface {
	HasService(serviceID string) bool
	ExecutableServiceCount() int
	DefaultExecutableService() (string, bool)
}

type SecretRefBindingError struct {
	Ref     string
	ServiceID string
	Reason  string
}

func (e *SecretRefBindingError) Error() string {
	return fmt.Sprintf("secret binding invalid: ref=%s serviceId=%s reason=%s", e.Ref, e.ServiceID, e.Reason)
}

func ValidateAndBindSecretRef(ref ServiceSecretManifest, view TopologyServiceView) (*ServiceSecretManifest, error) {
	if !ref.Ref.Valid() {
		return &ref, &SecretRefBindingError{Ref: string(ref.Ref), ServiceID: ref.ServiceID, Reason: "invalid secret ref"}
	}

	if ref.ServiceID != "" {
		if !view.HasService(ref.ServiceID) {
			return &ref, &SecretRefBindingError{Ref: string(ref.Ref), ServiceID: ref.ServiceID, Reason: "serviceId not found in runtime topology"}
		}
		return &ref, nil
	}

	count := view.ExecutableServiceCount()
	if count == 0 {
		return &ref, &SecretRefBindingError{Ref: string(ref.Ref), ServiceID: "", Reason: "no executable service available for auto-bind"}
	}
	if count > 1 {
		return &ref, &SecretRefBindingError{Ref: string(ref.Ref), ServiceID: "", Reason: "multiple executable services; serviceId must be specified"}
	}

	svcID, ok := view.DefaultExecutableService()
	if !ok || svcID == "" {
		return &ref, &SecretRefBindingError{Ref: string(ref.Ref), ServiceID: "", Reason: "unable to resolve default executable service"}
	}
	ref.ServiceID = svcID
	return &ref, nil
}

func ValidateAndBindSecretRefs(refs []ServiceSecretManifest, view TopologyServiceView) ([]ServiceSecretManifest, []error) {
	var result []ServiceSecretManifest
	var errs []error
	for _, ref := range refs {
		binded, err := ValidateAndBindSecretRef(ref, view)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		result = append(result, *binded)
	}
	return result, errs
}
