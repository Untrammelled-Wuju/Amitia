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
        "background.checkpoint.get",
        "background.checkpoint.set",
        "background.checkpoint.clear",
        "background.binding.get"
    ]

    public override init() {
        super.init()
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
        case "background.checkpoint.get":
            return handleCheckpointGet(request)
        case "background.checkpoint.set":
            return handleCheckpointSet(request)
        case "background.checkpoint.clear":
            return handleCheckpointClear(request)
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
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["available": available, "message": "BGTaskScheduler available"],
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
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: ["registered": success, "identifier": identifier],
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
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "task.progress handled by Backend TaskRuntime")
        )
    }

    private func handleTaskExpire(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["expired": true],
            error: nil
        )
    }

    private func handleTaskComplete(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "task.complete handled by Backend TaskRuntime")
        )
    }

    private func handleTaskReconcile(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "task.reconcile handled by Backend TaskRuntime")
        )
    }

    private func handleRuntimeReadiness(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let available = #available(iOS 13.0, *)
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["ready": available],
            error: nil
        )
    }

    private func handleRuntimeEnsure(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let available = #available(iOS 13.0, *)
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["ensured": available],
            error: nil
        )
    }

    private func handleCheckpointGet(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "checkpoint.get handled by Backend")
        )
    }

    private func handleCheckpointSet(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "checkpoint.set handled by Backend")
        )
    }

    private func handleCheckpointClear(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "checkpoint.clear handled by Backend")
        )
    }

    private func handleBindingGet(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "binding.get handled by Backend")
        )
    }
}
