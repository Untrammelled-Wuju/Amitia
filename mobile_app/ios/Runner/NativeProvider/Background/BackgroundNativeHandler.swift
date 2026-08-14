import Foundation
import BackgroundTasks

public class BackgroundNativeHandler: NSObject, IOSNativeOperationHandler {
    public let operations: Set<String> = [
        "background.status",
        "background.task.register",
        "background.task.cancel",
        "background.task.cancel_all",
        "background.refresh.schedule",
        "background.refresh.cancel",
        "background.processing.schedule",
        "background.processing.cancel",
        "background.url_session.complete",
        "background.url_session.events"
    ]

    public override init() {
        super.init()
    }

    public func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        switch request.operation {
        case "background.status":
            return handleStatus(request)
        case "background.task.register":
            return handleTaskRegister(request)
        case "background.task.cancel":
            return handleTaskCancel(request)
        case "background.task.cancel_all":
            return handleTaskCancelAll(request)
        case "background.refresh.schedule":
            return handleRefreshSchedule(request)
        case "background.refresh.cancel":
            return handleRefreshCancel(request)
        case "background.processing.schedule":
            return handleProcessingSchedule(request)
        case "background.processing.cancel":
            return handleProcessingCancel(request)
        case "background.url_session.complete":
            return handleURLSessionComplete(request)
        case "background.url_session.events":
            return handleURLSessionEvents(request)
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
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["available": true, "authorized": true, "message": "Background tasks available"],
            error: nil
        )
    }

    private func handleTaskRegister(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["registered": true],
            error: nil
        )
    }

    private func handleTaskCancel(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["cancelled": true],
            error: nil
        )
    }

    private func handleTaskCancelAll(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["cancelled": true],
            error: nil
        )
    }

    private func handleRefreshSchedule(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["scheduled": true],
            error: nil
        )
    }

    private func handleRefreshCancel(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["cancelled": true],
            error: nil
        )
    }

    private func handleProcessingSchedule(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["scheduled": true],
            error: nil
        )
    }

    private func handleProcessingCancel(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["cancelled": true],
            error: nil
        )
    }

    private func handleURLSessionComplete(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["completed": true],
            error: nil
        )
    }

    private func handleURLSessionEvents(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["events": []],
            error: nil
        )
    }
}
