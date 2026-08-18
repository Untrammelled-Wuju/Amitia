import Foundation
import UIKit
#if canImport(AlarmKit)
import AlarmKit
import SwiftUI
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

#if canImport(AlarmKit)
@available(iOS 26.0, *)
private struct AmitiaAlarmKitMetadata: AlarmMetadata {
    let title: String
    let kind: String?
    let ownerRef: String?
}
#endif

public final class AlarmKitAdapter {
    public static let shared = AlarmKitAdapter()

    private init() {}

    public func checkAvailability() -> AlarmKitAvailability {
        isAlarmKitAvailable() ? .available : .unsupported("PLATFORM_NOT_SUPPORTED")
    }

    public func authorizationStatus() -> AlarmAuthorizationStatus {
        guard isAlarmKitAvailable() else { return .notDetermined }
        #if canImport(AlarmKit)
        if #available(iOS 26.0, *) {
            switch AlarmManager.shared.authorizationState {
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
                switch try await AlarmManager.shared.requestAuthorization() {
                case .authorized: return (.authorized, nil)
                case .denied: return (.denied, nil)
                case .notDetermined: return (.notDetermined, nil)
                @unknown default: return (.notDetermined, nil)
                }
            } catch {
                return (.notDetermined, error.localizedDescription)
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
            guard let alarmID = UUID(uuidString: id) else {
                return (false, "INVALID_ALARM_ID")
            }
            if let sound, sound.kind == "named" {
                return (false, "CUSTOM_SOUND_NOT_SUPPORTED")
            }
            do {
                let alarmSchedule = try buildAlarmSchedule(schedule)
                let stopButton = AlarmButton(
                    text: "Dismiss",
                    textColor: .white,
                    systemImageName: "stop.circle"
                )
                let alert = AlarmPresentation.Alert(
                    title: presentation.alertTitle ?? title,
                    stopButton: stopButton
                )
                let attributes = AlarmAttributes<AmitiaAlarmKitMetadata>(
                    presentation: AlarmPresentation(alert: alert),
                    metadata: AmitiaAlarmKitMetadata(
                        title: title,
                        kind: metadata?.kind,
                        ownerRef: metadata?.ownerRef
                    ),
                    tintColor: tintColor(presentation.tintColor)
                )
                typealias Configuration = AlarmManager.AlarmConfiguration<AmitiaAlarmKitMetadata>
                let configuration = Configuration.alarm(
                    schedule: alarmSchedule,
                    attributes: attributes,
                    stopIntent: nil,
                    secondaryIntent: nil,
                    sound: .default
                )
                _ = try await AlarmManager.shared.schedule(id: alarmID, configuration: configuration)
                return (true, nil)
            } catch {
                return (false, error.localizedDescription)
            }
        }
        #endif
        return (false, "PLATFORM_NOT_SUPPORTED")
    }

    public func cancelAlarm(id: String) async -> (success: Bool, error: String?) {
        guard isAlarmKitAvailable() else { return (false, "PLATFORM_NOT_SUPPORTED") }
        #if canImport(AlarmKit)
        if #available(iOS 26.0, *) {
            guard let alarmID = UUID(uuidString: id) else { return (false, "INVALID_ALARM_ID") }
            do {
                try AlarmManager.shared.cancel(id: alarmID)
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
            guard let alarmID = UUID(uuidString: id) else { return (false, "INVALID_ALARM_ID") }
            do {
                try AlarmManager.shared.stop(id: alarmID)
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
                let alarms = try AlarmManager.shared.alarms
                return (alarms.map(alarmDictionary), nil)
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
            guard let alarmID = UUID(uuidString: id) else { return (nil, "INVALID_ALARM_ID") }
            do {
                let alarms = try AlarmManager.shared.alarms
                guard let target = alarms.first(where: { $0.id == alarmID }) else {
                    return (nil, "ALARM_NOT_FOUND")
                }
                return (alarmDictionary(target), nil)
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
    private func buildAlarmSchedule(_ schedule: AmitiaAlarmScheduleDTO) throws -> Alarm.Schedule {
        if let fireAt = schedule.fireAt {
            guard let date = ISO8601DateFormatter().date(from: fireAt) else {
                throw NSError(domain: "AmitiaAlarmKit", code: 1, userInfo: [NSLocalizedDescriptionKey: "invalid fireAt"])
            }
            return .fixed(date)
        }
        guard let hour = schedule.hour, let minute = schedule.minute else {
            throw NSError(domain: "AmitiaAlarmKit", code: 2, userInfo: [NSLocalizedDescriptionKey: "missing alarm schedule"])
        }
        let time = Alarm.Schedule.Relative.Time(hour: hour, minute: minute)
        let recurrence: Alarm.Schedule.Relative.Recurrence
        if schedule.recurrence == "weekly" {
            let weekdays = try (schedule.weekdays ?? []).map(localeWeekday)
            guard !weekdays.isEmpty else {
                throw NSError(domain: "AmitiaAlarmKit", code: 3, userInfo: [NSLocalizedDescriptionKey: "weekly recurrence requires weekdays"])
            }
            recurrence = .weekly(weekdays)
        } else {
            recurrence = .never
        }
        return .relative(Alarm.Schedule.Relative(time: time, repeats: recurrence))
    }

    @available(iOS 26.0, *)
    private func localeWeekday(_ weekday: String) throws -> Locale.Weekday {
        switch weekday.lowercased() {
        case "monday": return .monday
        case "tuesday": return .tuesday
        case "wednesday": return .wednesday
        case "thursday": return .thursday
        case "friday": return .friday
        case "saturday": return .saturday
        case "sunday": return .sunday
        default:
            throw NSError(domain: "AmitiaAlarmKit", code: 4, userInfo: [NSLocalizedDescriptionKey: "invalid weekday: \(weekday)"])
        }
    }

    @available(iOS 26.0, *)
    private func tintColor(_ value: String?) -> Color {
        switch value {
        case "amitia-green": return .green
        case "amitia-orange": return .orange
        case "amitia-blue": return .blue
        default: return .blue
        }
    }

    @available(iOS 26.0, *)
    private func alarmDictionary(_ alarm: Alarm) -> [String: Any] {
        var item: [String: Any] = [
            "id": alarm.id.uuidString,
            "state": String(describing: alarm.state)
        ]
        item["schedule"] = String(describing: alarm.schedule)
        if let countdown = alarm.countdownDuration {
            item["countdownPreAlert"] = countdown.preAlert
            item["countdownPostAlert"] = countdown.postAlert
        }
        return item
    }
    #endif
}
