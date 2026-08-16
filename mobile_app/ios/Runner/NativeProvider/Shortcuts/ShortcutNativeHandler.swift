import Foundation

@available(iOS 16.0, *)
public class ShortcutNativeHandler: NSObject, IOSNativeOperationHandler {
    public let operations: Set<String> = [
        "shortcuts.status",
        "shortcuts.entities.characters",
        "shortcuts.entities.conversations",
        "shortcuts.entities.alarms",
        "shortcuts.entities.reminders",
        "shortcuts.entities.actions",
        "shortcuts.entity.resolve",
        "shortcuts.entity.suggestions",
        "shortcuts.actions.catalog",
        "shortcuts.action.describe",
        "shortcuts.action.execute",
        "shortcuts.action.confirm",
        "shortcuts.runtime.readiness",
        "shortcuts.runtime.ensure",
        "shortcuts.snapshot.get",
        "shortcuts.snapshot.refresh",
        "shortcuts.shortcuts.provider",
        "shortcuts.shortcuts.phrase",
        "shortcuts.settings.get",
        "shortcuts.settings.update"
    ]

    public override init() {
        super.init()
    }

    public func capabilitySnapshot() -> IOSNativeCapability {
        if #available(iOS 16.0, *) {
            return IOSNativeCapability(
                available: true,
                authorized: true,
                hardwareAvailable: true,
                platformSupported: true,
                foregroundRequired: false
            )
        }
        return IOSNativeCapability(
            available: false,
            authorized: false,
            hardwareAvailable: false,
            platformSupported: false,
            foregroundRequired: false
        )
    }

    public func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        switch request.operation {
        case "shortcuts.status":
            return handleStatus(request)
        case "shortcuts.entities.characters":
            return handleEntitiesCharacters(request)
        case "shortcuts.entities.conversations":
            return handleEntitiesConversations(request)
        case "shortcuts.entities.alarms":
            return handleEntitiesAlarms(request)
        case "shortcuts.entities.reminders":
            return handleEntitiesReminders(request)
        case "shortcuts.entities.actions":
            return handleEntitiesActions(request)
        case "shortcuts.entity.resolve":
            return handleEntityResolve(request)
        case "shortcuts.entity.suggestions":
            return handleEntitySuggestions(request)
        case "shortcuts.actions.catalog":
            return handleActionsCatalog(request)
        case "shortcuts.action.describe":
            return handleActionDescribe(request)
        case "shortcuts.action.execute":
            return await handleActionExecute(request)
        case "shortcuts.action.confirm":
            return await handleActionConfirm(request)
        case "shortcuts.runtime.readiness":
            return handleRuntimeReadiness(request)
        case "shortcuts.runtime.ensure":
            return handleRuntimeEnsure(request)
        case "shortcuts.snapshot.get":
            return handleSnapshotGet(request)
        case "shortcuts.snapshot.refresh":
            return handleSnapshotRefresh(request)
        case "shortcuts.shortcuts.provider":
            return handleShortcutsProvider(request)
        case "shortcuts.shortcuts.phrase":
            return handleShortcutsPhrase(request)
        case "shortcuts.settings.get":
            return handleSettingsGet(request)
        case "shortcuts.settings.update":
            return handleSettingsUpdate(request)
        default:
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "OPERATION_NOT_SUPPORTED", message: "unsupported operation: \(request.operation)")
            )
        }
    }

    private func handleStatus(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let available = #available(iOS 16.0, *)
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: [
                "available": available,
                "actions": ShortcutActionGateway.shared.registeredActions.map { $0.actionId }
            ],
            error: nil
        )
    }

    private func handleEntitiesCharacters(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "entities.characters handled by Backend")
        )
    }

    private func handleEntitiesConversations(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "entities.conversations handled by Backend")
        )
    }

    private func handleEntitiesAlarms(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "entities.alarms handled by Backend")
        )
    }

    private func handleEntitiesReminders(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "entities.reminders handled by Backend")
        )
    }

    private func handleEntitiesActions(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "entities.actions handled by Backend")
        )
    }

    private func handleEntityResolve(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "entity.resolve handled by Backend")
        )
    }

    private func handleEntitySuggestions(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "entity.suggestions handled by Backend")
        )
    }

    private func handleActionsCatalog(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "actions.catalog handled by Backend")
        )
    }

    private func handleActionDescribe(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "action.describe handled by Backend")
        )
    }

    private func handleActionExecute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let actionId = request.payload?["actionId"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing actionId")
            )
        }

        guard let action = ShortcutAction.from(actionId: actionId) else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "ACTION_NOT_AVAILABLE", message: "unknown action: \(actionId)")
            )
        }

        guard ShortcutActionGateway.shared.isCuratedAction(action) else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "ACTION_NOT_AVAILABLE", message: "action not in curated catalog: \(actionId)")
            )
        }

        let result = await ShortcutActionGateway.shared.executeAction(action, payload: request.payload)
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: ["actionId": actionId, "backendResult": result],
            error: nil
        )
    }

    private func handleActionConfirm(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let actionId = request.payload?["actionId"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing actionId")
            )
        }

        guard let action = ShortcutAction.from(actionId: actionId) else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "ACTION_NOT_AVAILABLE", message: "unknown action: \(actionId)")
            )
        }

        guard ShortcutActionGateway.shared.isCuratedAction(action) else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "ACTION_NOT_AVAILABLE", message: "action not in curated catalog: \(actionId)")
            )
        }

        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: ["actionId": actionId, "confirmable": true],
            error: nil
        )
    }

    private func handleRuntimeReadiness(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let available = #available(iOS 16.0, *)
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: ["ready": available],
            error: nil
        )
    }

    private func handleRuntimeEnsure(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let available = #available(iOS 16.0, *)
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: ["ensured": available],
            error: nil
        )
    }

    private func handleSnapshotGet(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "snapshot.get handled by Backend")
        )
    }

    private func handleSnapshotRefresh(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "snapshot.refresh handled by Backend")
        )
    }

    private func handleShortcutsProvider(_ request: IOSNativeRequest) -> IOSNativeResponse {
        if #available(iOS 16.0, *) {
            let shortcuts = AmitiaAppShortcutsProvider.appShortcuts
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["shortcuts": shortcuts.map { $0.phrases.first ?? "" }],
                error: nil
            )
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "iOS 16.0+ required")
        )
    }

    private func handleShortcutsPhrase(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: ["phrases": []],
            error: nil
        )
    }

    private func handleSettingsGet(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "settings.get handled by Backend")
        )
    }

    private func handleSettingsUpdate(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "settings.update handled by Backend")
        )
    }
}
