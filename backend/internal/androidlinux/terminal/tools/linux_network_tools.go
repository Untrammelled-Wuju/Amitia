//go:build linux && !android

package tools

import (
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/androidlinux/terminal"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimehost"
)

func (r *terminalToolRegistrar) RegisterNetworkTools(
	host runtimehost.RuntimeHost,
	registry *capability.ToolRegistry,
) error {
	if !terminal.IsAndroidLinuxRuntime(host) {
		return nil
	}

	tools := BuildNetworkTools()

	for _, tool := range tools {
		if err := registry.Register(nil, tool); err != nil {
			if err := registry.Replace(nil, tool); err != nil {
				return fmt.Errorf("register network tool %s: %w", tool.ID, err)
			}
		}
	}

	return nil
}

func BuildNetworkTools() []capability.ToolDefinition {
	providerID := "android_linux"
	ns := "network"

	statusID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, ns+".status")
	interfacesID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, ns+".interfaces")
	routesID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, ns+".routes")
	dnsLookupID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, ns+".dns.lookup")
	pingID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, ns+".ping")
	tcpProbeID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, ns+".tcp.probe")
	httpRequestID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, ns+".http.request")
	multipartRequestID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, ns+".http.multipart")
	downloadID := capability.BuildToolID(capability.ToolSourceBuiltin, providerID, ns+".download")

	inspectPerm := []capability.PermissionRequirement{
		{Capability: "runtime.linux.network.inspect", Risk: "low"},
	}
	publicPerm := []capability.PermissionRequirement{
		{Capability: "runtime.linux.network.public", Risk: "medium"},
	}
	downloadPerm := []capability.PermissionRequirement{
		{Capability: "runtime.linux.network.download", Risk: "high"},
	}

	runtime := capability.RuntimeBinding{
		RuntimeType: capability.RuntimeTypeAndroidLinux,
		RuntimeID:   terminal.RuntimeIDAndroidLinux,
		HandlerName: "network.status",
	}

	return []capability.ToolDefinition{
		{
			ID:             string(statusID),
			ModelName:      "android_linux__network__status",
			Source:         capability.ToolSourceBuiltin,
			Name:           "Network Status",
			Description:    "Get network availability status",
			InputSchema:    json.RawMessage(`{"type": "object", "properties": {}}`),
			OutputSchema:   json.RawMessage(`{"type": "object", "properties": {"available": {"type": "boolean"}, "dnsAvailable": {"type": "boolean"}, "outboundAvailable": {"type": "boolean"}, "loopbackAvailable": {"type": "boolean"}, "httpAvailable": {"type": "boolean"}, "tcpAvailable": {"type": "boolean"}, "interfaceCount": {"type": "integer"}}}`),
			Permissions:    inspectPerm,
			RiskLevel:      capability.RiskLow,
			SideEffect:     capability.SideEffectReadOnly,
			HasSideEffects: false,
			Idempotent:     true,
			Retryable:      true,
			TimeoutMS:      5000,
			Runtime:        capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "network.status"},
			ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b20.2"},
			Enabled:        true,
		},
		{
			ID:             string(interfacesID),
			ModelName:      "android_linux__network__interfaces",
			Source:         capability.ToolSourceBuiltin,
			Name:           "Network Interfaces",
			Description:    "List network interfaces and their addresses",
			InputSchema:    json.RawMessage(`{"type": "object", "properties": {}}`),
			OutputSchema:   json.RawMessage(`{"type": "object", "properties": {"interfaces": {"type": "array"}, "count": {"type": "integer"}}}`),
			Permissions:    inspectPerm,
			RiskLevel:      capability.RiskLow,
			SideEffect:     capability.SideEffectReadOnly,
			HasSideEffects: false,
			Idempotent:     true,
			Retryable:      true,
			TimeoutMS:      5000,
			Runtime:        capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "network.interfaces"},
			ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b20.2"},
			Enabled:        true,
		},
		{
			ID:             string(routesID),
			ModelName:      "android_linux__network__routes",
			Source:         capability.ToolSourceBuiltin,
			Name:           "Network Routes",
			Description:    "Read the system routing table",
			InputSchema:    json.RawMessage(`{"type": "object", "properties": {}}`),
			OutputSchema:   json.RawMessage(`{"type": "object", "properties": {"routes": {"type": "array"}, "count": {"type": "integer"}}}`),
			Permissions:    inspectPerm,
			RiskLevel:      capability.RiskLow,
			SideEffect:     capability.SideEffectReadOnly,
			HasSideEffects: false,
			Idempotent:     true,
			Retryable:      true,
			TimeoutMS:      5000,
			Runtime:        capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "network.routes"},
			ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b20.2"},
			Enabled:        true,
		},
		{
			ID:             string(dnsLookupID),
			ModelName:      "android_linux__network__dns_lookup",
			Source:         capability.ToolSourceBuiltin,
			Name:           "DNS Lookup",
			Description:    "Resolve a hostname to IP addresses",
			InputSchema:    json.RawMessage(`{"type": "object", "required": ["host"], "properties": {"host": {"type": "string"}, "type": {"type": "string", "enum": ["A", "AAAA", "IP", "CNAME", "TXT", "MX"]}}}`),
			OutputSchema:   json.RawMessage(`{"type": "object", "properties": {"host": {"type": "string"}, "type": {"type": "string"}, "addresses": {"type": "array", "items": {"type": "string"}}, "cname": {"type": "string"}, "txt": {"type": "array", "items": {"type": "string"}}, "mx": {"type": "array"}}}`),
			Permissions:    inspectPerm,
			RiskLevel:      capability.RiskLow,
			SideEffect:     capability.SideEffectReadOnly,
			HasSideEffects: false,
			Idempotent:     true,
			Retryable:      true,
			TimeoutMS:      10000,
			Runtime:        capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "network.dns.lookup"},
			ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b20.2"},
			Enabled:        true,
		},
		{
			ID:             string(pingID),
			ModelName:      "android_linux__network__ping",
			Source:         capability.ToolSourceBuiltin,
			Name:           "Ping Host",
			Description:    "Send ICMP echo requests to a host",
			InputSchema:    json.RawMessage(`{"type": "object", "required": ["host"], "properties": {"host": {"type": "string"}, "count": {"type": "integer", "minimum": 1, "maximum": 5}, "timeoutMs": {"type": "integer", "minimum": 100, "maximum": 60000}}}`),
			OutputSchema:   json.RawMessage(`{"type": "object", "properties": {"host": {"type": "string"}, "resolvedIp": {"type": "string"}, "sent": {"type": "integer"}, "received": {"type": "integer"}, "lossPercent": {"type": "number"}, "minMs": {"type": "number"}, "avgMs": {"type": "number"}, "maxMs": {"type": "number"}, "mode": {"type": "string"}}}`),
			Permissions:    publicPerm,
			RiskLevel:      capability.RiskMedium,
			SideEffect:     capability.SideEffectExternal,
			HasSideEffects: true,
			Idempotent:     false,
			Retryable:      true,
			TimeoutMS:      30000,
			Runtime:        capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "network.ping"},
			ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b20.2"},
			Enabled:        true,
		},
		{
			ID:             string(tcpProbeID),
			ModelName:      "android_linux__network__tcp_probe",
			Source:         capability.ToolSourceBuiltin,
			Name:           "TCP Probe",
			Description:    "Probe a single TCP host:port for reachability",
			InputSchema:    json.RawMessage(`{"type": "object", "required": ["host", "port"], "properties": {"host": {"type": "string"}, "port": {"type": "integer", "minimum": 1, "maximum": 65535}, "timeoutMs": {"type": "integer", "minimum": 100, "maximum": 60000}}}`),
			OutputSchema:   json.RawMessage(`{"type": "object", "properties": {"host": {"type": "string"}, "resolvedIp": {"type": "string"}, "port": {"type": "integer"}, "reachable": {"type": "boolean"}, "connectTimeMs": {"type": "integer"}}}`),
			Permissions:    publicPerm,
			RiskLevel:      capability.RiskMedium,
			SideEffect:     capability.SideEffectExternal,
			HasSideEffects: true,
			Idempotent:     true,
			Retryable:      true,
			TimeoutMS:      15000,
			Runtime:        capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "network.tcp.probe"},
			ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b20.2"},
			Enabled:        true,
		},
		{
			ID:             string(httpRequestID),
			ModelName:      "android_linux__network__http_request",
			Source:         capability.ToolSourceBuiltin,
			Name:           "HTTP Request",
			Description:    "Make an HTTP/HTTPS request with security controls",
			InputSchema:    json.RawMessage(`{"type": "object", "required": ["method", "url"], "properties": {"method": {"type": "string", "enum": ["GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"]}, "url": {"type": "string"}, "headers": {"type": "object", "additionalProperties": {"type": "string"}}, "body": {"type": "string"}, "bodyBase64": {"type": "string"}, "timeoutMs": {"type": "integer", "minimum": 100, "maximum": 60000}, "followRedirects": {"type": "boolean"}, "maxResponseBytes": {"type": "integer", "minimum": 1, "maximum": 16777216}}}`),
			OutputSchema:   json.RawMessage(`{"type": "object", "properties": {"statusCode": {"type": "integer"}, "status": {"type": "string"}, "headers": {"type": "object"}, "body": {"type": "string"}, "bodyBase64": {"type": "string"}, "mimeType": {"type": "string"}, "charset": {"type": "string"}, "bytesRead": {"type": "integer"}, "truncated": {"type": "boolean"}, "finalUrl": {"type": "string"}, "durationMs": {"type": "integer"}}}`),
			Permissions:    publicPerm,
			RiskLevel:      capability.RiskMedium,
			SideEffect:     capability.SideEffectExternal,
			HasSideEffects: true,
			Idempotent:     false,
			Retryable:      false,
			TimeoutMS:      30000,
			Runtime:        capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "network.http.request"},
			ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b20.2"},
			Enabled:        true,
		},
		{
			ID:             string(multipartRequestID),
			ModelName:      "multipart_request",
			Source:         capability.ToolSourceBuiltin,
			Name:           "Multipart HTTP Request",
			Description:    "Send a bounded multipart/form-data HTTP request with text or base64 file parts using the Android Linux network policy.",
			InputSchema:    json.RawMessage(`{"type":"object","required":["url","parts"],"properties":{"method":{"type":"string","enum":["POST","PUT","PATCH"]},"url":{"type":"string","minLength":1,"maxLength":8192},"headers":{"type":"object","maxProperties":64,"additionalProperties":{"type":"string"}},"parts":{"type":"array","minItems":1,"maxItems":64,"items":{"type":"object","required":["name"],"properties":{"name":{"type":"string","minLength":1,"maxLength":256},"value":{"type":"string"},"filename":{"type":"string","maxLength":512},"contentType":{"type":"string","maxLength":256},"dataBase64":{"type":"string"}},"additionalProperties":false}},"timeoutMs":{"type":"integer","minimum":100,"maximum":120000},"followRedirects":{"type":"boolean"},"maxResponseBytes":{"type":"integer","minimum":1,"maximum":16777216}},"additionalProperties":false}`),
			OutputSchema:   json.RawMessage(`{"type":"object","properties":{"statusCode":{"type":"integer"},"status":{"type":"string"},"headers":{"type":"object"},"body":{"type":"string"},"bodyBase64":{"type":"string"},"bytesRead":{"type":"integer"},"truncated":{"type":"boolean"},"finalUrl":{"type":"string"},"durationMs":{"type":"integer"}}}`),
			Permissions:    publicPerm,
			RiskLevel:      capability.RiskMedium,
			SideEffect:     capability.SideEffectExternal,
			HasSideEffects: true,
			Idempotent:     false,
			Retryable:      false,
			TimeoutMS:      120000,
			Runtime:        capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "network.http.multipart"},
			ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "amitia-multipart-v1"},
			ModelExposure:  capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"network", "http"}, Priority: 45},
			Enabled:        true,
		},
		{
			ID:             string(downloadID),
			ModelName:      "android_linux__network__download",
			Source:         capability.ToolSourceBuiltin,
			Name:           "Download File",
			Description:    "Download a file via HTTP/HTTPS to the workspace",
			InputSchema:    json.RawMessage(`{"type": "object", "required": ["url", "target"], "properties": {"url": {"type": "string"}, "target": {"type": "string"}, "overwrite": {"type": "boolean"}, "timeoutMs": {"type": "integer", "minimum": 100, "maximum": 60000}, "maxBytes": {"type": "integer", "minimum": 1, "maximum": 268435456}}}`),
			OutputSchema:   json.RawMessage(`{"type": "object", "properties": {"path": {"type": "string"}, "bytesWritten": {"type": "integer"}, "mimeType": {"type": "string"}, "sha256": {"type": "string"}, "statusCode": {"type": "integer"}, "finalUrl": {"type": "string"}}}`),
			Permissions:    downloadPerm,
			RiskLevel:      capability.RiskHigh,
			SideEffect:     capability.SideEffectWrite,
			HasSideEffects: true,
			Idempotent:     false,
			Retryable:      false,
			TimeoutMS:      120000,
			Runtime:        capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: "network.download"},
			ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b20.2"},
			Enabled:        true,
		},
	}
}
