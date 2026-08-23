package sdk

import "github.com/u-ai/backend/pkg/gameplugin/protocol"

const GameProtocolVersion = protocol.GameProtocolVersion
const EventIDGameEvent = protocol.EventIDGameEvent

const (
	MethodGameSessionOpen     = protocol.MethodGameSessionOpen
	MethodGameSessionClose    = protocol.MethodGameSessionClose
	MethodGameSessionSnapshot = protocol.MethodGameSessionSnapshot
	MethodGameObservationGet  = protocol.MethodGameObservationGet
	MethodGameActionExecute   = protocol.MethodGameActionExecute
	MethodGameGoalSet         = protocol.MethodGameGoalSet
	MethodGameCapabilitiesGet = protocol.MethodGameCapabilitiesGet
)

type GameSessionOpenRequest = protocol.GameSessionOpenRequest
type GameSession = protocol.GameSession
type GameSessionStatus = protocol.GameSessionStatus
type GameLocation = protocol.GameLocation
type GameEntity = protocol.GameEntity
type GameInventoryItem = protocol.GameInventoryItem
type GameInventory = protocol.GameInventory
type GameObservation = protocol.GameObservation
type GameEvent = protocol.GameEvent
type GameAction = protocol.GameAction
type GameActionStatus = protocol.GameActionStatus
type GameActionResult = protocol.GameActionResult
type GameGoal = protocol.GameGoal
type GameCapability = protocol.GameCapability
type GameCompanionArtifact = protocol.GameCompanionArtifact
type GamePluginSpec = protocol.GamePluginSpec
type GameNetworkPolicy = protocol.GameNetworkPolicy
