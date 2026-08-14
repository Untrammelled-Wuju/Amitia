import Foundation
import AppIntents

@available(iOS 16.0, *)
public struct AmitiaAppIntents: AppIntent {
    public static var title: LocalizedStringResource = "Amitia Intent"
    public static var openAppWhenRun: Bool = false

    public init() {}

    public func perform() async throws -> some IntentResult {
        return .result()
    }
}
