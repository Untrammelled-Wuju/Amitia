import Foundation
import AppIntents

@available(iOS 16.0, *)
public struct AmitiaChatIntent: AppIntent {
    public static var title: LocalizedStringResource = "Chat with Amitia"
    public static var openAppWhenRun: Bool = false

    public init() {}

    public func perform() async throws -> some IntentResult {
        let result = await ShortcutActionGateway.shared.executeUnchecked(actionId: "com.amitia.action.chat", payload: nil)
        if let error = result["error"] as? String {
            throw NSError(domain: "AmitiaAppIntents", code: 1, userInfo: [NSLocalizedDescriptionKey: error])
        }
        return .result()
    }
}

@available(iOS 16.0, *)
public struct AmitiaAlarmAddIntent: AppIntent {
    public static var title: LocalizedStringResource = "Add Alarm"
    public static var openAppWhenRun: Bool = false

    public init() {}

    public func perform() async throws -> some IntentResult {
        let result = await ShortcutActionGateway.shared.executeUnchecked(actionId: "com.amitia.action.alarm.add", payload: nil)
        if let error = result["error"] as? String {
            throw NSError(domain: "AmitiaAppIntents", code: 1, userInfo: [NSLocalizedDescriptionKey: error])
        }
        return .result()
    }
}

@available(iOS 16.0, *)
public struct AmitiaReminderAddIntent: AppIntent {
    public static var title: LocalizedStringResource = "Add Reminder"
    public static var openAppWhenRun: Bool = false

    public init() {}

    public func perform() async throws -> some IntentResult {
        let result = await ShortcutActionGateway.shared.executeUnchecked(actionId: "com.amitia.action.reminder.add", payload: nil)
        if let error = result["error"] as? String {
            throw NSError(domain: "AmitiaAppIntents", code: 1, userInfo: [NSLocalizedDescriptionKey: error])
        }
        return .result()
    }
}

@available(iOS 16.0, *)
public struct AmitiaMediaPickIntent: AppIntent {
    public static var title: LocalizedStringResource = "Pick Media"
    public static var openAppWhenRun: Bool = false

    public init() {}

    public func perform() async throws -> some IntentResult {
        let result = await ShortcutActionGateway.shared.executeUnchecked(actionId: "com.amitia.action.media.pick", payload: nil)
        if let error = result["error"] as? String {
            throw NSError(domain: "AmitiaAppIntents", code: 1, userInfo: [NSLocalizedDescriptionKey: error])
        }
        return .result()
    }
}
