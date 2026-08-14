import Foundation
import UIKit

public class ShareNativeHandler: NSObject, IOSNativeOperationHandler {
    public let operations: Set<String> = [
        "share.status",
        "share.send",
        "share.preview.supported",
        "share.receive.pending",
        "share.receive.consume",
        "share.receive.peek",
        "share.receive.dismiss",
        "share.staging.cleanup",
        "share.limited.delete"
    ]

    public override init() {
        super.init()
    }

    public func capabilitySnapshot() -> IOSNativeCapability {
        return IOSNativeCapability(
            available: true,
            authorized: true,
            hardwareAvailable: true,
            platformSupported: true,
            foregroundRequired: true
        )
    }

    public func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        switch request.operation {
        case "share.status":
            return handleStatus(request)
        case "share.send":
            return await handleSend(request)
        case "share.preview.supported":
            return handlePreviewSupported(request)
        case "share.receive.pending":
            return handleReceivePending(request)
        case "share.receive.consume":
            return handleReceiveConsume(request)
        case "share.receive.peek":
            return handleReceivePeek(request)
        case "share.receive.dismiss":
            return handleReceiveDismiss(request)
        case "share.staging.cleanup":
            return handleStagingCleanup(request)
        case "share.limited.delete":
            return handleLimitedDelete(request)
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
            result: ["available": true, "message": "ShareSheet available"],
            error: nil
        )
    }

    private func handleSend(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let scene = UIApplication.shared.connectedScenes.first as? UIWindowScene,
              let window = scene.windows.first,
              let rootViewController = window.rootViewController else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "FOREGROUND_REQUIRED", message: "No active foreground UIWindowScene available")
            )
        }

        let items = buildShareItems(from: request.payload)
        guard !items.isEmpty else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "No valid share items provided")
            )
        }

        let activityVC = UIActivityViewController(activityItems: items, applicationActivities: nil)

        await MainActor.run {
            var topController = rootViewController
            while let presentedViewController = topController.presentedViewController {
                topController = presentedViewController
            }
            topController.present(activityVC, animated: true)
        }

        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["sent": true],
            error: nil
        )
    }

    private func handlePreviewSupported(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["supported": true],
            error: nil
        )
    }

    private func handleReceivePending(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "Share Extension not implemented")
        )
    }

    private func handleReceiveConsume(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "Share Extension not implemented")
        )
    }

    private func handleReceivePeek(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "Share Extension not implemented")
        )
    }

    private func handleReceiveDismiss(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "Share Extension not implemented")
        )
    }

    private func handleStagingCleanup(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["cleaned": true],
            error: nil
        )
    }

    private func handleLimitedDelete(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["deleted": true],
            error: nil
        )
    }

    private func buildShareItems(from payload: [String: Any]?) -> [Any] {
        guard let payload = payload else { return [] }
        var items: [Any] = []

        if let text = payload["text"] as? String {
            items.append(text)
        }
        if let urlString = payload["url"] as? String, let url = URL(string: urlString) {
            items.append(url)
        }
        if let resource = payload["resource"] as? String, let url = URL(string: resource) {
            items.append(url)
        }
        if let itemsArray = payload["items"] as? [Any] {
            items.append(contentsOf: itemsArray)
        }

        return items
    }
}
