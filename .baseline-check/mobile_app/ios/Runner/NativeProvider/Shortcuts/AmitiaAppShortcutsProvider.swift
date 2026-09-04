import Foundation
import AppIntents

@available(iOS 16.0, *)
public struct AmitiaAppShortcutsProvider: AppShortcutsProvider {
    public static var appShortcuts: [AppShortcut] {
        return [
            AppShortcut(
                intent: AmitiaChatIntent(),
                phrases: ["Start a chat with \(.applicationName)"],
                shortTitle: "Chat with Amitia",
                systemImageName: "message.circle"
            ),
            AppShortcut(
                intent: AmitiaAlarmAddIntent(),
                phrases: ["Add an alarm with \(.applicationName)"],
                shortTitle: "Add Alarm",
                systemImageName: "alarm"
            ),
            AppShortcut(
                intent: AmitiaReminderAddIntent(),
                phrases: ["Add a reminder with \(.applicationName)"],
                shortTitle: "Add Reminder",
                systemImageName: "bell"
            ),
            AppShortcut(
                intent: AmitiaMediaPickIntent(),
                phrases: ["Pick media with \(.applicationName)"],
                shortTitle: "Pick Media",
                systemImageName: "photo.on.rectangle"
            )
        ]
    }

    public init() {}
}
