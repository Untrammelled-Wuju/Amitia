import Foundation
import UIKit
import UniformTypeIdentifiers

public class FileNativeHandler: NSObject, IOSNativeOperationHandler {
    public let operations: Set<String> = [
        "file.pick.import",
        "file.pick.directory",
        "file.mount.reauthorize",
        "file.access.stat",
        "file.access.list",
        "file.access.read",
        "file.access.write",
        "file.access.mkdir",
        "file.access.rename",
        "file.access.move",
        "file.access.copy",
        "file.access.delete",
        "file.export",
        "file.mount.get",
        "file.mount.list",
        "file.mount.remove",
        "file.get_capabilities"
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
        case "file.pick.import":
            return await handlePickImport(request)
        case "file.pick.directory":
            return await handlePickDirectory(request)
        case "file.mount.reauthorize":
            return await handleMountReauthorize(request)
        case "file.access.stat":
            return handleAccessStat(request)
        case "file.access.list":
            return handleAccessList(request)
        case "file.access.read":
            return await handleAccessRead(request)
        case "file.access.write":
            return await handleAccessWrite(request)
        case "file.access.mkdir":
            return handleAccessMkdir(request)
        case "file.access.rename":
            return handleAccessRename(request)
        case "file.access.move":
            return handleAccessMove(request)
        case "file.access.copy":
            return handleAccessCopy(request)
        case "file.access.delete":
            return handleAccessDelete(request)
        case "file.export":
            return await handleExport(request)
        case "file.mount.get":
            return handleMountGet(request)
        case "file.mount.list":
            return handleMountList(request)
        case "file.mount.remove":
            return handleMountRemove(request)
        case "file.get_capabilities":
            return handleGetCapabilities(request)
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

    private func handlePickImport(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let scene = UIApplication.shared.connectedScenes.first as? UIWindowScene,
              let window = scene.windows.first,
              let rootViewController = window.rootViewController else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "FOREGROUND_REQUIRED", message: "No active foreground UIWindowScene")
            )
        }

        return await withCheckedContinuation { continuation in
            let documentPicker = UIDocumentPickerViewController(forOpeningContentTypes: [.data])
            documentPicker.allowsMultipleSelection = false

            let delegate = DocumentPickerDelegate { result in
                switch result {
                case .success(let grantId):
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestID: request.requestID,
                        status: "ok",
                        result: ["grantId": grantId, "picked": true],
                        error: nil
                    ))
                case .failure(let error):
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestID: request.requestID,
                        status: "error",
                        result: nil,
                        error: IOSNativeError(code: "PICK_FAILED", message: error)
                    ))
                }
            }

            documentPicker.delegate = delegate
            objc_setAssociatedObject(documentPicker, &delegateKey, delegate, .OBJC_ASSOCIATION_RETAIN_NONATOMIC)

            rootViewController.present(documentPicker, animated: true)
        }
    }

    private func handlePickDirectory(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let scene = UIApplication.shared.connectedScenes.first as? UIWindowScene,
              let window = scene.windows.first,
              let rootViewController = window.rootViewController else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "FOREGROUND_REQUIRED", message: "No active foreground UIWindowScene")
            )
        }

        if #available(iOS 14.0, *) {
            return await withCheckedContinuation { continuation in
                let documentPicker = UIDocumentPickerViewController(forOpeningContentTypes: [.folder])

                let delegate = DocumentPickerDelegate { result in
                    switch result {
                    case .success(let grantId):
                        continuation.resume(returning: IOSNativeResponse(
                            protocolVersion: request.protocolVersion,
                            requestID: request.requestID,
                            status: "ok",
                            result: ["grantId": grantId, "picked": true],
                            error: nil
                        ))
                    case .failure(let error):
                        continuation.resume(returning: IOSNativeResponse(
                            protocolVersion: request.protocolVersion,
                            requestID: request.requestID,
                            status: "error",
                            result: nil,
                            error: IOSNativeError(code: "PICK_FAILED", message: error)
                        ))
                    }
                }

                documentPicker.delegate = delegate
                objc_setAssociatedObject(documentPicker, &delegateKey, delegate, .OBJC_ASSOCIATION_RETAIN_NONATOMIC)

                rootViewController.present(documentPicker, animated: true)
            }
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "iOS 14.0+ required")
        )
    }

    private func handleMountReauthorize(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let grantId = request.payload?["grantId"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing grantId")
            )
        }
        let reauthorized = SecurityScopedBookmarkStore.shared.reauthorize(grantId: grantId)
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["reauthorized": reauthorized, "grantId": grantId],
            error: nil
        )
    }

    private func handleAccessStat(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let grantId = request.payload?["grantId"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing grantId")
            )
        }
        guard let attributes = SecurityScopedBookmarkStore.shared.stat(grantId: grantId) else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "NOT_FOUND", message: "file not found for grant: \(grantId)")
            )
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["stat": attributes],
            error: nil
        )
    }

    private func handleAccessList(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let grantId = request.payload?["grantId"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing grantId")
            )
        }
        guard let items = SecurityScopedBookmarkStore.shared.list(grantId: grantId) else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "NOT_FOUND", message: "directory not found for grant: \(grantId)")
            )
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["items": items],
            error: nil
        )
    }

    private func handleAccessRead(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let grantId = request.payload?["grantId"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing grantId")
            )
        }
        guard let data = SecurityScopedBookmarkStore.shared.read(grantId: grantId) else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "READ_FAILED", message: "failed to read file for grant: \(grantId)")
            )
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["size": data.count, "grantId": grantId],
            error: nil
        )
    }

    private func handleAccessWrite(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let grantId = request.payload?["grantId"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing grantId")
            )
        }
        let data = request.payload?["data"] as? Data ?? Data()
        let success = SecurityScopedBookmarkStore.shared.write(grantId: grantId, data: data)
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["written": success, "grantId": grantId],
            error: nil
        )
    }

    private func handleAccessMkdir(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let grantId = request.payload?["grantId"] as? String,
              let name = request.payload?["name"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing grantId or name")
            )
        }
        let newGrantId = SecurityScopedBookmarkStore.shared.mkdir(parentGrantId: grantId, name: name)
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["grantId": newGrantId ?? ""],
            error: nil
        )
    }

    private func handleAccessRename(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let grantId = request.payload?["grantId"] as? String,
              let newName = request.payload?["newName"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing grantId or newName")
            )
        }
        let success = SecurityScopedBookmarkStore.shared.rename(grantId: grantId, newName: newName)
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["renamed": success],
            error: nil
        )
    }

    private func handleAccessMove(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let sourceGrantId = request.payload?["sourceGrantId"] as? String,
              let destGrantId = request.payload?["destGrantId"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing sourceGrantId or destGrantId")
            )
        }
        let success = SecurityScopedBookmarkStore.shared.move(sourceGrantId: sourceGrantId, destGrantId: destGrantId)
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["moved": success],
            error: nil
        )
    }

    private func handleAccessCopy(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let sourceGrantId = request.payload?["sourceGrantId"] as? String,
              let destGrantId = request.payload?["destGrantId"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing sourceGrantId or destGrantId")
            )
        }
        let newGrantId = SecurityScopedBookmarkStore.shared.copy(sourceGrantId: sourceGrantId, destGrantId: destGrantId)
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["grantId": newGrantId ?? ""],
            error: nil
        )
    }

    private func handleAccessDelete(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let grantId = request.payload?["grantId"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing grantId")
            )
        }
        let success = SecurityScopedBookmarkStore.shared.delete(grantId: grantId)
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["deleted": success],
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

    private func handleMountGet(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let mountId = request.payload?["mountId"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing mountId")
            )
        }
        let info = SecurityScopedBookmarkStore.shared.getMount(mountId: mountId)
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["mount": info ?? [:]],
            error: nil
        )
    }

    private func handleMountList(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let mounts = SecurityScopedBookmarkStore.shared.listMounts()
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["mounts": mounts],
            error: nil
        )
    }

    private func handleMountRemove(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let mountId = request.payload?["mountId"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing mountId")
            )
        }
        let success = SecurityScopedBookmarkStore.shared.removeMount(mountId: mountId)
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["removed": success],
            error: nil
        )
    }

    private func handleGetCapabilities(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: [
                "canPickFiles": true,
                "canPickDirectories": true,
                "canRead": true,
                "canWrite": true,
                "canDelete": true,
                "canExport": true,
                "canMount": true
            ],
            error: nil
        )
    }
}

private var delegateKey: UInt8 = 0

private class DocumentPickerDelegate: NSObject, UIDocumentPickerDelegate {
    private let completion: (Result<String, String>) -> Void

    init(completion: @escaping (Result<String, String>) -> Void) {
        self.completion = completion
        super.init()
    }

    func documentPicker(_ controller: UIDocumentPickerViewController, didPickDocumentsAt urls: [URL]) {
        guard let url = urls.first else {
            completion(.failure("no file selected"))
            return
        }
        do {
            let grantId = try SecurityScopedBookmarkStore.shared.createBookmark(for: url)
            completion(.success(grantId))
        } catch {
            completion(.failure(error.localizedDescription))
        }
    }

    func documentPickerWasCancelled(_ controller: UIDocumentPickerViewController) {
        completion(.failure("user cancelled"))
    }
}
