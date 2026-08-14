import Foundation
import AppIntents

@available(iOS 16.0, *)
public struct AmitiaAppIntents: AppIntent {
    public static var title: LocalizedStringResource = "Amitia Intent"
    public static var openAppWhenRun: Bool = false

    @Parameter(title: "Action ID")
    public var actionId: String

    public init() {
        self.actionId = ""
    }

    public init(actionId: String) {
        self.actionId = actionId
    }

    public func perform() async throws -> some IntentResult {
        let result = await ShortcutActionGateway.shared.executeAction(actionId: actionId, payload: nil)
        if let error = result["error"] as? String {
            throw NSError(domain: "AmitiaAppIntents", code: 1, userInfo: [NSLocalizedDescriptionKey: error])
        }
        return .result()
    }
}
