import Foundation
import UIKit

public class ClipboardNativeHandler: NSObject, IOSNativeOperationHandler {
    public let operations: Set<String> = [
        "clipboard.status",
        "clipboard.detect",
        "clipboard.read",
        "clipboard.write",
        "clipboard.clear"
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
            foregroundRequired: false
        )
    }

    public func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        switch request.operation {
        case "clipboard.status":
            return handleStatus(request)
        case "clipboard.detect":
            return handleDetect(request)
        case "clipboard.read":
            return handleRead(request)
        case "clipboard.write":
            return handleWrite(request)
        case "clipboard.clear":
            return handleClear(request)
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
        let clipboard = UIPasteboard.general
        let hasContent = clipboard.hasStrings || clipboard.hasURLs || clipboard.hasImages
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["available": true, "hasContent": hasContent],
            error: nil
        )
    }

    private func handleDetect(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let clipboard = UIPasteboard.general
        var types: [String] = []
        if clipboard.hasStrings { types.append("text") }
        if clipboard.hasURLs { types.append("url") }
        if clipboard.hasImages { types.append("image") }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["types": types, "hasContent": !types.isEmpty],
            error: nil
        )
    }

    private func handleRead(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let clipboard = UIPasteboard.general
        if let text = clipboard.string {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: ["text": text, "type": "text"],
                error: nil
            )
        }
        if let url = clipboard.url {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: ["url": url.absoluteString, "type": "url"],
                error: nil
            )
        }
        if let image = clipboard.image {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: ["type": "image", "hasImage": image != nil],
                error: nil
            )
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["empty": true],
            error: nil
        )
    }

    private func handleWrite(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let clipboard = UIPasteboard.general
        var written = false

        if let text = request.payload?["text"] as? String {
            clipboard.string = text
            written = true
        } else if let url = request.payload?["url"] as? String {
            clipboard.url = URL(string: url)
            written = true
        }

        if !written {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing text or url in payload")
            )
        }

        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["written": true],
            error: nil
        )
    }

    private func handleClear(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let clipboard = UIPasteboard.general
        clipboard.items = []
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["cleared": true],
            error: nil
        )
    }
}
