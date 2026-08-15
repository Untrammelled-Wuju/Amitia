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

    private var activeBGTaskIdentifiers: Set<String> = []
    private let queue = DispatchQueue(label: "com.amitia.backgroundnative", attributes: .concurrent)

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
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "OPERATION_NOT_SUPPORTED", message: "unsupported operation: \(request.operation)")
            )
        }
    }

    private func handleStatus(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let available = #available(iOS 13.0, *)
        let pendingCount = pendingTaskCount()
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: [
                "available": available,
                "message": "BGTaskScheduler available",
                "pendingCount": pendingCount,
                "activeTaskCount": activeTaskCount()
            ],
            error: nil
        )
    }

    private func handleTaskRegister(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let identifier = request.payload?["identifier"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing identifier")
            )
        }

        if #available(iOS 13.0, *) {
            let success = BGTaskScheduler.shared.register(forTaskWithIdentifier: identifier, using: nil) { task in
                BackgroundTaskBridge.shared.handleBackgroundTask(task)
            }
            if success {
                markTaskActive(identifier)
                return IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestID: request.requestID,
                    status: "ok",
                    result: ["registered": success, "identifier": identifier],
                    error: nil
                )
            } else {
                return IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestID: request.requestID,
                    status: "error",
                    result: nil,
                    error: IOSNativeError(code: "REGISTRATION_FAILED", message: "failed to register background task: \(identifier)")
                )
            }
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "iOS 13.0+ required")
        )
    }

    private func handleTaskSubmit(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let identifier = request.payload?["identifier"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing identifier")
            )
        }

        if #available(iOS 13.0, *) {
            let taskType = request.payload?["taskType"] as? String ?? "refresh"

            let request_bg: BGTaskRequest
            if taskType == "processing" {
                let bgRequest = BGProcessingTaskRequest(identifier: identifier)
                bgRequest.earliestBeginDate = request.payload?["earliestBeginDate"] as? Date
                if let requiresExternalPower = request.payload?["requiresExternalPower"] as? Bool {
                    bgRequest.requiresExternalPower = requiresExternalPower
                }
                if let requiresNetworkConnectivity = request.payload?["requiresNetworkConnectivity"] as? Bool {
                    bgRequest.requiresNetworkConnectivity = requiresNetworkConnectivity
                }
                request_bg = bgRequest
            } else {
                let bgRequest = BGAppRefreshTaskRequest(identifier: identifier)
                bgRequest.earliestBeginDate = request.payload?["earliestBeginDate"] as? Date
                request_bg = bgRequest
            }

            do {
                try BGTaskScheduler.shared.submit(request_bg)
                return IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestID: request.requestID,
                    status: "ok",
                    result: ["submitted": true, "identifier": identifier, "taskType": taskType],
                    error: nil
                )
            } catch {
                return IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestID: request.requestID,
                    status: "error",
                    result: nil,
                    error: IOSNativeError(code: "SUBMIT_FAILED", message: error.localizedDescription)
                )
            }
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "iOS 13.0+ required")
        )
    }

    private func handleTaskCancel(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let identifier = request.payload?["identifier"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing identifier")
            )
        }

        if #available(iOS 13.0, *) {
            BGTaskScheduler.shared.cancel(taskRequestWithIdentifier: identifier)
            markTaskInactive(identifier)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: ["cancelled": true, "identifier": identifier],
                error: nil
            )
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "iOS 13.0+ required")
        )
    }

    private func handleTaskCancelAll(_ request: IOSNativeRequest) -> IOSNativeResponse {
        if #available(iOS 13.0, *) {
            BGTaskScheduler.shared.cancelAllTaskRequests()
            clearAllActiveTasks()
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: ["cancelled": true],
                error: nil
            )
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "iOS 13.0+ required")
        )
    }

    private func handleTaskGetPending(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        if #available(iOS 13.0, *) {
            let requests = await BGTaskScheduler.shared.pendingTaskRequests()
            let pending = requests.map { req in
                ["identifier": req.identifier, "taskType": String(describing: type(of: req))]
            }
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: ["pending": pending],
                error: nil
            )
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "iOS 13.0+ required")
        )
    }

    private func handleTaskProgress(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let identifier = request.payload?["identifier"] as? String,
              let completedUnits = request.payload?["completedUnits"] as? Int,
              let totalUnits = request.payload?["totalUnits"] as? Int else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing required progress parameters")
            )
        }
        let phase = request.payload?["phase"] as? String ?? ""
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: [
                "identifier": identifier,
                "completedUnits": completedUnits,
                "totalUnits": totalUnits,
                "phase": phase,
                "recorded": true
            ],
            error: nil
        )
    }

    private func handleTaskExpire(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let identifier = request.payload?["identifier"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing identifier")
            )
        }
        markTaskInactive(identifier)
        BackgroundTaskBridge.shared.markTaskCompleted(identifier, success: false)
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["expired": true, "identifier": identifier, "success": false],
            error: nil
        )
    }

    private func handleTaskComplete(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let identifier = request.payload?["identifier"] as? String,
              let success = request.payload?["success"] as? Bool else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing identifier or success flag")
            )
        }
        markTaskInactive(identifier)
        BackgroundTaskBridge.shared.markTaskCompleted(identifier, success: success)
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["completed": true, "identifier": identifier, "success": success],
            error: nil
        )
    }

    private func handleTaskReconcile(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let identifier = request.payload?["identifier"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing identifier")
            )
        }
        let stillPending = BackgroundTaskBridge.shared.hasPendingTask(identifier)
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["identifier": identifier, "stillPending": stillPending],
            error: nil
        )
    }

    private func handleRuntimeReadiness(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let available = #available(iOS 13.0, *)
        let activeCount = activeTaskCount()
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: [
                "ready": available,
                "activeTaskCount": activeCount,
                "canAcceptNewTask": available && activeCount < 4
            ],
            error: nil
        )
    }

    private func handleRuntimeEnsure(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let available = #available(iOS 13.0, *)
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: [
                "ensured": available,
                "platformSupported": available
            ],
            error: nil
        )
    }

    private func handleBindingGet(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let identifier = request.payload?["identifier"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing identifier")
            )
        }
        let isActive = isTaskActive(identifier)
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["identifier": identifier, "active": isActive],
            error: nil
        )
    }

    private func markTaskActive(_ identifier: String) {
        queue.async(flags: .barrier) {
            self.activeBGTaskIdentifiers.insert(identifier)
        }
    }

    private func markTaskInactive(_ identifier: String) {
        queue.async(flags: .barrier) {
            self.activeBGTaskIdentifiers.remove(identifier)
        }
    }

    private func clearAllActiveTasks() {
        queue.async(flags: .barrier) {
            self.activeBGTaskIdentifiers.removeAll()
        }
    }

    private func isTaskActive(_ identifier: String) -> Bool {
        return queue.sync { activeBGTaskIdentifiers.contains(identifier) }
    }

    private func activeTaskCount() -> Int {
        return queue.sync { activeBGTaskIdentifiers.count }
    }

    private func pendingTaskCount() -> Int {
        if #available(iOS 13.0, *) {
            return BGTaskScheduler.shared.pendingTaskRequests().count
        }
        return 0
    }
}

extension BackgroundNativeHandler: BackgroundTaskBridgeDelegate {
    public func backgroundTaskBridge(_ bridge: BackgroundTaskBridge, didReceiveTask task: BGTask) {
        markTaskActive(task.identifier)
    }

    public func backgroundTaskBridge(_ bridge: BackgroundTaskBridge, taskDidExpire task: BGTask) {
        markTaskInactive(task.identifier)
    }
}
