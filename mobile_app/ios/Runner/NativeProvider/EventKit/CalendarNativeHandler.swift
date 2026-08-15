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

    public func capabilitySnapshot() -> IOSNativeCapability {
        let status = EKEventStore.authorizationStatus(for: .event)
        let authorized = status == .authorized || status == .fullAccess
        return IOSNativeCapability(
            available: true,
            authorized: authorized,
            hardwareAvailable: true,
            platformSupported: true,
            foregroundRequired: false
        )
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
            return await handleEventsQuery(request)
        case "calendar.events.get":
            return await handleEventsGet(request)
        case "calendar.events.create":
            return await handleEventsCreate(request)
        case "calendar.events.update":
            return await handleEventsUpdate(request)
        case "calendar.events.delete":
            return await handleEventsDelete(request)
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
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: ["available": true, "message": "EventKit Calendar available"],
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
            requestId: request.requestId,
            status: "ok",
            result: ["authorized": authorized, "status": "\(status.rawValue)"],
            error: nil
        )
    }

    private func handleAuthorizationRequest(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let granted = await store.requestCalendarAccess()
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
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
            requestId: request.requestId,
            status: "ok",
            result: ["calendars": list],
            error: nil
        )
    }

    private func handleEventsQuery(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let startDate = (request.payload?["startDate"] as? Date) ?? Date.distantPast
        let endDate = (request.payload?["endDate"] as? Date) ?? Date()
        let calendarId = request.payload?["calendarId"] as? String

        var calendars: [EKCalendar] = []
        if let calendarId = calendarId {
            calendars = store.eventStore.calendars(for: .event).filter { $0.calendarIdentifier == calendarId }
        } else {
            calendars = store.eventStore.calendars(for: .event)
        }

        let predicate = store.eventStore.predicateForEvents(withStart: startDate, end: endDate, calendars: calendars.isEmpty ? nil : calendars)
        let events = store.eventStore.events(matching: predicate)

        let results = events.map { event in
            [
                "id": event.eventIdentifier,
                "title": event.title ?? "",
                "startDate": event.startDate.timeIntervalSince1970,
                "endDate": event.endDate.timeIntervalSince1970,
                "calendarId": event.calendar.calendarIdentifier
            ]
        }

        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: ["events": results, "count": results.count],
            error: nil
        )
    }

    private func handleEventsGet(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let eventId = request.payload?["id"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing event id")
            )
        }

        guard let event = store.eventStore.event(withIdentifier: eventId) else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "NOT_FOUND", message: "event not found: \(eventId)")
            )
        }

        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: [
                "event": [
                    "id": event.eventIdentifier,
                    "title": event.title ?? "",
                    "startDate": event.startDate.timeIntervalSince1970,
                    "endDate": event.endDate.timeIntervalSince1970,
                    "calendarId": event.calendar.calendarIdentifier,
                    "notes": event.notes ?? ""
                ]
            ],
            error: nil
        )
    }

    private func handleEventsCreate(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let event = EKEvent(eventStore: store.eventStore)

        guard let title = request.payload?["title"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing title")
            )
        }

        event.title = title
        event.startDate = (request.payload?["startDate"] as? Date) ?? Date()
        event.endDate = (request.payload?["endDate"] as? Date) ?? Date().addingTimeInterval(3600)
        event.notes = request.payload?["notes"] as? String

        if let calendarId = request.payload?["calendarId"] as? String,
           let calendar = store.eventStore.calendars(for: .event).first(where: { $0.calendarIdentifier == calendarId }) {
            event.calendar = calendar
        } else {
            event.calendar = store.eventStore.defaultCalendarForNewEvents
        }

        do {
            try store.eventStore.save(event, span: .thisEvent)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["created": true, "eventId": event.eventIdentifier],
                error: nil
            )
        } catch {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "CREATE_FAILED", message: error.localizedDescription)
            )
        }
    }

    private func handleEventsUpdate(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let eventId = request.payload?["id"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing event id")
            )
        }

        guard let event = store.eventStore.event(withIdentifier: eventId) else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "NOT_FOUND", message: "event not found: \(eventId)")
            )
        }

        if let title = request.payload?["title"] as? String {
            event.title = title
        }
        if let startDate = request.payload?["startDate"] as? Date {
            event.startDate = startDate
        }
        if let endDate = request.payload?["endDate"] as? Date {
            event.endDate = endDate
        }
        if let notes = request.payload?["notes"] as? String {
            event.notes = notes
        }

        do {
            try store.eventStore.save(event, span: .thisEvent)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["updated": true, "eventId": event.eventIdentifier],
                error: nil
            )
        } catch {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "UPDATE_FAILED", message: error.localizedDescription)
            )
        }
    }

    private func handleEventsDelete(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let eventId = request.payload?["id"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing event id")
            )
        }

        guard let event = store.eventStore.event(withIdentifier: eventId) else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "NOT_FOUND", message: "event not found: \(eventId)")
            )
        }

        do {
            try store.eventStore.remove(event, span: .thisEvent)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["deleted": true, "eventId": eventId],
                error: nil
            )
        } catch {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "DELETE_FAILED", message: error.localizedDescription)
            )
        }
    }
}
