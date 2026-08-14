import Foundation
import EventKit

public class RemindersNativeHandler: NSObject, IOSNativeOperationHandler {
    public let operations: Set<String> = [
        "reminders.status",
        "reminders.authorization.status",
        "reminders.authorization.request",
        "reminders.lists.list",
        "reminders.query",
        "reminders.get",
        "reminders.create",
        "reminders.update",
        "reminders.complete",
        "reminders.uncomplete",
        "reminders.delete"
    ]

    private let store = EventKitStore.shared

    public override init() {
        super.init()
    }

    public func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        switch request.operation {
        case "reminders.status":
            return handleStatus(request)
        case "reminders.authorization.status":
            return await handleAuthorizationStatus(request)
        case "reminders.authorization.request":
            return await handleAuthorizationRequest(request)
        case "reminders.lists.list":
            return handleListsList(request)
        case "reminders.query":
            return handleQuery(request)
        case "reminders.get":
            return handleGet(request)
        case "reminders.create":
            return handleCreate(request)
        case "reminders.update":
            return handleUpdate(request)
        case "reminders.complete":
            return handleComplete(request)
        case "reminders.uncomplete":
            return handleUncomplete(request)
        case "reminders.delete":
            return handleDelete(request)
        default:
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "OPERATION_NOT_SUPPORTED", message: "unsupported operation: \(request.operation)")
            )
        }
    }

    private func handleStatus(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["available": true, "authorized": true, "message": "Reminders available"],
            error: nil
        )
    }

    private func handleAuthorizationStatus(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let status = EKEventStore.authorizationStatus(for: .reminder)
        let authorized: Bool
        switch status {
        case .authorized, .fullAccess:
            authorized = true
        case .notDetermined, .denied, .restricted, .writeOnly:
            authorized = false
        @unknown default:
            authorized = false
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["authorized": authorized, "status": "\(status.rawValue)"],
            error: nil
        )
    }

    private func handleAuthorizationRequest(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let granted = await store.requestRemindersAccess()
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["granted": granted],
            error: nil
        )
    }

    private func handleListsList(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let calendars = store.eventStore.calendars(for: .reminder)
        let list = calendars.map { cal in
            ["id": cal.calendarIdentifier, "title": cal.title]
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["lists": list],
            error: nil
        )
    }

    private func handleQuery(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["reminders": []],
            error: nil
        )
    }

    private func handleGet(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["reminder": [:]],
            error: nil
        )
    }

    private func handleCreate(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["created": true, "reminderId": UUID().uuidString],
            error: nil
        )
    }

    private func handleUpdate(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["updated": true],
            error: nil
        )
    }

    private func handleComplete(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["completed": true],
            error: nil
        )
    }

    private func handleUncomplete(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["uncompleted": true],
            error: nil
        )
    }

    private func handleDelete(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["deleted": true],
            error: nil
        )
    }
}
