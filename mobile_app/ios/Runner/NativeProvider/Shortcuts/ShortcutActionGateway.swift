import Foundation
import AppIntents

public class ShortcutActionGateway: NSObject {
    public static let shared = ShortcutActionGateway()

    private var actionRegistry: [String: () async -> [String: Any]] = [:]
    public private(set) var registeredActionIDs: [String] = []

    private override init() {
        super.init()
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
    }

    public func donateIntent(actionId: String) async -> Bool {
        guard actionRegistry[actionId] != nil else { return false }
        return true
    }

    public func executeAction(actionId: String, payload: [String: Any]?) async -> [String: Any] {
        guard let handler = actionRegistry[actionId] else {
            return ["error": "action not found: \(actionId)"]
        }
        return await handler()
    }

    public func updateShortcuts() {
        if #available(iOS 16.0, *) {
            INInteraction.deleteAll()
        }
    }
}
