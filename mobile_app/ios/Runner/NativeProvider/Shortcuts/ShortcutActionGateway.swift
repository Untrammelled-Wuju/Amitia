import Foundation
import Intents

public class ShortcutActionGateway: NSObject {
    public static let shared = ShortcutActionGateway()

    private var actionRegistry: [String: () async -> [String: Any]] = [:]
    public private(set) var registeredActionIDs: [String] = []
    private var interactionCache: [String: INInteraction] = [:]

    private let curatedActionIDs: Set<String> = [
        "com.amitia.action.chat",
        "com.amitia.action.alarm.add",
        "com.amitia.action.alarm.remove",
        "com.amitia.action.reminder.add",
        "com.amitia.action.calendar.add",
        "com.amitia.action.media.pick",
        "com.amitia.action.media.export",
        "com.amitia.action.search",
        "com.amitia.action.settings",
        "com.amitia.action.status"
    ]

    private let actionTitles: [String: String] = [
        "com.amitia.action.chat": "Chat with Amitia",
        "com.amitia.action.alarm.add": "Add Alarm",
        "com.amitia.action.alarm.remove": "Remove Alarm",
        "com.amitia.action.reminder.add": "Add Reminder",
        "com.amitia.action.calendar.add": "Add Calendar Event",
        "com.amitia.action.media.pick": "Pick Media",
        "com.amitia.action.media.export": "Export Media",
        "com.amitia.action.search": "Search Memories",
        "com.amitia.action.settings": "Open Settings",
        "com.amitia.action.status": "Check Status"
    ]

    private override init() {
        super.init()
    }

    public func isCuratedAction(_ actionId: String) -> Bool {
        return curatedActionIDs.contains(actionId)
    }

    public var availableActions: [String] {
        return Array(curatedActionIDs)
    }

    public func titleForAction(_ actionId: String) -> String {
        return actionTitles[actionId] ?? actionId
    }

    public func registerAction(_ actionId: String, handler: @escaping () async -> [String: Any] = { return [:] }) {
        actionRegistry[actionId] = handler
        if !registeredActionIDs.contains(actionId) {
            registeredActionIDs.append(actionId)
        }
    }

    public func revokeAction(_ actionId: String) {
        actionRegistry.removeValue(forKey: actionId)
        registeredActionIDs.removeAll { $0 == actionId }
        if let interaction = interactionCache[actionId] {
            interaction.donate { error in
            }
            interactionCache.removeValue(forKey: actionId)
        }
    }

    public func donateIntent(actionId: String) async -> Bool {
        guard isCuratedAction(actionId) else { return false }
        guard #available(iOS 16.0, *) else { return false }

        let interaction: INInteraction
        switch actionId {
        case "com.amitia.action.chat":
            interaction = INInteraction(intent: AmitiaChatIntent(), response: nil)
        case "com.amitia.action.alarm.add":
            interaction = INInteraction(intent: AmitiaAlarmAddIntent(), response: nil)
        case "com.amitia.action.reminder.add":
            interaction = INInteraction(intent: AmitiaReminderAddIntent(), response: nil)
        case "com.amitia.action.media.pick":
            interaction = INInteraction(intent: AmitiaMediaPickIntent(), response: nil)
        default:
            return false
        }

        return await withCheckedContinuation { continuation in
            interaction.donate { error in
                if let error = error {
                    continuation.resume(returning: false)
                } else {
                    self.interactionCache[actionId] = interaction
                    continuation.resume(returning: true)
                }
            }
        }
    }

    public func executeAction(actionId: String, payload: [String: Any]?) async -> [String: Any] {
        guard let handler = actionRegistry[actionId] else {
            return ["error": "action not found: \(actionId)"]
        }
        return await handler()
    }

    public func updateShortcuts() {
        if #available(iOS 16.0, *) {
            for (_, interaction) in interactionCache {
                interaction.donate { _ in }
            }
        }
    }

    public func clearAllDonations() {
        if #available(iOS 16.0, *) {
            INInteraction.deleteAll()
            interactionCache.removeAll()
        }
    }
}
