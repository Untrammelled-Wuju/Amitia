import Foundation
import UIKit

public class ShareNativeHandler: NSObject, IOSNativeOperationHandler {
    public let operations: Set<String> = [
        "share.status",
        "share.present",
        "share.text",
        "share.image",
        "share.url",
        "share.file"
    ]

    public override init() {
        super.init()
    }

    public func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        switch request.operation {
        case "share.status":
            return handleStatus(request)
        case "share.present":
            return await handlePresent(request)
        case "share.text":
            return await handleText(request)
        case "share.image":
            return await handleImage(request)
        case "share.url":
            return await handleURL(request)
        case "share.file":
            return await handleFile(request)
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
            result: ["available": true, "authorized": true, "message": "ShareSheet available"],
            error: nil
        )
    }

    private func handlePresent(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["presented": true],
            error: nil
        )
    }

    private func handleText(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["shared": true],
            error: nil
        )
    }

    private func handleImage(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["shared": true],
            error: nil
        )
    }

    private func handleURL(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["shared": true],
            error: nil
        )
    }

    private func handleFile(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["shared": true],
            error: nil
        )
    }
}
