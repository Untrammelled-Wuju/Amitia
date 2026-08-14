import Foundation
import Intents
import AppIntents

public class ShortcutNativeHandler: NSObject, IOSNativeOperationHandler {
    public let operations: Set<String> = [
        "shortcuts.status",
        "shortcuts.list",
        "shortcuts.get",
        "shortcuts.donate",
        "shortcuts.delete",
        "shortcuts.perform",
        "shortcuts.siri_phrases"
    ]

    public override init() {
        super.init()
    }

    public func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        switch request.operation {
        case "shortcuts.status":
            return handleStatus(request)
        case "shortcuts.list":
            return handleList(request)
        case "shortcuts.get":
            return handleGet(request)
        case "shortcuts.donate":
            return await handleDonate(request)
        case "shortcuts.delete":
            return await handleDelete(request)
        case "shortcuts.perform":
            return await handlePerform(request)
        case "shortcuts.siri_phrases":
            return handleSiriPhrases(request)
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
            result: ["available": true, "authorized": true, "message": "Shortcuts available"],
            error: nil
        )
    }

    private func handleList(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["shortcuts": []],
            error: nil
        )
    }

    private func handleGet(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["shortcut": [:]],
            error: nil
        )
    }

    private func handleDonate(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["donated": true],
            error: nil
        )
    }

    private func handleDelete(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["deleted": true],
            error: nil
        )
    }

    private func handlePerform(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["performed": true],
            error: nil
        )
    }

    private func handleSiriPhrases(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["phrases": []],
            error: nil
        )
    }
}
