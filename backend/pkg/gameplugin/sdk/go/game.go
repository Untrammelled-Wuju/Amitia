package sdk

import "github.com/u-ai/backend/pkg/gameplugin/protocol"

type HostFeature = protocol.HostFeature

const (
	HostFeatureRealtimeControl = protocol.HostFeatureRealtimeControl
	HostFeatureStateStreaming  = protocol.HostFeatureStateStreaming
	HostFeatureEventStreaming  = protocol.HostFeatureEventStreaming
	HostFeatureCustomRPC       = protocol.HostFeatureCustomRPC
	HostFeatureHostAPI         = protocol.HostFeatureHostAPI
	HostFeatureSharedControl   = protocol.HostFeatureSharedControl
	HostFeatureMultiService    = protocol.HostFeatureMultiService
)

type PluginSessionOpenRequest = protocol.PluginSessionOpenRequest
type PluginSession = protocol.PluginSession
type PluginSessionStatus = protocol.PluginSessionStatus
type PluginEvent = protocol.PluginEvent
type PluginOperation = protocol.PluginOperation
type PluginOperationStatus = protocol.PluginOperationStatus
type PluginOperationResult = protocol.PluginOperationResult
type PluginArtifact = protocol.PluginArtifact
type PluginNetworkPolicy = protocol.PluginNetworkPolicy
type PluginServiceSpec = protocol.PluginServiceSpec
type PluginChannelSpec = protocol.PluginChannelSpec
type PluginControlEffectSinkSpec = protocol.PluginControlEffectSinkSpec
type PluginHostSpec = protocol.PluginHostSpec
