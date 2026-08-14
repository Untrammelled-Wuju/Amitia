import Foundation
import BackgroundTasks

public class BackgroundTaskBridge: NSObject {
    public static let shared = BackgroundTaskBridge()

    private var taskHandlers: [String: (BGTask) -> Void] = [:]

    private override init() {
        super.init()
    }

    public func registerHandler(for identifier: String, handler: @escaping (BGTask) -> Void) {
        taskHandlers[identifier] = handler
    }

    public func handleBackgroundTask(_ task: BGTask) {
        guard let identifier = task.identifier as String?,
              let handler = taskHandlers[identifier] else {
            task.setTaskCompleted(success: false)
            return
        }

        let expirationHandler = { [weak task] in
            task?.setTaskCompleted(success: false)
        }
        task.expirationHandler = expirationHandler

        handler(task)
    }

    public func completeTask(_ task: BGTask, success: Bool) {
        task.setTaskCompleted(success: success)
    }
}
