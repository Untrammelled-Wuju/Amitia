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
                await self.resumeTaskRuntime(taskRunId: taskRunId, identifier: identifier)
            }
        }
    }

    private func resumeTaskRuntime(taskRunId: String, identifier: String) async {
        await withCheckedContinuation { (continuation: CheckedContinuation<Void, Never>) in
            let resumeEvent: [String: Any] = [
                "domain": "background",
                "event": "task_runtime_resume",
                "timestamp": ISO8601DateFormatter().string(from: Date()),
                "data": [
                    "taskRunId": taskRunId,
                    "identifier": identifier
                ]
            ]
            Task { @MainActor in
                self.eventEmitter?(resumeEvent)
                continuation.resume()
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
                await self.expireTaskRuntime(taskRunId: taskRunId, identifier: identifier)
            }
        }

        queue.async(flags: .barrier) {
            self.pendingTasks.removeValue(forKey: identifier)
        }
    }

    private func expireTaskRuntime(taskRunId: String, identifier: String) async {
        await withCheckedContinuation { (continuation: CheckedContinuation<Void, Never>) in
            let expireEvent: [String: Any] = [
                "domain": "background",
                "event": "task_runtime_expire",
                "timestamp": ISO8601DateFormatter().string(from: Date()),
                "data": [
                    "taskRunId": taskRunId,
                    "identifier": identifier
                ]
            ]
            Task { @MainActor in
                self.eventEmitter?(expireEvent)
                continuation.resume()
            }
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
            await self.persistTaskRunCompletion(taskRunId: taskRunId, success: success)
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

    private func persistTaskRunCompletion(taskRunId: String, success: Bool) async {
        await withCheckedContinuation { (continuation: CheckedContinuation<Void, Never>) in
            let completeEvent: [String: Any] = [
                "domain": "background",
                "event": "task_runtime_complete",
                "timestamp": ISO8601DateFormatter().string(from: Date()),
                "data": [
                    "taskRunId": taskRunId,
                    "success": success
                ]
            ]
            Task { @MainActor in
                self.eventEmitter?(completeEvent)
                continuation.resume()
            }
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

    public func updateTaskRunProgress(
        _ taskRunId: String,
        totalUnits: Int64,
        completedUnits: Int64,
        phase: String
    ) async -> Bool {
        guard totalUnits > 0,
              let identifier = await BGTaskIdentifierRegistry.shared.identifier(
                forTaskRunId: taskRunId
              ) else {
            return false
        }
        guard #available(iOS 26.0, *) else {
            return false
        }
        guard let task = queue.sync(execute: {
            self.pendingTasks[identifier] as? BGContinuedProcessingTask
        }) else {
            return false
        }

        let clamped = min(max(completedUnits, 0), totalUnits)
        task.progress.totalUnitCount = totalUnits
        task.progress.completedUnitCount = clamped
        let normalizedPhase = phase.trimmingCharacters(in: .whitespacesAndNewlines)
        if !normalizedPhase.isEmpty {
            await MainActor.run {
                task.updateTitle(task.title, subtitle: normalizedPhase)
            }
        }
        return true
    }

    public func cancelTaskRun(_ taskRunId: String) async -> Bool {
        guard let identifier = await BGTaskIdentifierRegistry.shared.identifier(
            forTaskRunId: taskRunId
        ) else {
            return false
        }
        let task = queue.sync(flags: .barrier) {
            self.pendingTasks.removeValue(forKey: identifier)
        }
        if let task {
            task.setTaskCompleted(success: false)
            return true
        }
        return false
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
