import Foundation
import UIKit

public class ClipboardNativeHandler: NSObject, IOSNativeOperationHandler {
    public let operations: Set<String> = [
        "clipboard.read",
        "clipboard.write",
        "clipboard.clear",
        "clipboard.has_content",
        "clipboard.types"
    ]

    public override init() {
        super.init()
    }

    public func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        switch request.operation {
        case "clipboard.read":
            return handleRead(request)
        case "clipboard.write":
            return handleWrite(request)
        case "clipboard.clear":
            return handleClear(request)
        case "clipboard.has_content":
            return handleHasContent(request)
        case "clipboard.types":
            return handleTypes(request)
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

    private func handleRead(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let clipboard = UIPasteboard.general
        let text = clipboard.string ?? ""
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["text": text, "type": "text"],
            error: nil
        )
    }

    private func handleWrite(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let clipboard = UIPasteboard.general
        clipboard.string = ""
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

    private func handleHasContent(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let clipboard = UIPasteboard.general
        let hasContent = clipboard.hasStrings || clipboard.hasURLs || clipboard.hasImages
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["hasContent": hasContent],
            error: nil
        )
    }

    private func handleTypes(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let clipboard = UIPasteboard.general
        var types: [String] = []
        if clipboard.hasStrings { types.append("text") }
        if clipboard.hasURLs { types.append("url") }
        if clipboard.hasImages { types.append("image") }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["types": types],
            error: nil
        )
    }
}
