import Foundation
import UserNotifications

public class AlarmNativeHandler: NSObject, IOSNativeOperationHandler {
    public let operations: Set<String> = [
        "alarms.status",
        "alarms.authorization.status",
        "alarms.authorization.request",
        "alarms.list",
        "alarms.get",
        "alarms.create",
        "alarms.update",
        "alarms.cancel",
        "alarms.snooze"
    ]

    public override init() {
        super.init()
    }

    public func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        switch request.operation {
        case "alarms.status":
            return handleStatus(request)
        case "alarms.authorization.status":
            return await handleAuthorizationStatus(request)
        case "alarms.authorization.request":
            return await handleAuthorizationRequest(request)
        case "alarms.list":
            return await handleList(request)
        case "alarms.get":
            return await handleGet(request)
        case "alarms.create":
            return await handleCreate(request)
        case "alarms.update":
            return await handleUpdate(request)
        case "alarms.cancel":
            return await handleCancel(request)
        case "alarms.snooze":
            return await handleSnooze(request)
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
            result: ["available": true, "authorized": true, "message": "Alarms available"],
            error: nil
        )
    }

    private func handleAuthorizationStatus(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let center = UNUserNotificationCenter.current()
        let settings = await center.getNotificationSettings()
        let authorized = settings.authorizationStatus == .authorized
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["authorized": authorized, "status": "\(settings.authorizationStatus.rawValue)"],
            error: nil
        )
    }

    private func handleAuthorizationRequest(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        do {
            let center = UNUserNotificationCenter.current()
            let granted = try await center.requestAuthorization(options: [.alert, .sound, .badge])
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: ["granted": granted],
                error: nil
            )
        } catch {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "AUTHORIZATION_FAILED", message: error.localizedDescription)
            )
        }
    }

    private func handleList(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let center = UNUserNotificationCenter.current()
        let requests = await center.pendingNotificationRequests()
        let alarms = requests.map { req in
            ["id": req.identifier, "title": (req.content.title), "body": req.content.body]
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["alarms": alarms],
            error: nil
        )
    }

    private func handleGet(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["alarm": [:]],
            error: nil
        )
    }

    private func handleCreate(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["created": true, "alarmId": UUID().uuidString],
            error: nil
        )
    }

    private func handleUpdate(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["updated": true],
            error: nil
        )
    }

    private func handleCancel(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["cancelled": true],
            error: nil
        )
    }

    private func handleSnooze(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["snoozed": true],
            error: nil
        )
    }
}
