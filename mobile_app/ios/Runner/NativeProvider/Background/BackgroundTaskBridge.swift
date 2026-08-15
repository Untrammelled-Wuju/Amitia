import Foundation
import BackgroundTasks

public protocol BackgroundTaskBridgeDelegate: AnyObject {
    func backgroundTaskBridge(_ bridge: BackgroundTaskBridge, didReceiveTask task: BGTask)
    func backgroundTaskBridge(_ bridge: BackgroundTaskBridge, taskDidExpire task: BGTask)
}

public class BackgroundTaskBridge: NSObject {
    public static let shared = BackgroundTaskBridge()

    public weak var delegate: BackgroundTaskBridgeDelegate?

    private var taskHandlers: [String: (BGTask) -> Void] = [:]
    private var pendingCompletions: [String: BGTask] = [:]
    private let queue = DispatchQueue(label: "com.amitia.backgroundtaskbridge", attributes: .concurrent)

    private override init() {
        super.init()
    }

    public func registerHandler(for identifier: String, handler: @escaping (BGTask) -> Void) {
        queue.async(flags: .barrier) {
            self.taskHandlers[identifier] = handler
        }
    }

    public func handleBackgroundTask(_ task: BGTask) {
        let identifier = task.identifier

        queue.async(flags: .barrier) {
            self.pendingCompletions[identifier] = task
        }

        task.expirationHandler = { [weak self, weak task] in
            guard let self = self, let task = task else { return }
            self.handleExpiration(task: task)
        }

        queue.sync {
            if let handler = self.taskHandlers[identifier] {
                handler(task)
            } else {
                self.delegate?.backgroundTaskBridge(self, didReceiveTask: task)
            }
        }
    }

    private func handleExpiration(task: BGTask) {
        let identifier = task.identifier

        delegate?.backgroundTaskBridge(self, taskDidExpire: task)

        queue.async(flags: .barrier) {
            self.pendingCompletions.removeValue(forKey: identifier)
        }
    }

    public func completeTask(_ task: BGTask, success: Bool) {
        task.setTaskCompleted(success: success)
        queue.async(flags: .barrier) {
            self.pendingCompletions.removeValue(forKey: task.identifier)
        }
    }

    public func markTaskCompleted(_ identifier: String, success: Bool) {
        queue.sync(flags: .barrier) {
            if let task = self.pendingCompletions.removeValue(forKey: identifier) {
                task.setTaskCompleted(success: success)
            }
        }
    }

    public func hasPendingTask(_ identifier: String) -> Bool {
        return queue.sync { pendingCompletions[identifier] != nil }
    }
}
