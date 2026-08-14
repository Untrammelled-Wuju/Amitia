package authority

import "fmt"

type Domain string

const (
	DomainPlugin     Domain = "plugin"
	DomainDevice     Domain = "device"
	DomainCapability Domain = "capability"
	DomainTask       Domain = "task"
	DomainEvent      Domain = "event"
	DomainPermission Domain = "permission"
)

type Disposition string

const (
	DispositionRetain     Disposition = "retain"
	DispositionPromote    Disposition = "promote"
	DispositionAdapter    Disposition = "adapter"
	DispositionProjection Disposition = "projection"
	DispositionDeprecate  Disposition = "deprecate"
)

type CanonicalBinding struct {
	Domain       Domain
	OwnerPackage string
	PrimaryTypes []string
}

type ExistingComponentBinding struct {
	ComponentPackage string
	Disposition      Disposition
	CanonicalDomain  Domain
	Role             string
}

var canonicalBindings = []CanonicalBinding{
	{
		Domain:       DomainPlugin,
		OwnerPackage: "backend/internal/extension/kernel/domain",
		PrimaryTypes: []string{
			"ExtensionDefinition",
			"ExtensionInstallation",
			"DefinitionRepository",
			"InstallationRepository",
		},
	},
	{
		Domain:       DomainDevice,
		OwnerPackage: "backend/internal/extension/kernel/host_registry",
		PrimaryTypes: []string{
			"HostRegistry",
			"HostEntry",
			"ConnectionState",
		},
	},
	{
		Domain:       DomainCapability,
		OwnerPackage: "backend/internal/extension/kernel/capability",
		PrimaryTypes: []string{
			"CapabilityDefinition",
			"CapabilityID",
			"ToolDefinition",
			"ToolRegistry",
			"RuntimeBinding",
			"RuntimeAdapterRegistry",
		},
	},
	{
		Domain:       DomainTask,
		OwnerPackage: "backend/internal/extension/kernel/task_runtime",
		PrimaryTypes: []string{
			"TaskRun",
			"TaskRunStatus",
			"TaskRuntimeService",
			"TaskExecutor",
		},
	},
	{
		Domain:       DomainEvent,
		OwnerPackage: "backend/internal/extension/kernel/event",
		PrimaryTypes: []string{
			"EventEnvelope",
			"EventPublisher",
			"OutboxRepository",
			"Dispatcher",
			"SQLiteDeliveryStore",
			"Service",
		},
	},
	{
		Domain:       DomainPermission,
		OwnerPackage: "backend/internal/extension/kernel/permission",
		PrimaryTypes: []string{
			"PermissionBroker",
			"DefaultPermissionBroker",
			"PermissionDefinitionRegistry",
			"PermissionEvaluationRequest",
			"ApprovalRequest",
		},
	},
}

var existingComponentBindings = []ExistingComponentBinding{
	{
		ComponentPackage: "backend/internal/runtimeorchestrator",
		Disposition:      DispositionRetain,
		CanonicalDomain:  DomainCapability,
		Role:             "Runtime Provider Factory / Slot / Instance 构造与 Runtime 生命周期装配；不是 AI Capability Provider 权威。",
	},
	{
		ComponentPackage: "backend/internal/desktoppet/runtime/protocol/v2",
		Disposition:      DispositionAdapter,
		CanonicalDomain:  DomainDevice,
		Role:             "现有桌宠 Runtime v2 中的 hello/session/heartbeat/command/ack/sequence/resume/reconcile/dedup 是后续通用 Device Runtime Protocol 的抽取来源，但桌宠协议本身不能继续作为全局 Device 权威。",
	},
	{
		ComponentPackage: "backend/internal/system",
		Disposition:      DispositionProjection,
		CanonicalDomain:  DomainEvent,
		Role:             "仅作为进程内实时 UI/SSE 消息投影和即时通知通道，不是 durable event、sync log 或跨设备最终一致性的权威来源。",
	},
	{
		ComponentPackage: "backend/internal/extension/kernel",
		Disposition:      DispositionAdapter,
		CanonicalDomain:  DomainDevice,
		Role:             "sse_ui_host.go / ui_contribution 仅作为 UI Host Adapter，不得继续扩张成独立的 Cloud Device Registry。",
	},
	{
		ComponentPackage: "backend/internal/gamehost",
		Disposition:      DispositionAdapter,
		CanonicalDomain:  DomainDevice,
		Role:             "仅作为领域消费者 / Adapter，不得声明自己为 Device、Task、Event 或 Permission 的全局 Source of Truth。",
	},
	{
		ComponentPackage: "backend/internal/desktoppet",
		Disposition:      DispositionAdapter,
		CanonicalDomain:  DomainDevice,
		Role:             "仅作为领域消费者 / Adapter，不得声明自己为 Device、Task、Event 或 Permission 的全局 Source of Truth。",
	},
}

func Canonical(domain Domain) (CanonicalBinding, bool) {
	for _, b := range canonicalBindings {
		if b.Domain == domain {
			return b, true
		}
	}
	return CanonicalBinding{}, false
}

func CanonicalBindings() []CanonicalBinding {
	result := make([]CanonicalBinding, len(canonicalBindings))
	copy(result, canonicalBindings)
	return result
}

func ExistingComponentBindings() []ExistingComponentBinding {
	result := make([]ExistingComponentBinding, len(existingComponentBindings))
	copy(result, existingComponentBindings)
	return result
}

func MustValidate() {
	seen := make(map[Domain]bool)
	for _, b := range canonicalBindings {
		if b.Domain == "" {
			panic("authority: canonical binding domain is empty")
		}
		if seen[b.Domain] {
			panic(fmt.Sprintf("authority: duplicate canonical domain: %s", b.Domain))
		}
		seen[b.Domain] = true
		if b.OwnerPackage == "" {
			panic(fmt.Sprintf("authority: canonical binding for %s has empty owner package", b.Domain))
		}
		if len(b.PrimaryTypes) == 0 {
			panic(fmt.Sprintf("authority: canonical binding for %s has empty primary types", b.Domain))
		}
	}

	requiredDomains := []Domain{DomainPlugin, DomainDevice, DomainCapability, DomainTask, DomainEvent, DomainPermission}
	for _, d := range requiredDomains {
		if !seen[d] {
			panic(fmt.Sprintf("authority: missing required canonical domain: %s", d))
		}
	}

	seenComponents := make(map[string]Disposition)
	for _, b := range existingComponentBindings {
		if b.ComponentPackage == "" {
			panic("authority: existing component binding has empty package")
		}
		if b.CanonicalDomain == "" {
			panic(fmt.Sprintf("authority: existing component %s has empty canonical domain", b.ComponentPackage))
		}
		if _, ok := seen[b.CanonicalDomain]; !ok {
			panic(fmt.Sprintf("authority: existing component %s references unknown canonical domain: %s", b.ComponentPackage, b.CanonicalDomain))
		}
		if !isValidDisposition(b.Disposition) {
			panic(fmt.Sprintf("authority: existing component %s has invalid disposition: %s", b.ComponentPackage, b.Disposition))
		}
		if prev, exists := seenComponents[b.ComponentPackage]; exists && prev != b.Disposition {
			panic(fmt.Sprintf("authority: component %s has conflicting dispositions: %s vs %s", b.ComponentPackage, prev, b.Disposition))
		}
		seenComponents[b.ComponentPackage] = b.Disposition
	}
}

func isValidDisposition(d Disposition) bool {
	switch d {
	case DispositionRetain, DispositionPromote, DispositionAdapter, DispositionProjection, DispositionDeprecate:
		return true
	}
	return false
}
