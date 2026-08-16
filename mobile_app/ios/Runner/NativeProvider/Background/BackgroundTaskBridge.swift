import Foundation
import BackgroundTasks

public protocol BackgroundTaskBridgeDelegate: AnyObject {
    func backgroundTaskBridge(_ bridge: BackgroundTaskBridge, didReceiveTask task: BGTask)
    func backgroundTaskBridge(_ bridge: BackgroundTaskBridge, taskDidExpire task: BGTask)
}

public typealias BackgroundEventEmitter = @Sendable ([String: Any]) -> Void

public class BackgroundTaskBridge: NSObject {
    public static let shared = BackgroundTaskBridge()

    public weak var delegate: BackgroundTaskBridgeDelegate?
    public var eventEmitter: BackgroundEventEmitter?

    private var taskHandlers: [String: (BGTask) -> Void] = [:]
    private var pendingTasks: [String: BGTask] = [:]
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
            self.pendingTasks[identifier] = task
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

        Task {
            if let taskRunId = await BGTaskIdentifierRegistry.shared.taskRunId(forIdentifier: identifier) {
                let event: [String: Any] = [
                    "domain": "background",
                    "event": "execution_window_started",
                    "timestamp": ISO8601DateFormatter().string(from: Date()),
                    "data": [
                        "taskRunId": taskRunId,
                        "identifier": identifier
                    ]
                ]
                await MainActor.run {
                    self.eventEmitter?(event)
                }
            }
        }
    }

    private func handleExpiration(task: BGTask) {
        let identifier = task.identifier

        delegate?.backgroundTaskBridge(self, taskDidExpire: task)

        Task {
            if let taskRunId = await BGTaskIdentifierRegistry.shared.taskRunId(forIdentifier: identifier) {
                let event: [String: Any] = [
                    "domain": "background",
                    "event": "execution_window_expired",
                    "timestamp": ISO8601DateFormatter().string(from: Date()),
                    "data": [
                        "taskRunId": taskRunId,
                        "identifier": identifier
                    ]
                ]
                await MainActor.run {
                    self.eventEmitter?(event)
                }
            }
        }

        queue.async(flags: .barrier) {
            self.pendingTasks.removeValue(forKey: identifier)
        }
    }

    public func completeTask(_ task: BGTask, success: Bool) {
        task.setTaskCompleted(success: success)
        queue.async(flags: .barrier) {
            self.pendingTasks.removeValue(forKey: task.identifier)
        }
    }

    public func markTaskRunCompleted(_ taskRunId: String, success: Bool) {
        Task {
            if let identifier = await BGTaskIdentifierRegistry.shared.identifier(forTaskRunId: taskRunId) {
                queue.sync(flags: .barrier) {
                    if let task = self.pendingTasks.removeValue(forKey: identifier) {
                        task.setTaskCompleted(success: success)
                    }
                }
            }
            await BGTaskIdentifierRegistry.shared.removeMapping(taskRunId: taskRunId)
        }
    }

    public func markTaskRunExpired(_ taskRunId: String) {
        Task {
            if let identifier = await BGTaskIdentifierRegistry.shared.identifier(forTaskRunId: taskRunId) {
                queue.sync(flags: .barrier) {
                    if let task = self.pendingTasks.removeValue(forKey: identifier) {
                        task.setTaskCompleted(success: false)
                    }
                }
            }
            await BGTaskIdentifierRegistry.shared.removeMapping(taskRunId: taskRunId)
        }
    }

    public func hasPendingTaskRun(_ taskRunId: String) async -> Bool {
        var result = false
        if let identifier = await BGTaskIdentifierRegistry.shared.identifier(forTaskRunId: taskRunId) {
            result = queue.sync(execute: { self.pendingTasks[identifier] != nil })
        }
        return result
    }

    public func taskRunIdForPendingTask(_ identifier: String) async -> String? {
        if queue.sync(execute: { self.pendingTasks[identifier] != nil }) {
            return await BGTaskIdentifierRegistry.shared.taskRunId(forIdentifier: identifier)
        }
        return nil
    }
}
