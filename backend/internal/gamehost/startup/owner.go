package startup

import (
	"context"
	"time"
)

type ProcessOwnershipVerifier interface {
	VerifyProcess(ctx context.Context, pid int, proof OwnershipProof) OwnershipResult
}

type DirectoryOwnershipVerifier interface {
	VerifyPath(ctx context.Context, canonicalPath string, proof OwnershipProof) OwnershipResult
}

type BinaryOwnershipVerifier interface {
	VerifyBinary(ctx context.Context, binaryID string, proof OwnershipProof) OwnershipResult
}

type EndpointOwnershipVerifier interface {
	VerifyEndpoint(ctx context.Context, endpointID string, proof OwnershipProof) OwnershipResult
}

type SharedMemoryOwnershipVerifier interface {
	VerifySharedMemory(ctx context.Context, shmID string, proof OwnershipProof) OwnershipResult
}

type DefaultProcessOwnershipVerifier struct {
	hostInstanceID string
}

func NewDefaultProcessOwnershipVerifier(hostInstanceID string) *DefaultProcessOwnershipVerifier {
	return &DefaultProcessOwnershipVerifier{hostInstanceID: hostInstanceID}
}

func (v *DefaultProcessOwnershipVerifier) VerifyProcess(ctx context.Context, pid int, proof OwnershipProof) OwnershipResult {
	if v.hostInstanceID == "" || proof.HostInstanceID == "" {
		return OwnershipUnknown
	}
	if proof.HostInstanceID != v.hostInstanceID {
		return OwnershipBelongsToForeign
	}
	if proof.RuntimeID == "" && proof.PluginID == "" {
		return OwnershipUnknown
	}
	return OwnershipVerified
}

type DefaultDirectoryOwnershipVerifier struct {
	managedRoot string
}

func NewDefaultDirectoryOwnershipVerifier(managedRoot string) *DefaultDirectoryOwnershipVerifier {
	return &DefaultDirectoryOwnershipVerifier{managedRoot: managedRoot}
}

func (v *DefaultDirectoryOwnershipVerifier) VerifyPath(ctx context.Context, canonicalPath string, proof OwnershipProof) OwnershipResult {
	if v.managedRoot == "" {
		return OwnershipUnknown
	}
	if !IsSubPath(canonicalPath, v.managedRoot) {
		return OwnershipUnknown
	}
	if proof.HostInstanceID != "" {
		return OwnershipVerified
	}
	if proof.RuntimeID != "" || proof.PluginID != "" {
		return OwnershipVerified
	}
	return OwnershipUnknown
}

type DefaultBinaryOwnershipVerifier struct{}

func NewDefaultBinaryOwnershipVerifier() *DefaultBinaryOwnershipVerifier {
	return &DefaultBinaryOwnershipVerifier{}
}

func (v *DefaultBinaryOwnershipVerifier) VerifyBinary(ctx context.Context, binaryID string, proof OwnershipProof) OwnershipResult {
	if binaryID == "" {
		return OwnershipUnknown
	}
	if proof.PluginID != "" && proof.RuntimeID != "" {
		return OwnershipVerified
	}
	if proof.ServiceID != "" {
		return OwnershipVerified
	}
	return OwnershipUnknown
}

type DefaultEndpointOwnershipVerifier struct{}

func NewDefaultEndpointOwnershipVerifier() *DefaultEndpointOwnershipVerifier {
	return &DefaultEndpointOwnershipVerifier{}
}

func (v *DefaultEndpointOwnershipVerifier) VerifyEndpoint(ctx context.Context, endpointID string, proof OwnershipProof) OwnershipResult {
	if endpointID == "" {
		return OwnershipUnknown
	}
	if proof.RuntimeID != "" || proof.ServiceID != "" {
		return OwnershipVerified
	}
	return OwnershipUnknown
}

type DefaultSharedMemoryOwnershipVerifier struct{}

func NewDefaultSharedMemoryOwnershipVerifier() *DefaultSharedMemoryOwnershipVerifier {
	return &DefaultSharedMemoryOwnershipVerifier{}
}

func (v *DefaultSharedMemoryOwnershipVerifier) VerifySharedMemory(ctx context.Context, shmID string, proof OwnershipProof) OwnershipResult {
	if shmID == "" {
		return OwnershipUnknown
	}
	if proof.HostInstanceID != "" && proof.RuntimeID != "" {
		return OwnershipVerified
	}
	if proof.Generation > 0 && proof.RuntimeID != "" {
		return OwnershipVerified
	}
	return OwnershipUnknown
}

func IsSubPath(target, root string) bool {
	if len(target) < len(root) {
		return false
	}
	return target[:len(root)] == root
}

type OrphanCandidate struct {
	Resource   OrphanResource
	Discovered time.Time
}

func NewOrphanCandidate(resource OrphanResource) *OrphanCandidate {
	return &OrphanCandidate{
		Resource:   resource,
		Discovered: time.Now(),
	}
}
