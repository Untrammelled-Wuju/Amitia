import Foundation
import AppIntents

@available(iOS 16.0, *)
public struct AmitiaAppShortcutsProvider: AppShortcutsProvider {
    public static var appShortcuts: [AppShortcut] {
        return []
    }

    public init() {}
}
