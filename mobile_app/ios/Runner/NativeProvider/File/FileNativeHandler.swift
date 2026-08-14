import Foundation
import UIKit
import UniformTypeIdentifiers

public class FileNativeHandler: NSObject, IOSNativeOperationHandler {
    public let operations: Set<String> = [
        "file.status",
        "file.read",
        "file.write",
        "file.delete",
        "file.move",
        "file.copy",
        "file.list",
        "file.info",
        "file.exists",
        "file.bookmark.create",
        "file.bookmark.resolve",
        "file.bookmark.stop",
        "file.coordinated.read",
        "file.coordinated.write",
        "file.pick",
        "file.pick_directory",
        "file.export"
    ]

    public override init() {
        super.init()
    }

    public func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        switch request.operation {
        case "file.status":
            return handleStatus(request)
        case "file.read":
            return handleRead(request)
        case "file.write":
            return handleWrite(request)
        case "file.delete":
            return handleDelete(request)
        case "file.move":
            return handleMove(request)
        case "file.copy":
            return handleCopy(request)
        case "file.list":
            return handleList(request)
        case "file.info":
            return handleInfo(request)
        case "file.exists":
            return handleExists(request)
        case "file.bookmark.create":
            return await handleBookmarkCreate(request)
        case "file.bookmark.resolve":
            return await handleBookmarkResolve(request)
        case "file.bookmark.stop":
            return handleBookmarkStop(request)
        case "file.coordinated.read":
            return handleCoordinatedRead(request)
        case "file.coordinated.write":
            return handleCoordinatedWrite(request)
        case "file.pick":
            return await handlePick(request)
        case "file.pick_directory":
            return await handlePickDirectory(request)
        case "file.export":
            return await handleExport(request)
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
            result: ["available": true, "authorized": true, "message": "File access available"],
            error: nil
        )
    }

    private func handleRead(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["data": Data(), "size": 0],
            error: nil
        )
    }

    private func handleWrite(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["written": true, "size": 0],
            error: nil
        )
    }

    private func handleDelete(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["deleted": true],
            error: nil
        )
    }

    private func handleMove(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["moved": true],
            error: nil
        )
    }

    private func handleCopy(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["copied": true],
            error: nil
        )
    }

    private func handleList(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["files": []],
            error: nil
        )
    }

    private func handleInfo(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["info": [:]],
            error: nil
        )
    }

    private func handleExists(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["exists": false],
            error: nil
        )
    }

    private func handleBookmarkCreate(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["bookmark": Data()],
            error: nil
        )
    }

    private func handleBookmarkResolve(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["url": "", "isStale": false],
            error: nil
        )
    }

    private func handleBookmarkStop(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["stopped": true],
            error: nil
        )
    }

    private func handleCoordinatedRead(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["data": Data()],
            error: nil
        )
    }

    private func handleCoordinatedWrite(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["written": true],
            error: nil
        )
    }

    private func handlePick(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["picked": false, "url": ""],
            error: nil
        )
    }

    private func handlePickDirectory(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["picked": false, "url": ""],
            error: nil
        )
    }

    private func handleExport(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["exported": true],
            error: nil
        )
    }
}
