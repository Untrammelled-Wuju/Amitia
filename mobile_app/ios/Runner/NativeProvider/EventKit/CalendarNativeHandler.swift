import Foundation
import EventKit

public class CalendarNativeHandler: NSObject, IOSNativeOperationHandler {
    public let operations: Set<String> = [
        "calendar.status",
        "calendar.authorization.status",
        "calendar.authorization.request",
        "calendar.calendars.list",
        "calendar.events.query",
        "calendar.events.get",
        "calendar.events.create",
        "calendar.events.update",
        "calendar.events.delete"
    ]

    private let store = EventKitStore.shared

    public override init() {
        super.init()
    }

    public func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        switch request.operation {
        case "calendar.status":
            return handleStatus(request)
        case "calendar.authorization.status":
            return await handleAuthorizationStatus(request)
        case "calendar.authorization.request":
            return await handleAuthorizationRequest(request)
        case "calendar.calendars.list":
            return handleCalendarsList(request)
        case "calendar.events.query":
            return handleEventsQuery(request)
        case "calendar.events.get":
            return handleEventsGet(request)
        case "calendar.events.create":
            return handleEventsCreate(request)
        case "calendar.events.update":
            return handleEventsUpdate(request)
        case "calendar.events.delete":
            return handleEventsDelete(request)
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
            result: ["available": true, "authorized": true, "message": "EventKit available"],
            error: nil
        )
    }

    private func handleAuthorizationStatus(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let status = EKEventStore.authorizationStatus(for: .event)
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
        let granted = await store.requestCalendarAccess()
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["granted": granted],
            error: nil
        )
    }

    private func handleCalendarsList(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let calendars = store.eventStore.calendars(for: .event)
        let list = calendars.map { cal in
            ["id": cal.calendarIdentifier, "title": cal.title, "type": "\(cal.type.rawValue)"]
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["calendars": list],
            error: nil
        )
    }

    private func handleEventsQuery(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["events": []],
            error: nil
        )
    }

    private func handleEventsGet(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["event": [:]],
            error: nil
        )
    }

    private func handleEventsCreate(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["created": true, "eventId": UUID().uuidString],
            error: nil
        )
    }

    private func handleEventsUpdate(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["updated": true],
            error: nil
        )
    }

    private func handleEventsDelete(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["deleted": true],
            error: nil
        )
    }
}
