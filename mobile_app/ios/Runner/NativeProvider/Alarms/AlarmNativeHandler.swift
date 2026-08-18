import Foundation

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
        let availability = AlarmKitAdapter.shared.checkAvailability()
        switch availability {
        case .available:
            return IOSNativeCapability(
                available: true,
                authorized: false,
                hardwareAvailable: true,
                platformSupported: true,
                foregroundRequired: false
            )
        case .unsupported:
            return IOSNativeCapability(
                available: false,
                authorized: false,
                hardwareAvailable: true,
                platformSupported: false,
                foregroundRequired: false
            )
        }
    }

    @available(iOS, deprecated: 16.0, renamed: "iOS_alarm_unsupported_message")
    private static var unsupportedMessage: String {
        let majorVersion = Int((UIDevice.current.systemVersion).split(separator: ".").first ?? "25") ?? 25
        return "system clock alarm API requires iOS 26.0+ AlarmKit. Current iOS \(majorVersion) does not expose alarm alarm management."
    }

    public func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        switch request.operation {
        case "media.alarms.status":
            return handleStatus(request)
        case "media.alarms.authorization.status":
            return handleAuthorizationStatus(request)
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
            return handlePause(request)
        case "media.alarms.resume":
            return handleResume(request)
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
        let availability = AlarmKitAdapter.shared.checkAvailability()
        switch availability {
        case .available:
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: [
                    "available": true,
                    "message": "AlarmKit available on iOS 26+",
                    "platformVersion": UIDevice.current.systemVersion
                ],
                error: nil
            )
        case .unsupported(let reason):
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: reason)
            )
        }
    }

    private func handleAuthorizationStatus(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let availability = AlarmKitAdapter.shared.checkAvailability()
        switch availability {
        case .available:
            let status = AlarmKitAdapter.shared.authorizationStatus()
            let authString: String
            switch status {
            case .authorized: authString = "authorized"
            case .denied: authString = "denied"
            case .notDetermined: authString = "notDetermined"
            case .restricted: authString = "restricted"
            }
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["authorized": status == .authorized, "status": authString],
                error: nil
            )
        case .unsupported(let reason):
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: reason)
            )
        }
    }

    private func handleAuthorizationRequest(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let availability = AlarmKitAdapter.shared.checkAvailability()
        switch availability {
        case .available:
            let (state, error) = await AlarmKitAdapter.shared.requestAuthorization()
            if let error = error {
                return IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestId: request.requestId,
                    status: "error",
                    result: nil,
                    error: IOSNativeError(code: "AUTHORIZATION_FAILED", message: error)
                )
            }
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["granted": state == .authorized, "state": "\(state)"],
                error: nil
            )
        case .unsupported(let reason):
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: reason)
            )
        }
    }

    private func handleList(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let availability = AlarmKitAdapter.shared.checkAvailability()
        switch availability {
        case .available:
            let (alarms, error) = await AlarmKitAdapter.shared.listAlarms()
            if let error = error {
                return IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestId: request.requestId,
                    status: "error",
                    result: nil,
                    error: IOSNativeError(code: "ALARMKIT_ERROR", message: error)
                )
            }
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["alarms": alarms],
                error: nil
            )
        case .unsupported(let reason):
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: reason)
            )
        }
    }

    private func handleGet(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let alarmId = request.payload?["alarmId"] as? String ?? request.payload?["id"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing alarmId")
            )
        }

        let availability = AlarmKitAdapter.shared.checkAvailability()
        switch availability {
        case .available:
            let (alarm, error) = await AlarmKitAdapter.shared.getAlarm(id: alarmId)
            if let error = error {
                return IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestId: request.requestId,
                    status: "error",
                    result: nil,
                    error: IOSNativeError(code: "ALARMKIT_ERROR", message: error)
                )
            }
            guard let alarm = alarm else {
                return IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestId: request.requestId,
                    status: "error",
                    result: nil,
                    error: IOSNativeError(code: "NOT_FOUND", message: "alarm not found: \(alarmId)")
                )
            }
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["alarm": alarm],
                error: nil
            )
        case .unsupported(let reason):
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: reason)
            )
        }
    }

    private func handleSchedule(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let alarmId = request.payload?["alarmId"] as? String ?? request.payload?["id"] as? String,
              let title = request.payload?["title"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing alarmId or title")
            )
        }

        let availability = AlarmKitAdapter.shared.checkAvailability()
        switch availability {
        case .available:
            var schedule = AmitiaAlarmScheduleDTO()
            if let scheduleRaw = request.payload?["schedule"] as? [String: Any] {
                if let fireAt = scheduleRaw["fireAt"] as? String { schedule.fireAt = fireAt }
                if let hour = scheduleRaw["hour"] as? Int { schedule.hour = hour }
                if let minute = scheduleRaw["minute"] as? Int { schedule.minute = minute }
                if let recurrence = scheduleRaw["recurrence"] as? String { schedule.recurrence = recurrence }
                if let weekdays = scheduleRaw["weekdays"] as? [String] { schedule.weekdays = weekdays }
            }

            var presentation = AmitiaAlarmPresentationDTO()
            if let presRaw = request.payload?["presentation"] as? [String: Any] {
                if let alertTitle = presRaw["alertTitle"] as? String { presentation.alertTitle = alertTitle }
                if let countdownTitle = presRaw["countdownTitle"] as? String { presentation.countdownTitle = countdownTitle }
                if let pausedTitle = presRaw["pausedTitle"] as? String { presentation.pausedTitle = pausedTitle }
                if let tintColor = presRaw["tintColor"] as? String { presentation.tintColor = tintColor }
                if let secondaryAction = presRaw["secondaryAction"] as? String { presentation.secondaryAction = secondaryAction }
            } else {
                presentation.alertTitle = title
            }

            var sound: AmitiaAlarmSoundDTO?
            if let soundRaw = request.payload?["sound"] as? [String: Any] {
                sound = AmitiaAlarmSoundDTO()
                if let kind = soundRaw["kind"] as? String { sound?.kind = kind }
                if let soundId = soundRaw["soundId"] as? String { sound?.soundId = soundId }
            }

            var metadata: AmitiaAlarmMetadataDTO?
            if let metaRaw = request.payload?["metadata"] as? [String: Any] {
                metadata = AmitiaAlarmMetadataDTO()
                if let kind = metaRaw["kind"] as? String { metadata?.kind = kind }
                if let icon = metaRaw["icon"] as? String { metadata?.icon = icon }
                if let ownerRef = metaRaw["ownerRef"] as? String { metadata?.ownerRef = ownerRef }
            }

            let (success, error) = await AlarmKitAdapter.shared.createAlarm(
                id: alarmId,
                title: title,
                schedule: schedule,
                presentation: presentation,
                sound: sound,
                metadata: metadata
            )

            if success {
                return IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestId: request.requestId,
                    status: "ok",
                    result: ["scheduled": true, "alarmId": alarmId],
                    error: nil
                )
            } else {
                return IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestId: request.requestId,
                    status: "error",
                    result: nil,
                    error: IOSNativeError(code: "SCHEDULE_FAILED", message: error ?? "failed to schedule alarm \(alarmId)")
                )
            }
        case .unsupported(let reason):
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: reason)
            )
        }
    }

    private func handleStop(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let alarmId = request.payload?["alarmId"] as? String ?? request.payload?["id"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing alarmId")
            )
        }

        let availability = AlarmKitAdapter.shared.checkAvailability()
        switch availability {
        case .available:
            let (success, error) = await AlarmKitAdapter.shared.cancelAlarm(id: alarmId)
            if success {
                return IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestId: request.requestId,
                    status: "ok",
                    result: ["stopped": true, "alarmId": alarmId],
                    error: nil
                )
            } else {
                return IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestId: request.requestId,
                    status: "error",
                    result: nil,
                    error: IOSNativeError(code: "CANCEL_FAILED", message: error ?? "alarm not found: \(alarmId)")
                )
            }
        case .unsupported(let reason):
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: reason)
            )
        }
    }

    private func handleCancel(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let alarmId = request.payload?["alarmId"] as? String ?? request.payload?["id"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing alarmId")
            )
        }

        let availability = AlarmKitAdapter.shared.checkAvailability()
        switch availability {
        case .available:
            let (success, error) = await AlarmKitAdapter.shared.cancelAlarm(id: alarmId)
            if success {
                return IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestId: request.requestId,
                    status: "ok",
                    result: ["cancelled": true, "alarmId": alarmId],
                    error: nil
                )
            } else {
                return IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestId: request.requestId,
                    status: "error",
                    result: nil,
                    error: IOSNativeError(code: "CANCEL_FAILED", message: error ?? "alarm not found: \(alarmId)")
                )
            }
        case .unsupported(let reason):
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: reason)
            )
        }
    }

    private func handleCountdown(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let seconds = request.payload?["seconds"] as? Double else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing seconds")
            )
        }

        let alarmId = "countdown-\(UUID().uuidString)"
        var countdownPayload = request.payload ?? [:]
        if countdownPayload["alarmId"] == nil && countdownPayload["id"] == nil {
            countdownPayload["id"] = alarmId
        }
        let title = (countdownPayload["title"] as? String) ?? "Countdown"
        countdownPayload["title"] = title

        var schedule = AmitiaAlarmScheduleDTO()
        let futureDate = Date().addingTimeInterval(seconds)
        let formatter = ISO8601DateFormatter()
        schedule.fireAt = formatter.string(from: futureDate)
        countdownPayload["schedule"] = ["fireAt": formatter.string(from: futureDate)]

        countdownPayload["presentation"] = ["alertTitle": title]

        let countdownRequest = IOSNativeRequest(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            platform: request.platform,
            operation: "media.alarms.schedule",
            payload: countdownPayload
        )
        return await handleSchedule(countdownRequest)
    }

    private func handlePause(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let alarmId = request.payload?["alarmId"] as? String, !alarmId.isEmpty else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing required field: alarmId")
            )
        }
        let (success, error) = await AlarmKitAdapter.shared.pauseAlarm(id: alarmId)
        if success {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["alarmId": alarmId, "paused": true],
                error: nil
            )
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PAUSE_FAILED", message: error ?? "unknown error")
        )
    }

    private func handleResume(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let alarmId = request.payload?["alarmId"] as? String, !alarmId.isEmpty else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing required field: alarmId")
            )
        }
        let (success, error) = await AlarmKitAdapter.shared.resumeAlarm(id: alarmId)
        if success {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["alarmId": alarmId, "resumed": true],
                error: nil
            )
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "RESUME_FAILED", message: error ?? "unknown error")
        )
    }
}
