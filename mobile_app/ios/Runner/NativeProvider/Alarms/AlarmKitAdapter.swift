import Foundation
#if canImport(AlarmKit)
import AlarmKit
#endif

public enum AlarmKitAvailability {
    case available
    case unsupported(String)
}

public enum AlarmAuthorizationStatus {
    case authorized
    case denied
    case notDetermined
    case restricted
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
            return .unsupported("system clock alarm API requires iOS 26.0+ AlarmKit. Current iOS \(majorVersion) does not expose alarm management.")
        }
        return .unsupported("AlarmKit framework not available on this device")
    }

    public func authorizationStatus() -> AlarmAuthorizationStatus {
        guard isAlarmKitAvailable() else { return .notDetermined }
        #if canImport(AlarmKit)
        if #available(iOS 26.0, *) {
            let manager = AlarmManager.shared
            switch manager.authorizationStatus {
            case .authorized: return .authorized
            case .denied: return .denied
            case .notDetermined: return .notDetermined
            case .restricted: return .restricted
            @unknown default: return .notDetermined
            }
        }
        #endif
        return .notDetermined
    }

    public func requestAuthorization() async -> Bool {
        guard isAlarmKitAvailable() else { return false }
        #if canImport(AlarmKit)
        if #available(iOS 26.0, *) {
            do {
                return try await AlarmManager.shared.requestAuthorization()
            } catch {
                return false
            }
        }
        #endif
        return false
    }

    public func createAlarm(
        id: String,
        title: String,
        schedule: AmitiaAlarmScheduleDTO,
        presentation: AmitiaAlarmPresentationDTO,
        sound: AmitiaAlarmSoundDTO?,
        metadata: AmitiaAlarmMetadataDTO?
    ) async -> Bool {
        guard isAlarmKitAvailable() else { return false }
        #if canImport(AlarmKit)
        if #available(iOS 26.0, *) {
            return await createAlarmInternal(
                id: id,
                title: title,
                schedule: schedule,
                presentation: presentation,
                sound: sound,
                metadata: metadata
            )
        }
        #endif
        return false
    }

    public func cancelAlarm(id: String) async -> Bool {
        guard isAlarmKitAvailable() else { return false }
        #if canImport(AlarmKit)
        if #available(iOS 26.0, *) {
            do {
                let alarms = try await AlarmManager.shared.alarms
                guard let target = alarms.first(where: { $0.id == id }) else {
                    return false
                }
                try await AlarmManager.shared.remove(alarm: target)
                return true
            } catch {
                return false
            }
        }
        #endif
        return false
    }

    public func listAlarms() async -> [String: Any] {
        guard isAlarmKitAvailable() else { return ["alarms": []] }
        #if canImport(AlarmKit)
        if #available(iOS 26.0, *) {
            do {
                let alarms = try await AlarmManager.shared.alarms
                let result = alarms.map { alarm -> [String: Any] in
                    var item: [String: Any] = [
                        "id": alarm.id,
                        "title": alarm.title
                    ]
                    if let state = alarm.state {
                        item["state"] = String(describing: state)
                    }
                    return item
                }
                return ["alarms": result]
            } catch {
                return ["alarms": []]
            }
        }
        #endif
        return ["alarms": []]
    }

    public func getAlarm(id: String) async -> [String: Any]? {
        guard isAlarmKitAvailable() else { return nil }
        #if canImport(AlarmKit)
        if #available(iOS 26.0, *) {
            do {
                let alarms = try await AlarmManager.shared.alarms
                guard let target = alarms.first(where: { $0.id == id }) else {
                    return nil
                }
                var item: [String: Any] = [
                    "id": target.id,
                    "title": target.title
                ]
                if let state = target.state {
                    item["state"] = String(describing: state)
                }
                return item
            } catch {
                return nil
            }
        }
        #endif
        return nil
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
        sound: AmitiaAlarmSoundDTO?,
        metadata: AmitiaAlarmMetadataDTO?
    ) async -> Bool {
        do {
            let manager = AlarmManager.shared
            var alarm = Alarm(id: id, title: title)
            alarm.schedule = buildAlarmSchedule(schedule)
            alarm.presentation = buildAlarmPresentation(presentation)
            try await manager.add(alarm: alarm)
            return true
        } catch {
            return false
        }
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
