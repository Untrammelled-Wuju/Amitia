import Foundation
import AppIntents

@available(iOS 16.0, *)
public struct AmitiaAppShortcutsProvider: AppShortcutsProvider {
    public static var appShortcuts: [AppShortcut] {
        return [
            AppShortcut(
                intent: AmitiaAppIntents(actionId: "com.amitia.action.default"),
                phrases: ["Start an action with \(.applicationName)"],
                shortTitle: "Start Action",
                systemImageName: "play.circle"
            )
        ]
    }

    public init() {}
}
