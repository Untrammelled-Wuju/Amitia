import Foundation
#if canImport(AlarmKit)
import AlarmKit
#endif

public enum AlarmKitAvailability {
    case available
    case unsupported(String)
}

public enum AlarmAuthorizationStatus: String, Sendable {
    case authorized
    case denied
    case notDetermined
}

public struct AmitiaAlarmScheduleDTO: Codable {
    var fireAt: String?
    var hour: Int?
    var minute: Int?
    var recurrence: String?
    var weekdays: [String]?
}

public struct AmitiaAlarmPresentationDTO: Codable {
    var alertTitle: String?
    var countdownTitle: String?
    var pausedTitle: String?
    var tintColor: String?
    var secondaryAction: String?
}

public struct AmitiaAlarmSoundDTO: Codable {
    var kind: String?
    var soundId: String?
}

public struct AmitiaAlarmMetadataDTO: Codable {
    var kind: String?
    var icon: String?
    var ownerRef: String?
}

public class AlarmKitAdapter {
    public static let shared = AlarmKitAdapter()

    private init() {}

    public func checkAvailability() -> AlarmKitAvailability {
        if isAlarmKitAvailable() {
            return .available
        }
        let majorVersion = Int((UIDevice.current.systemVersion).split(separator: ".").first ?? "25") ?? 25
        if majorVersion < 26 {
            return .unsupported("PLATFORM_NOT_SUPPORTED")
        }
        return .unsupported("PLATFORM_NOT_SUPPORTED")
    }

