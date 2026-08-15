import Foundation
import EventKit

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

public struct AlarmSchedule: Codable {
    var fireAt: String?
    var hour: Int?
    var minute: Int?
    var recurrence: String?
    var weekdays: [String]?
}

public struct AlarmPresentation: Codable {
    var alertTitle: String?
    var countdownTitle: String?
    var pausedTitle: String?
    var tintColor: String?
    var secondaryAction: String?
}

public struct AlarmSound: Codable {
    var kind: String?
    var soundId: String?
}

public struct AlarmMetadata: Codable {
    var kind: String?
    var icon: String?
    var ownerRef: String?
}

public class AlarmKitAdapter {
    public static let shared = AlarmKitAdapter()
    private let eventStore = EKEventStore()
    private var alarmCalendar: EKCalendar?

    private init() {}

    public func checkAvailability() -> AlarmKitAvailability {
        if #available(iOS 26.0, *) {
            return AlarmKitManager.shared.isAlarmKitSupported ? .available : .unsupported("AlarmKit not available on this device")
        } else if #available(iOS 10.0, *) {
            return .unsupported("system alarm API requires iOS 26.0+")
        } else {
            return .unsupported("iOS 10.0+ required for alarm functionality")
        }
    }

    public func authorizationStatus() -> AlarmAuthorizationStatus {
        if #available(iOS 26.0, *) {
            return AlarmKitManager.shared.authorizationStatus()
        }
        return .notDetermined
    }

    public func requestAuthorization() async -> Bool {
        if #available(iOS 26.0, *) {
            return await AlarmKitManager.shared.requestAuthorization()
        }
        return false
    }

    private func getOrCreateAlarmCalendar() -> EKCalendar? {
        if let calendar = alarmCalendar { return calendar }

        let calendars = eventStore.calendars(for: .event)
        if let existing = calendars.first(where: { $0.title == "Amitia" && $0.allowsContentModifications }) {
            alarmCalendar = existing
            return existing
        }

        let newCalendar = EKCalendar(for: .event, eventStore: eventStore)
        newCalendar.title = "Amitia"
        if let source = eventStore.sources.first(where: { $0.sourceType == .local }) ?? eventStore.defaultCalendarForNewEvents?.source {
            newCalendar.source = source
        } else {
            return nil
        }

        do {
            try eventStore.saveCalendar(newCalendar, commit: true)
            alarmCalendar = newCalendar
            return newCalendar
        } catch {
            return nil
        }
    }

    public func createAlarm(
        id: String,
        title: String,
        schedule: AlarmSchedule,
        presentation: AlarmPresentation,
        sound: AlarmSound?,
        metadata: AlarmMetadata?
    ) async -> Bool {
        let reminderResult = await createReminder(
            id: id,
            title: title,
            schedule: schedule,
            metadata: metadata
        )
        return reminderResult
    }

    public func cancelAlarm(id: String) async -> Bool {
        let reminders = await newWaitContinuation { continuation in
            let predicate = eventStore.predicateForReminders(in: nil)
            eventStore.fetchReminders(matching: predicate) { reminders in
                continuation.resume(returning: reminders ?? [])
            }
        }

        guard let target = reminders.first(where: { $0.calendarItemIdentifier == id }) else {
            return false
        }

        do {
            try eventStore.remove(target, commit: true)
            return true
        } catch {
            return false
        }
    }

    public func listAlarms() async -> [String: Any] {
        let reminders = await newWaitContinuation { continuation in
            let predicate = eventStore.predicateForReminders(in: nil)
            eventStore.fetchReminders(matching: predicate) { reminders in
                continuation.resume(returning: reminders ?? [])
            }
        }

        let alarms = reminders.map { reminder -> [String: Any] in
            var alarm: [String: Any] = [
                "id": reminder.calendarItemIdentifier,
                "title": reminder.title ?? ""
            ]
            if let dueDate = reminder.dueDateComponents?.date {
                alarm["dueDate"] = ISO8601DateFormatter().string(from: dueDate)
            }
            alarm["completed"] = reminder.isCompleted
            alarm["calendarItemIdentifier"] = reminder.calendarItemIdentifier
            return alarm
        }

        return ["alarms": alarms]
    }

    public func getAlarm(id: String) async -> [String: Any]? {
        let reminders = await newWaitContinuation { continuation in
            let predicate = eventStore.predicateForReminders(in: nil)
            eventStore.fetchReminders(matching: predicate) { reminders in
                continuation.resume(returning: reminders ?? [])
            }
        }

        guard let reminder = reminders.first(where: { $0.calendarItemIdentifier == id }) else {
            return nil
        }

        var alarm: [String: Any] = [
            "id": reminder.calendarItemIdentifier,
            "title": reminder.title ?? ""
        ]
        if let dueDate = reminder.dueDateComponents?.date {
            alarm["dueDate"] = ISO8601DateFormatter().string(from: dueDate)
        }
        alarm["completed"] = reminder.isCompleted
        return alarm
    }

    private func createReminder(
        id: String,
        title: String,
        schedule: AlarmSchedule,
        metadata: AlarmMetadata?
    ) async -> Bool {
        let authStatus = eventStore.authorizationStatus(for: .reminder)
        if authStatus != .authorized && authStatus != .fullAccess {
            return false
        }

        let reminder = EKReminder(eventStore: eventStore)
        reminder.title = title
        reminder.calendar = eventStore.defaultCalendarForNewReminders()
        reminder.calendarItemIdentifier = id

        if let fireAt = schedule.fireAt, let date = ISO8601DateFormatter().date(from: fireAt) {
            let dc = Calendar.current.dateComponents([.year, .month, .day, .hour, .minute, .second], from: date)
            reminder.dueDateComponents = dc
        } else if let hour = schedule.hour, let minute = schedule.minute {
            var dc = DateComponents()
            dc.hour = hour
            dc.minute = minute
            reminder.dueDateComponents = dc
            if let recurrence = schedule.recurrence, recurrence == "daily" {
                reminder.recurrenceRules = [EKRecurrenceRule(recurrenceWith: .daily, interval: 1, end: nil)]
            }
        }

        do {
            try eventStore.save(reminder, commit: true)
            return true
        } catch {
            return false
        }
    }

    private func newWaitContinuation<T>(_ operation: @escaping (@escaping (T) -> Void) -> Void) async -> T {
        return await withCheckedContinuation { continuation in
            operation { result in
                continuation.resume(returning: result)
            }
        }
    }
}

@available(iOS 26.0, *)
private class AlarmKitManager {
    static let shared = AlarmKitManager()

    func isAlarmKitSupported() -> Bool {
        return NSClassFromString("AlarmKit.AlarmManager") != nil
    }

    func authorizationStatus() -> AlarmAuthorizationStatus {
        return .authorized
    }

    func requestAuthorization() async -> Bool {
        return true
    }
}
