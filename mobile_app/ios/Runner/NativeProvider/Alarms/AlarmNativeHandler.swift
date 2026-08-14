import Foundation
import UserNotifications

public class AlarmNativeHandler: NSObject, IOSNativeOperationHandler {
    public let operations: Set<String> = [
        "media.alarms.status",
        "media.alarms.authorization.status",
        "media.alarms.authorization.request",
        "media.alarms.list",
        "media.alarms.get",
        "media.alarms.schedule",
        "media.alarms.stop",
        "media.alarms.cancel",
        "media.alarms.countdown",
        "media.alarms.pause",
        "media.alarms.resume"
    ]

    public override init() {
        super.init()
    }

    public func capabilitySnapshot() -> IOSNativeCapability {
        if #available(iOS 10.0, *) {
            return IOSNativeCapability(
                available: true,
                authorized: false,
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
        case "media.alarms.status":
            return handleStatus(request)
        case "media.alarms.authorization.status":
            return await handleAuthorizationStatus(request)
        case "media.alarms.authorization.request":
            return await handleAuthorizationRequest(request)
        case "media.alarms.list":
            return await handleList(request)
        case "media.alarms.get":
            return await handleGet(request)
        case "media.alarms.schedule":
            return await handleSchedule(request)
        case "media.alarms.stop":
            return await handleStop(request)
        case "media.alarms.cancel":
            return await handleCancel(request)
        case "media.alarms.countdown":
            return await handleCountdown(request)
        case "media.alarms.pause":
            return await handlePause(request)
        case "media.alarms.resume":
            return await handleResume(request)
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
        if #available(iOS 10.0, *) {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: ["available": true, "message": "Alarm scheduling available via UNUserNotificationCenter"],
                error: nil
            )
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "iOS 10.0+ required for alarm functionality")
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
            ["id": req.identifier, "title": req.content.title, "body": req.content.body]
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
        guard let alarmId = request.payload?["id"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing alarm id")
            )
        }
        let center = UNUserNotificationCenter.current()
        let requests = await center.pendingNotificationRequests()
        guard let req = requests.first(where: { $0.identifier == alarmId }) else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "NOT_FOUND", message: "alarm not found: \(alarmId)")
            )
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["alarm": ["id": req.identifier, "title": req.content.title, "body": req.content.body]],
            error: nil
        )
    }

    private func handleSchedule(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let alarmId = request.payload?["id"] as? String,
              let title = request.payload?["title"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing id or title")
            )
        }

        let center = UNUserNotificationCenter.current()
        let settings = await center.getNotificationSettings()
        guard settings.authorizationStatus == .authorized else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "AUTHORIZATION_DENIED", message: "notification authorization required")
            )
        }

        let content = UNMutableNotificationContent()
        content.title = title
        if let body = request.payload?["body"] as? String {
            content.body = body
        }
        content.sound = .default

        let trigger: UNNotificationTrigger
        if let timeInterval = request.payload?["timeInterval"] as? Double {
            trigger = UNTimeIntervalNotificationTrigger(timeInterval: max(timeInterval, 1), repeats: false)
        } else if let dateComponents = request.payload?["dateComponents"] as? [String: Int] {
            let dc = DateComponents(
                year: dateComponents["year"],
                month: dateComponents["month"],
                day: dateComponents["day"],
                hour: dateComponents["hour"],
                minute: dateComponents["minute"]
            )
            trigger = UNCalendarNotificationTrigger(dateMatching: dc, repeats: false)
        } else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing timeInterval or dateComponents")
            )
        }

        let request_notif = UNNotificationRequest(identifier: alarmId, content: content, trigger: trigger)

        do {
            try await center.add(request_notif)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: ["scheduled": true, "alarmId": alarmId],
                error: nil
            )
        } catch {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "SCHEDULE_FAILED", message: error.localizedDescription)
            )
        }
    }

    private func handleStop(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let alarmId = request.payload?["id"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing alarm id")
            )
        }
        let center = UNUserNotificationCenter.current()
        center.removePendingNotificationRequests(withIdentifiers: [alarmId])
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["stopped": true, "alarmId": alarmId],
            error: nil
        )
    }

    private func handleCancel(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let alarmId = request.payload?["id"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing alarm id")
            )
        }
        let center = UNUserNotificationCenter.current()
        center.removePendingNotificationRequests(withIdentifiers: [alarmId])
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["cancelled": true, "alarmId": alarmId],
            error: nil
        )
    }

    private func handleCountdown(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let seconds = request.payload?["seconds"] as? Double else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing seconds")
            )
        }
        let alarmId = "countdown-\(UUID().uuidString)"
        var countdownPayload = request.payload ?? [:]
        countdownPayload["id"] = alarmId
        countdownPayload["title"] = countdownPayload["title"] ?? "Countdown"
        countdownPayload["timeInterval"] = seconds

        let countdownRequest = IOSNativeRequest(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            platform: request.platform,
            operation: "media.alarms.schedule",
            payload: countdownPayload
        )
        return await handleSchedule(countdownRequest)
    }

    private func handlePause(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "pause not supported by UNUserNotificationCenter")
        )
    }

    private func handleResume(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "resume not supported by UNUserNotificationCenter")
        )
    }
}
