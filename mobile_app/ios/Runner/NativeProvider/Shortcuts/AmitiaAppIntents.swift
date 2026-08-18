import Foundation
import AppIntents

@available(iOS 16.0, *)
private enum AmitiaIntentResultValidator {
    static func requireSuccess(_ result: [String: Any]) throws {
        let failed = (result["status"] as? String)?.lowercased() == "error" || result["error"] != nil
        guard failed else { return }

        var code = "BACKEND_ACTION_FAILED"
        var message = "Backend action failed"
        if let error = result["error"] as? [String: Any] {
            if let value = error["code"] as? String, !value.isEmpty { code = value }
            if let value = error["message"] as? String, !value.isEmpty { message = value }
        } else if let error = result["error"] as? String, !error.isEmpty {
            code = error
            message = error
        }
        throw NSError(
            domain: "AmitiaAppIntents",
            code: 1,
            userInfo: [
                NSLocalizedDescriptionKey: message,
                "AmitiaErrorCode": code
            ]
        )
    }
}

@available(iOS 16.0, *)
public struct AmitiaChatIntent: AppIntent {
    public static var title: LocalizedStringResource = "Chat with Amitia"
    public static var openAppWhenRun: Bool = false

    public init() {}

    public func perform() async throws -> some IntentResult {
        let result = await ShortcutActionGateway.shared.executeUnchecked(
            actionId: "com.amitia.action.chat",
            payload: nil
        )
        try AmitiaIntentResultValidator.requireSuccess(result)
        return .result()
    }
}

@available(iOS 16.0, *)
public struct AmitiaAlarmAddIntent: AppIntent {
    public static var title: LocalizedStringResource = "Add Alarm"
    public static var openAppWhenRun: Bool = false

    @Parameter(title: "Title")
    public var alarmTitle: String

    @Parameter(title: "Time")
    public var fireAt: Date

    public init() {}

    public func perform() async throws -> some IntentResult {
        let formatter = ISO8601DateFormatter()
        let payload: [String: Any] = [
            "kind": "alarm",
            "title": alarmTitle,
            "schedule": ["fireAt": formatter.string(from: fireAt)],
            "presentation": ["alertTitle": alarmTitle]
        ]
        let result = await ShortcutActionGateway.shared.executeUnchecked(
            actionId: "com.amitia.action.alarm.add",
            payload: payload
        )
        try AmitiaIntentResultValidator.requireSuccess(result)
        return .result()
    }
}

@available(iOS 16.0, *)
public struct AmitiaReminderAddIntent: AppIntent {
    public static var title: LocalizedStringResource = "Add Reminder"
    public static var openAppWhenRun: Bool = false

    @Parameter(title: "Title")
    public var reminderTitle: String

    public init() {}

    public func perform() async throws -> some IntentResult {
        let result = await ShortcutActionGateway.shared.executeUnchecked(
            actionId: "com.amitia.action.reminder.add",
            payload: ["title": reminderTitle]
        )
        try AmitiaIntentResultValidator.requireSuccess(result)
        return .result()
    }
}

@available(iOS 16.0, *)
public struct AmitiaMediaPickIntent: AppIntent {
    public static var title: LocalizedStringResource = "Pick Media"
    public static var openAppWhenRun: Bool = false

    public init() {}

    public func perform() async throws -> some IntentResult {
        let result = await ShortcutActionGateway.shared.executeUnchecked(
            actionId: "com.amitia.action.media.pick",
            payload: nil
        )
        try AmitiaIntentResultValidator.requireSuccess(result)
        return .result()
    }
}
