package resource

import "context"

type ResourceClass string

const (
	ResourceCPU           ResourceClass = "cpu"
	ResourceMemory        ResourceClass = "memory"
	ResourceDisk          ResourceClass = "disk"
	ResourceOpenFiles     ResourceClass = "open_files"
	ResourceSubprocess    ResourceClass = "subprocess"
	ResourcePendingRPC    ResourceClass = "pending_rpc"
	ResourceQueue         ResourceClass = "queue"
	ResourceBufferedBytes ResourceClass = "buffered_bytes"
	ResourceBinaryCount   ResourceClass = "binary_count"
	ResourceBinaryBytes   ResourceClass = "binary_bytes"
	ResourceBandwidth     ResourceClass = "bandwidth"
	ResourceReplayWindow  ResourceClass = "replay_window"
)

type DenyReason string

const (
	DenyUnknown          DenyReason = "unknown"
	DenyCPULimit         DenyReason = "cpu_limit"
	DenyMemoryLimit      DenyReason = "memory_limit"
	DenyDiskLimit        DenyReason = "disk_limit"
	DenyOpenFilesLimit   DenyReason = "open_files_limit"
	DenySubprocessLimit  DenyReason = "subprocess_limit"
	DenyPendingLimit     DenyReason = "pending_limit"
	DenyQueueLimit       DenyReason = "queue_limit"
	DenyBufferedBytes    DenyReason = "buffered_bytes"
	DenyBinaryCountLimit DenyReason = "binary_count_limit"
	DenyBinaryBytesLimit DenyReason = "binary_bytes_limit"
	DenyBandwidthLimit   DenyReason = "bandwidth_limit"
	DenyReplayLimit      DenyReason = "replay_limit"
)

type AdmissionDecision struct {
	Allowed bool
	Reason  DenyReason
	Class   ResourceClass
	Current int64
	Limit   int64
}

type RuntimeIdentitySubject struct {
	ExtensionID string
	PluginID    string
	RuntimeID   string
	ServiceID   string
	Generation  int64
}

type StartupRevertFunc func()
type ReleaseFunc func()
type BinaryRevertFunc func()

type RuntimeResourceProfile struct {
	MaxMemoryMB        int64
	MaxCPUPercent      int
	MaxFileDescriptors int
	MaxDiskMB          int64
	MaxSubprocesses    int
}

type AdmissionAdapter interface {
	AcquireRuntimeStartup(ctx context.Context, subj RuntimeIdentitySubject, profile *RuntimeResourceProfile) (StartupRevertFunc, error)
	AcquireRPCPending(ctx context.Context, subj RuntimeIdentitySubject) (AdmissionDecision, ReleaseFunc)
	AcquireBinaryObject(ctx context.Context, subj RuntimeIdentitySubject, requestedBytes int64) (AdmissionDecision, BinaryRevertFunc)
	AcquireQueuePublish(ctx context.Context, subj RuntimeIdentitySubject) (AdmissionDecision, ReleaseFunc)
	Shutdown()
	Reset()
}
