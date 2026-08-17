import Foundation

public protocol BackendActionDispatcher: AnyObject {
    func executeAction(actionId: String, payload: [String: Any]?) async -> [String: Any]
}

public enum ShortcutAction: String, Sendable {
    case openConversation
    case sendMessage
    case createReminder
    case startTask
    case addAlarm
    case removeAlarm
    case addCalendarEvent
    case pickMedia
    case exportMedia
    case search
    case settings
    case status

    public var actionId: String {
        switch self {
        case .openConversation: return "com.amitia.action.chat"
        case .sendMessage: return "com.amitia.action.send_message"
        case .createReminder: return "com.amitia.action.reminder.add"
        case .startTask: return "com.amitia.action.task.start"
        case .addAlarm: return "com.amitia.action.alarm.add"
        case .removeAlarm: return "com.amitia.action.alarm.remove"
        case .addCalendarEvent: return "com.amitia.action.calendar.add"
        case .pickMedia: return "com.amitia.action.media.pick"
        case .exportMedia: return "com.amitia.action.media.export"
        case .search: return "com.amitia.action.search"
        case .settings: return "com.amitia.action.settings"
        case .status: return "com.amitia.action.status"
        }
    }

    public var title: String {
        switch self {
        case .openConversation: return "Chat with Amitia"
        case .sendMessage: return "Send Message"
        case .createReminder: return "Add Reminder"
        case .startTask: return "Start Task"
        case .addAlarm: return "Add Alarm"
        case .removeAlarm: return "Remove Alarm"
        case .addCalendarEvent: return "Add Calendar Event"
        case .pickMedia: return "Pick Media"
        case .exportMedia: return "Export Media"
        case .search: return "Search Memories"
        case .settings: return "Open Settings"
        case .status: return "Check Status"
        }
    }

    public static func from(actionId: String) -> ShortcutAction? {
        switch actionId {
        case "com.amitia.action.chat": return .openConversation
        case "com.amitia.action.send_message": return .sendMessage
        case "com.amitia.action.reminder.add": return .createReminder
        case "com.amitia.action.task.start": return .startTask
        case "com.amitia.action.alarm.add": return .addAlarm
        case "com.amitia.action.alarm.remove": return .removeAlarm
        case "com.amitia.action.calendar.add": return .addCalendarEvent
        case "com.amitia.action.media.pick": return .pickMedia
        case "com.amitia.action.media.export": return .exportMedia
        case "com.amitia.action.search": return .search
        case "com.amitia.action.settings": return .settings
        case "com.amitia.action.status": return .status
        default: return nil
        }
    }
}

public class ShortcutActionGateway: NSObject {
    public static let shared = ShortcutActionGateway()

    public private(set) var registeredActions: [ShortcutAction] = []
    public weak var backendDispatcher: BackendActionDispatcher? {
        didSet {
            _dispatcherReady = (backendDispatcher != nil)
        }
    }
    private var _dispatcherReady: Bool = false

    public var isDispatcherReady: Bool {
        return _dispatcherReady && backendDispatcher != nil
    }

    private override init() {
        super.init()
        registeredActions = [
            .openConversation,
            .createReminder,
            .addAlarm,
            .addCalendarEvent,
            .pickMedia,
            .search,
            .settings,
            .status
        ]
    }

    public func setupBackendDispatcher(_ dispatcher: BackendActionDispatcher) {
        backendDispatcher = dispatcher
        _dispatcherReady = true
    }

    public func isCuratedAction(_ action: ShortcutAction) -> Bool {
        return registeredActions.contains(action)
    }

    public var availableActions: [ShortcutAction] {
        return registeredActions
    }

    public func titleForAction(_ action: ShortcutAction) -> String {
        return action.title
    }

    public func executeAction(_ action: ShortcutAction, payload: [String: Any]?) async -> [String: Any] {
        guard isCuratedAction(action) else {
            return ["error": "ACTION_NOT_AVAILABLE", "actionId": action.actionId]
        }

        guard isDispatcherReady, let dispatcher = backendDispatcher else {
            return ["error": "BACKEND_DISPATCHER_NOT_READY", "actionId": action.actionId]
        }

        return await dispatcher.executeAction(actionId: action.actionId, payload: payload)
    }

    public func executeUnchecked(actionId: String, payload: [String: Any]?) async -> [String: Any] {
        guard let action = ShortcutAction.from(actionId: actionId) else {
            return ["error": "ACTION_NOT_AVAILABLE", "actionId": actionId]
        }
        return await executeAction(action, payload: payload)
    }
}
