import Foundation
import BackgroundTasks

public class BackgroundNativeHandler: NSObject, IOSNativeOperationHandler {
    public let operations: Set<String> = [
        "background.status",
        "background.task.register",
        "background.task.submit",
        "background.task.cancel",
        "background.task.cancel_all",
        "background.task.get_pending",
        "background.task.progress",
        "background.task.expire",
        "background.task.complete",
        "background.task.reconcile",
        "background.runtime.readiness",
        "background.runtime.ensure",
        "background.binding.get"
    ]

    private var activeTaskRunIds: Set<String> = []
    private let queue = DispatchQueue(label: "com.amitia.backgroundnative", attributes: .concurrent)

    public static func registerBGTaskHandlers() {
        if #available(iOS 13.0, *) {
            BGTaskScheduler.shared.register(forTaskWithIdentifier: "com.amitia.background.refresh", using: nil) { task in
                BackgroundTaskBridge.shared.handleBackgroundTask(task)
            }
            BGTaskScheduler.shared.register(forTaskWithIdentifier: "com.amitia.background.processing", using: nil) { task in
                BackgroundTaskBridge.shared.handleBackgroundTask(task)
            }
        }
    }

    public override init() {
        super.init()
        BackgroundTaskBridge.shared.delegate = self
    }

    public func capabilitySnapshot() -> IOSNativeCapability {
        if #available(iOS 13.0, *) {
            return IOSNativeCapability(
                available: true,
                authorized: true,
                hardwareAvailable: true,
                platformSupported: true,
                foregroundRequired: false
            )
        }
        return IOSNativeCapability(
            available: false,
            authorized: false,
            hardwareAvailable: false,
            platformSupported: false,
            foregroundRequired: false
        )
    }

    public func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        switch request.operation {
        case "background.status":
            return handleStatus(request)
        case "background.task.register":
            return handleTaskRegister(request)
        case "background.task.submit":
            return await handleTaskSubmit(request)
        case "background.task.cancel":
            return handleTaskCancel(request)
        case "background.task.cancel_all":
            return handleTaskCancelAll(request)
        case "background.task.get_pending":
            return await handleTaskGetPending(request)
        case "background.task.progress":
            return handleTaskProgress(request)
        case "background.task.expire":
            return handleTaskExpire(request)
        case "background.task.complete":
            return handleTaskComplete(request)
        case "background.task.reconcile":
            return handleTaskReconcile(request)
        case "background.runtime.readiness":
            return handleRuntimeReadiness(request)
        case "background.runtime.ensure":
            return handleRuntimeEnsure(request)
        case "background.binding.get":
            return handleBindingGet(request)
        default:
            return errorResponse(request, code: "OPERATION_NOT_SUPPORTED", message: "unsupported operation: \(request.operation)")
        }
    }

    private func handleStatus(_ request: IOSNativeRequest) -> IOSNativeResponse {
        var supported = false
        if #available(iOS 13.0, *) { supported = true }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: [
                "supported": supported,
                "pendingCount": activeTaskCount(),
                "activeTaskCount": activeTaskCount()
            ],
            error: nil
        )
    }

    private func handleTaskRegister(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let identifier = request.payload?["identifier"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing identifier")
        }
        let systemClass = request.payload?["systemClass"] as? String ?? "app_refresh"

        if #available(iOS 13.0, *) {
            let success = BGTaskScheduler.shared.register(forTaskWithIdentifier: identifier, using: nil) { task in
                BackgroundTaskBridge.shared.handleBackgroundTask(task)
            }
            if success {
                Task {
                    await BGTaskIdentifierRegistry.shared.register(
                        systemClass: systemClass,
                        identifier: identifier
                    )
                }
                return successResponse(request, result: [
                    "success": true,
                    "identifier": identifier,
                    "systemClass": systemClass
                ])
            }
            return errorResponse(request, code: "REGISTRATION_FAILED", message: "failed to register identifier")
        }
        return errorResponse(request, code: "PLATFORM_NOT_SUPPORTED", message: "iOS 13.0+ required")
    }

    private func handleTaskSubmit(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let taskRunId = request.payload?["taskRunId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing taskRunId")
        }
        let systemClass = request.payload?["systemClass"] as? String ?? "app_refresh"

        var identifier = request.payload?["identifier"] as? String
        if identifier == nil {
            identifier = await BGTaskIdentifierRegistry.shared.resolveIdentifier(systemClass: systemClass)
        }
        guard let resolvedIdentifier = identifier else {
            return errorResponse(request, code: "IDENTIFIER_NOT_FOUND", message: "no identifier registered for systemClass: \(systemClass)")
        }

        if #available(iOS 13.0, *) {
            let bgRequest: BGTaskRequest
            if systemClass == "processing" {
                let procRequest = BGProcessingTaskRequest(identifier: resolvedIdentifier)
                if let earliestBeginAt = parseDate(request.payload?["earliestBeginAt"]) {
                    procRequest.earliestBeginDate = earliestBeginAt
                }
                procRequest.requiresExternalPower = request.payload?["externalPowerRequired"] as? Bool ?? true
                procRequest.requiresNetworkConnectivity = request.payload?["networkRequired"] as? Bool ?? true
                bgRequest = procRequest
            } else {
                let refreshRequest = BGAppRefreshTaskRequest(identifier: resolvedIdentifier)
                if let earliestBeginAt = parseDate(request.payload?["earliestBeginAt"]) {
                    refreshRequest.earliestBeginDate = earliestBeginAt
                }
                bgRequest = refreshRequest
            }

            do {
                try BGTaskScheduler.shared.submit(bgRequest)
                await BGTaskIdentifierRegistry.shared.createMapping(
                    taskRunId: taskRunId,
                    systemClass: systemClass,
                    identifier: resolvedIdentifier
                )
                markTaskActive(taskRunId)
                return successResponse(request, result: [
                    "submitted": true,
                    "taskRunId": taskRunId,
                    "identifier": resolvedIdentifier,
                    "systemClass": systemClass
                ])
            } catch {
                return errorResponse(request, code: "SUBMIT_FAILED", message: error.localizedDescription)
            }
        }
        return errorResponse(request, code: "PLATFORM_NOT_SUPPORTED", message: "iOS 13.0+ required")
    }

    private func handleTaskCancel(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let taskRunId = request.payload?["taskRunId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing taskRunId")
        }
        Task {
            if let identifier = await BGTaskIdentifierRegistry.shared.identifier(forTaskRunId: taskRunId) {
                if #available(iOS 13.0, *) {
                    BGTaskScheduler.shared.cancel(taskRequestWithIdentifier: identifier)
                }
            }
            await BGTaskIdentifierRegistry.shared.removeMapping(taskRunId: taskRunId)
        }
        markTaskInactive(taskRunId)
        return successResponse(request, result: ["cancelled": true, "taskRunId": taskRunId])
    }

    private func handleTaskCancelAll(_ request: IOSNativeRequest) -> IOSNativeResponse {
        if #available(iOS 13.0, *) {
            BGTaskScheduler.shared.cancelAllTaskRequests()
        }
        clearAllActiveTasks()
        return successResponse(request, result: ["cancelled": true])
    }

    private func handleTaskGetPending(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        if #available(iOS 13.0, *) {
            let requests = await BGTaskScheduler.shared.pendingTaskRequests()
            let pending = requests.map { req -> [String: Any] in
                var info: [String: Any] = ["identifier": req.identifier]
                if let mapping = await BGTaskIdentifierRegistry.shared.mappingForIdentifier(req.identifier) {
                    info["taskRunId"] = mapping.taskRunId
                    info["systemClass"] = mapping.systemClass
                }
                return info
            }
            return successResponse(request, result: ["pending": pending])
        }
        return errorResponse(request, code: "PLATFORM_NOT_SUPPORTED", message: "iOS 13.0+ required")
    }

    private func handleTaskProgress(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let taskRunId = request.payload?["taskRunId"] as? String,
              let completedUnits = request.payload?["completedUnits"] as? Int,
              let totalUnits = request.payload?["totalUnits"] as? Int else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing required progress parameters")
        }
        let phase = request.payload?["phase"] as? String ?? ""
        return successResponse(request, result: [
            "taskRunId": taskRunId,
            "completedUnits": completedUnits,
            "totalUnits": totalUnits,
            "phase": phase,
            "recorded": true
        ])
    }

    private func handleTaskExpire(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let taskRunId = request.payload?["taskRunId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing taskRunId")
        }
        markTaskInactive(taskRunId)
        BackgroundTaskBridge.shared.markTaskRunExpired(taskRunId)
        return successResponse(request, result: [
            "expired": true,
            "taskRunId": taskRunId,
            "success": false
        ])
    }

    private func handleTaskComplete(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let taskRunId = request.payload?["taskRunId"] as? String,
              let success = request.payload?["success"] as? Bool else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing taskRunId or success flag")
        }
        markTaskInactive(taskRunId)
        BackgroundTaskBridge.shared.markTaskRunCompleted(taskRunId, success: success)
        return successResponse(request, result: [
            "completed": true,
            "taskRunId": taskRunId,
            "success": success
        ])
    }

    private func handleTaskReconcile(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let taskRunId = request.payload?["taskRunId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing taskRunId")
        }
        let stillPending = isTaskActive(taskRunId)
        return successResponse(request, result: [
            "taskRunId": taskRunId,
            "stillPending": stillPending
        ])
    }

    private func handleRuntimeReadiness(_ request: IOSNativeRequest) -> IOSNativeResponse {
        var available = false
        if #available(iOS 13.0, *) { available = true }
        let activeCount = activeTaskCount()
        return successResponse(request, result: [
            "ready": available,
            "activeTaskCount": activeCount,
            "canAcceptNewTask": available && activeCount < 4
        ])
    }

    private func handleRuntimeEnsure(_ request: IOSNativeRequest) -> IOSNativeResponse {
        var available = false
        if #available(iOS 13.0, *) { available = true }
        return successResponse(request, result: [
            "ensured": available,
            "platformSupported": available
        ])
    }

    private func handleBindingGet(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let taskRunId = request.payload?["taskRunId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing taskRunId")
        }
        let isActive = isTaskActive(taskRunId)
        var result: [String: Any] = [
            "taskRunId": taskRunId,
            "active": isActive
        ]
        Task {
            if let mapping = await BGTaskIdentifierRegistry.shared.mappingForTaskRun(taskRunId) {
                result["identifier"] = mapping.identifier
                result["systemClass"] = mapping.systemClass
            }
        }
        return successResponse(request, result: result)
    }

    private func parseDate(_ value: Any?) -> Date? {
        if let date = value as? Date {
            return date
        }
        if let timestamp = value as? Double {
            return Date(timeIntervalSince1970: timestamp)
        }
        if let timestamp = value as? Int {
            return Date(timeIntervalSince1970: TimeInterval(timestamp))
        }
        if let isoString = value as? String {
            let formatter = ISO8601DateFormatter()
            return formatter.date(from: isoString)
        }
        return nil
    }

    private func markTaskActive(_ taskRunId: String) {
        queue.async(flags: .barrier) {
            self.activeTaskRunIds.insert(taskRunId)
        }
    }

    private func markTaskInactive(_ taskRunId: String) {
        queue.async(flags: .barrier) {
            self.activeTaskRunIds.remove(taskRunId)
        }
    }

    private func clearAllActiveTasks() {
        queue.async(flags: .barrier) {
            self.activeTaskRunIds.removeAll()
        }
    }

    private func isTaskActive(_ taskRunId: String) -> Bool {
        return queue.sync { activeTaskRunIds.contains(taskRunId) }
    }

    private func activeTaskCount() -> Int {
        return queue.sync { activeTaskRunIds.count }
    }

    private func successResponse(_ request: IOSNativeRequest, result: [String: Any]) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: result,
            error: nil
        )
    }

    private func errorResponse(_ request: IOSNativeRequest, code: String, message: String) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "error",
            result: nil,
            error: IOSNativeError(code: code, message: message)
        )
    }
}

extension BackgroundNativeHandler: BackgroundTaskBridgeDelegate {
    public func backgroundTaskBridge(_ bridge: BackgroundTaskBridge, didReceiveTask task: BGTask) {
        Task {
            if let taskRunId = await BGTaskIdentifierRegistry.shared.taskRunId(forIdentifier: task.identifier) {
                markTaskActive(taskRunId)
            }
        }
    }

    public func backgroundTaskBridge(_ bridge: BackgroundTaskBridge, taskDidExpire task: BGTask) {
        Task {
            if let taskRunId = await BGTaskIdentifierRegistry.shared.taskRunId(forIdentifier: task.identifier) {
                markTaskInactive(taskRunId)
            }
        }
    }
}