    public func authorizationStatus() -> AlarmAuthorizationStatus {
        guard isAlarmKitAvailable() else { return .notDetermined }
        #if canImport(AlarmKit)
        if #available(iOS 26.0, *) {
            let manager = AlarmManager.shared
            switch manager.authorizationState {
            case .authorized: return .authorized
            case .denied: return .denied
            case .notDetermined: return .notDetermined
            @unknown default: return .notDetermined
            }
        }
        #endif
        return .notDetermined
    }

    public func requestAuthorization() async -> (state: AlarmAuthorizationStatus, error: String?) {
        guard isAlarmKitAvailable() else { return (.notDetermined, "PLATFORM_NOT_SUPPORTED") }
        #if canImport(AlarmKit)
        if #available(iOS 26.0, *) {
            do {
                let state = try await AlarmManager.shared.requestAuthorization()
                switch state {
                case .authorized: return (.authorized, nil)
                case .denied: return (.denied, nil)
                case .notDetermined: return (.notDetermined, nil)
                @unknown default: return (.notDetermined, nil)
                }
            } catch {
                return (.denied, error.localizedDescription)
            }
        }
        #endif
        return (.notDetermined, "PLATFORM_NOT_SUPPORTED")
    }

    public func createAlarm(
        id: String,
        title: String,
        schedule: AmitiaAlarmScheduleDTO,
        presentation: AmitiaAlarmPresentationDTO,
        sound: AmitiaAlarmSoundDTO?,
        metadata: AmitiaAlarmMetadataDTO?
    ) async -> (success: Bool, error: String?) {
        guard isAlarmKitAvailable() else { return (false, "PLATFORM_NOT_SUPPORTED") }
        #if canImport(AlarmKit)
        if #available(iOS 26.0, *) {
            let result = await createAlarmInternal(
                id: id,
                title: title,
                schedule: schedule,
                presentation: presentation,
                sound: sound
            )
            return result
        }
        #endif
        return (false, "PLATFORM_NOT_SUPPORTED")
    }

    public func cancelAlarm(id: String) async -> (success: Bool, error: String?) {
        guard isAlarmKitAvailable() else { return (false, "PLATFORM_NOT_SUPPORTED") }
        #if canImport(AlarmKit)
        if #available(iOS 26.0, *) {
            do {
                try await AlarmManager.shared.cancel(id: id)
                return (true, nil)
            } catch {
                return (false, error.localizedDescription)
            }
        }
        #endif
        return (false, "PLATFORM_NOT_SUPPORTED")
    }

    public func stopAlarm(id: String) async -> (success: Bool, error: String?) {
        guard isAlarmKitAvailable() else { return (false, "PLATFORM_NOT_SUPPORTED") }
        #if canImport(AlarmKit)
        if #available(iOS 26.0, *) {
            do {
                guard let alarm = try await AlarmManager.shared.alarm(id: id) else {
                    return (false, "ALARM_NOT_FOUND")
                }
                try await alarm.stop()
                return (true, nil)
            } catch {
                return (false, error.localizedDescription)
            }
        }
        #endif
        return (false, "PLATFORM_NOT_SUPPORTED")
    }

    public func listAlarms() async -> (alarms: [[String: Any]], error: String?) {
        guard isAlarmKitAvailable() else { return ([], "PLATFORM_NOT_SUPPORTED") }
        #if canImport(AlarmKit)
        if #available(iOS 26.0, *) {
            do {
                let alarms = try await AlarmManager.shared.alarms
                let result = alarms.map { alarm -> [String: Any] in
                    var item: [String: Any] = [
                        "id": alarm.id,
                        "title": alarm.title
                    ]
                    item["state"] = String(describing: alarm.state)
                    if let countdown = alarm.countdown {
                        item["countdownDuration"] = countdown.duration
                    }
                    return item
                }
                return (result, nil)
            } catch {
                return ([], error.localizedDescription)
            }
        }
        #endif
        return ([], "PLATFORM_NOT_SUPPORTED")
    }

    public func getAlarm(id: String) async -> (alarm: [String: Any]?, error: String?) {
        guard isAlarmKitAvailable() else { return (nil, "PLATFORM_NOT_SUPPORTED") }
        #if canImport(AlarmKit)
        if #available(iOS 26.0, *) {
            do {
                guard let target = try await AlarmManager.shared.alarm(id: id) else {
                    return (nil, "ALARM_NOT_FOUND")
                }
                var item: [String: Any] = [
                    "id": target.id,
                    "title": target.title
                ]
                item["state"] = String(describing: target.state)
                if let countdown = target.countdown {
                    item["countdownDuration"] = countdown.duration
                }
                return (item, nil)
            } catch {
                return (nil, error.localizedDescription)
            }
        }
        #endif
        return (nil, "PLATFORM_NOT_SUPPORTED")
    }

    private func isAlarmKitAvailable() -> Bool {
        if #available(iOS 26.0, *) {
            #if canImport(AlarmKit)
            return true
            #endif
        }
        return false
    }

    #if canImport(AlarmKit)
    @available(iOS 26.0, *)
    private func createAlarmInternal(
        id: String,
        title: String,
        schedule: AmitiaAlarmScheduleDTO,
        presentation: AmitiaAlarmPresentationDTO,
        sound: AmitiaAlarmSoundDTO?
    ) async -> (success: Bool, error: String?) {
        do {
            let manager = AlarmManager.shared
            let config = buildAlarmConfiguration(title: title, schedule: schedule, presentation: presentation, sound: sound)
            try await manager.schedule(id: id, configuration: config)
            return (true, nil)
        } catch {
            return (false, error.localizedDescription)
        }
    }

    @available(iOS 26.0, *)
    private func buildAlarmConfiguration(
        title: String,
        schedule: AmitiaAlarmScheduleDTO,
        presentation: AmitiaAlarmPresentationDTO,
        sound: AmitiaAlarmSoundDTO?
    ) -> AlarmKit.AlarmConfiguration {
        var config = AlarmKit.AlarmConfiguration(title: title)
        if let alarmSchedule = buildAlarmSchedule(schedule) {
            config.schedule = alarmSchedule
        }
        config.presentation = buildAlarmPresentation(presentation)
        return config
    }

    @available(iOS 26.0, *)
    private func buildAlarmSchedule(_ schedule: AmitiaAlarmScheduleDTO) -> AlarmKit.AlarmSchedule? {
        if let fireAt = schedule.fireAt, let date = ISO8601DateFormatter().date(from: fireAt) {
            return AlarmKit.AlarmSchedule(date: date)
        }
        if let hour = schedule.hour, let minute = schedule.minute {
            var dc = DateComponents()
            dc.hour = hour
            dc.minute = minute
            return AlarmKit.AlarmSchedule(dateComponents: dc)
        }
        return nil
    }

    @available(iOS 26.0, *)
    private func buildAlarmPresentation(_ presentation: AmitiaAlarmPresentationDTO) -> AlarmKit.AlarmPresentation? {
        var p = AlarmKit.AlarmPresentation()
        if let alertTitle = presentation.alertTitle {
            p.alertTitle = alertTitle
        }
        if let countdownTitle = presentation.countdownTitle {
            p.countdownTitle = countdownTitle
        }
        if let pausedTitle = presentation.pausedTitle {
            p.pausedTitle = pausedTitle
        }
        return p
    }
    #endif
}

