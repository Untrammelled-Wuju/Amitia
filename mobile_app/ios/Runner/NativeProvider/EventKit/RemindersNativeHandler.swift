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

    public func capabilitySnapshot() -> IOSNativeCapability {
        let status = EKEventStore.authorizationStatus(for: .reminder)
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
        case "reminders.status":
            return handleStatus(request)
        case "reminders.authorization.status":
            return await handleAuthorizationStatus(request)
        case "reminders.authorization.request":
            return await handleAuthorizationRequest(request)
        case "reminders.lists.list":
            return handleListsList(request)
        case "reminders.query":
            return await handleQuery(request)
        case "reminders.get":
            return await handleGet(request)
        case "reminders.create":
            return await handleCreate(request)
        case "reminders.update":
            return await handleUpdate(request)
        case "reminders.complete":
            return await handleComplete(request)
        case "reminders.uncomplete":
            return await handleUncomplete(request)
        case "reminders.delete":
            return await handleDelete(request)
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
            result: ["available": true, "message": "Reminders available"],
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

    private func handleQuery(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let calendarId = request.payload?["calendarId"] as? String
        var calendars: [EKCalendar]? = nil
        if let calendarId = calendarId {
            calendars = store.eventStore.calendars(for: .reminder).filter { $0.calendarIdentifier == calendarId }
            if calendars?.isEmpty == true { calendars = nil }
        }

        let predicate = store.eventStore.predicateForReminders(in: calendars)

        return await withCheckedContinuation { continuation in
            store.eventStore.fetchReminders(matching: predicate) { reminders in
                let ekReminders = reminders ?? []
                let results = ekReminders.map { reminder in
                    [
                        "id": reminder.calendarItemIdentifier,
                        "title": reminder.title ?? "",
                        "isCompleted": reminder.isCompleted,
                        "dueDate": reminder.dueDateComponents?.date?.timeIntervalSince1970 ?? 0,
                        "calendarId": reminder.calendar.calendarIdentifier
                    ]
                }

                continuation.resume(returning: IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestID: request.requestID,
                    status: "ok",
                    result: ["reminders": results, "count": results.count],
                    error: nil
                ))
            }
        }
    }

    private func handleGet(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let reminderId = request.payload?["id"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing reminder id")
            )
        }

        let predicate = store.eventStore.predicateForReminders(in: nil)

        return await withCheckedContinuation { continuation in
            store.eventStore.fetchReminders(matching: predicate) { reminders in
                guard let reminder = reminders?.first(where: { $0.calendarItemIdentifier == reminderId }) else {
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestID: request.requestID,
                        status: "error",
                        result: nil,
                        error: IOSNativeError(code: "NOT_FOUND", message: "reminder not found: \(reminderId)")
                    ))
                    return
                }

                continuation.resume(returning: IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestID: request.requestID,
                    status: "ok",
                    result: [
                        "reminder": [
                            "id": reminder.calendarItemIdentifier,
                            "title": reminder.title ?? "",
                            "isCompleted": reminder.isCompleted,
                            "dueDate": reminder.dueDateComponents?.date?.timeIntervalSince1970 ?? 0,
                            "notes": reminder.notes ?? ""
                        ]
                    ],
                    error: nil
                ))
            }
        }
    }

    private func handleCreate(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let reminder = EKReminder(eventStore: store.eventStore)

        guard let title = request.payload?["title"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing title")
            )
        }

        reminder.title = title
        reminder.notes = request.payload?["notes"] as? String

        if let dueDate = request.payload?["dueDate"] as? Date {
            reminder.dueDateComponents = Calendar.current.dateComponents([.year, .month, .day, .hour, .minute], from: dueDate)
        }

        if let calendarId = request.payload?["calendarId"] as? String,
           let calendar = store.eventStore.calendars(for: .reminder).first(where: { $0.calendarIdentifier == calendarId }) {
            reminder.calendar = calendar
        } else {
            reminder.calendar = store.eventStore.defaultCalendarForNewReminders()
        }

        do {
            try store.eventStore.save(reminder, commit: true)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: ["created": true, "reminderId": reminder.calendarItemIdentifier],
                error: nil
            )
        } catch {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "CREATE_FAILED", message: error.localizedDescription)
            )
        }
    }

    private func handleUpdate(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let reminderId = request.payload?["id"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing reminder id")
            )
        }

        let predicate = store.eventStore.predicateForReminders(in: nil)

        return await withCheckedContinuation { continuation in
            store.eventStore.fetchReminders(matching: predicate) { reminders in
                guard let reminder = reminders?.first(where: { $0.calendarItemIdentifier == reminderId }) else {
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestID: request.requestID,
                        status: "error",
                        result: nil,
                        error: IOSNativeError(code: "NOT_FOUND", message: "reminder not found: \(reminderId)")
                    ))
                    return
                }

                if let title = request.payload?["title"] as? String {
                    reminder.title = title
                }
                if let notes = request.payload?["notes"] as? String {
                    reminder.notes = notes
                }
                if let dueDate = request.payload?["dueDate"] as? Date {
                    reminder.dueDateComponents = Calendar.current.dateComponents([.year, .month, .day, .hour, .minute], from: dueDate)
                }

                do {
                    try self.store.eventStore.save(reminder, commit: true)
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestID: request.requestID,
                        status: "ok",
                        result: ["updated": true, "reminderId": reminder.calendarItemIdentifier],
                        error: nil
                    ))
                } catch {
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestID: request.requestID,
                        status: "error",
                        result: nil,
                        error: IOSNativeError(code: "UPDATE_FAILED", message: error.localizedDescription)
                    ))
                }
            }
        }
    }

    private func handleComplete(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let reminderId = request.payload?["id"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing reminder id")
            )
        }

        let predicate = store.eventStore.predicateForReminders(in: nil)

        return await withCheckedContinuation { continuation in
            store.eventStore.fetchReminders(matching: predicate) { reminders in
                guard let reminder = reminders?.first(where: { $0.calendarItemIdentifier == reminderId }) else {
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestID: request.requestID,
                        status: "error",
                        result: nil,
                        error: IOSNativeError(code: "NOT_FOUND", message: "reminder not found: \(reminderId)")
                    ))
                    return
                }

                reminder.isCompleted = true

                do {
                    try self.store.eventStore.save(reminder, commit: true)
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestID: request.requestID,
                        status: "ok",
                        result: ["completed": true, "reminderId": reminder.calendarItemIdentifier],
                        error: nil
                    ))
                } catch {
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestID: request.requestID,
                        status: "error",
                        result: nil,
                        error: IOSNativeError(code: "UPDATE_FAILED", message: error.localizedDescription)
                    ))
                }
            }
        }
    }

    private func handleUncomplete(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let reminderId = request.payload?["id"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing reminder id")
            )
        }

        let predicate = store.eventStore.predicateForReminders(in: nil)

        return await withCheckedContinuation { continuation in
            store.eventStore.fetchReminders(matching: predicate) { reminders in
                guard let reminder = reminders?.first(where: { $0.calendarItemIdentifier == reminderId }) else {
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestID: request.requestID,
                        status: "error",
                        result: nil,
                        error: IOSNativeError(code: "NOT_FOUND", message: "reminder not found: \(reminderId)")
                    ))
                    return
                }

                reminder.isCompleted = false

                do {
                    try self.store.eventStore.save(reminder, commit: true)
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestID: request.requestID,
                        status: "ok",
                        result: ["uncompleted": true, "reminderId": reminder.calendarItemIdentifier],
                        error: nil
                    ))
                } catch {
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestID: request.requestID,
                        status: "error",
                        result: nil,
                        error: IOSNativeError(code: "UPDATE_FAILED", message: error.localizedDescription)
                    ))
                }
            }
        }
    }

    private func handleDelete(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let reminderId = request.payload?["id"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing reminder id")
            )
        }

        let predicate = store.eventStore.predicateForReminders(in: nil)

        return await withCheckedContinuation { continuation in
            store.eventStore.fetchReminders(matching: predicate) { reminders in
                guard let reminder = reminders?.first(where: { $0.calendarItemIdentifier == reminderId }) else {
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestID: request.requestID,
                        status: "error",
                        result: nil,
                        error: IOSNativeError(code: "NOT_FOUND", message: "reminder not found: \(reminderId)")
                    ))
                    return
                }

                do {
                    try self.store.eventStore.remove(reminder, commit: true)
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestID: request.requestID,
                        status: "ok",
                        result: ["deleted": true, "reminderId": reminderId],
                        error: nil
                    ))
                } catch {
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestID: request.requestID,
                        status: "error",
                        result: nil,
                        error: IOSNativeError(code: "DELETE_FAILED", message: error.localizedDescription)
                    ))
                }
            }
        }
    }
}
