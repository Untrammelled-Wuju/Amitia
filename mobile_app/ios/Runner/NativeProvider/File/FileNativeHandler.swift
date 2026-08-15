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
                case .success(let mountId):
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestID: request.requestID,
                        status: "ok",
                        result: ["mountId": mountId, "picked": true],
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
                    case .success(let mountId):
                        continuation.resume(returning: IOSNativeResponse(
                            protocolVersion: request.protocolVersion,
                            requestID: request.requestID,
                            status: "ok",
                            result: ["mountId": mountId, "picked": true],
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
        guard let mountId = request.payload?["mountId"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing mountId")
            )
        }
        let reauthorized = SecurityScopedBookmarkStore.shared.reauthorize(mountId: mountId)
        if !reauthorized {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "REAUTHORIZE_FAILED", message: "failed to reauthorize mount: \(mountId)")
            )
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["reauthorized": reauthorized, "mountId": mountId],
            error: nil
        )
    }

    private func handleAccessStat(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let mountId = request.payload?["mountId"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing mountId")
            )
        }
        guard let attributes = SecurityScopedBookmarkStore.shared.stat(mountId: mountId) else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "NOT_FOUND", message: "file not found for mount: \(mountId)")
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
        guard let mountId = request.payload?["mountId"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing mountId")
            )
        }
        guard let items = SecurityScopedBookmarkStore.shared.list(mountId: mountId) else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "NOT_FOUND", message: "directory not found for mount: \(mountId)")
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
        guard let mountId = request.payload?["mountId"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing mountId")
            )
        }
        let offset = Int64(request.payload?["offset"] as? Int ?? 0)
        let length = Int64(request.payload?["length"] as? Int ?? 0)
        guard let data = SecurityScopedBookmarkStore.shared.read(mountId: mountId, offset: offset, length: length) else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "READ_FAILED", message: "failed to read file for mount: \(mountId)")
            )
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["size": data.count, "mountId": mountId],
            error: nil
        )
    }

    private func handleAccessWrite(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let mountId = request.payload?["mountId"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing mountId")
            )
        }
        let data = request.payload?["data"] as? Data ?? Data()
        let offset = Int64(request.payload?["offset"] as? Int ?? 0)
        let success = SecurityScopedBookmarkStore.shared.write(mountId: mountId, data: data, offset: offset)
        if !success {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "WRITE_FAILED", message: "failed to write file for mount: \(mountId)")
            )
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["written": success, "mountId": mountId],
            error: nil
        )
    }

    private func handleAccessMkdir(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let mountId = request.payload?["mountId"] as? String,
              let name = request.payload?["name"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing mountId or name")
            )
        }
        guard let newMountId = SecurityScopedBookmarkStore.shared.mkdir(parentMountId: mountId, name: name) else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "MKDIR_FAILED", message: "failed to create directory")
            )
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["mountId": newMountId],
            error: nil
        )
    }

    private func handleAccessRename(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let mountId = request.payload?["mountId"] as? String,
              let newName = request.payload?["newName"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing mountId or newName")
            )
        }
        let success = SecurityScopedBookmarkStore.shared.rename(mountId: mountId, newName: newName)
        if !success {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "RENAME_FAILED", message: "failed to rename")
            )
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["renamed": success],
            error: nil
        )
    }

    private func handleAccessMove(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let sourceMountId = request.payload?["sourceMountId"] as? String,
              let destMountId = request.payload?["destMountId"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing sourceMountId or destMountId")
            )
        }
        let success = SecurityScopedBookmarkStore.shared.move(sourceMountId: sourceMountId, destMountId: destMountId)
        if !success {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "MOVE_FAILED", message: "failed to move")
            )
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["moved": success],
            error: nil
        )
    }

    private func handleAccessCopy(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let sourceMountId = request.payload?["sourceMountId"] as? String,
              let destMountId = request.payload?["destMountId"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing sourceMountId or destMountId")
            )
        }
        guard let newMountId = SecurityScopedBookmarkStore.shared.copy(sourceMountId: sourceMountId, destMountId: destMountId) else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "COPY_FAILED", message: "failed to copy")
            )
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["mountId": newMountId],
            error: nil
        )
    }

    private func handleAccessDelete(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let mountId = request.payload?["mountId"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing mountId")
            )
        }
        let success = SecurityScopedBookmarkStore.shared.delete(mountId: mountId)
        if !success {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "DELETE_FAILED", message: "failed to delete")
            )
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["deleted": success],
            error: nil
        )
    }

    private func handleExport(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let mountId = request.payload?["mountId"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing mountId")
            )
        }
        guard let url = SecurityScopedBookmarkStore.shared.resolve(identifier: mountId) else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "NOT_FOUND", message: "mount not found: \(mountId)")
            )
        }

        guard let scene = UIApplication.shared.connectedScenes.first as? UIWindowScene,
              let window = scene.windows.first,
              let rootViewController = window.rootViewController else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "FOREGROUND_REQUIRED", message: "No active foreground UIWindowScene for export")
            )
        }

        return await withCheckedContinuation { continuation in
            let documentPicker = UIDocumentPickerViewController(forExporting: [url], asCopy: true)
            documentPicker.allowsMultipleSelection = false

            let delegate = ExportPickerDelegate { result in
                switch result {
                case .success:
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestID: request.requestID,
                        status: "ok",
                        result: ["exported": true, "mountId": mountId],
                        error: nil
                    ))
                case .failure(let error):
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestID: request.requestID,
                        status: "error",
                        result: nil,
                        error: IOSNativeError(code: "EXPORT_FAILED", message: error)
                    ))
                }
            }

            documentPicker.delegate = delegate
            objc_setAssociatedObject(documentPicker, &exportDelegateKey, delegate, .OBJC_ASSOCIATION_RETAIN_NONATOMIC)

            rootViewController.present(documentPicker, animated: true)
        }
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
private var exportDelegateKey: UInt8 = 0

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
            let mountId = try SecurityScopedBookmarkStore.shared.createBookmark(for: url)
            completion(.success(mountId))
        } catch {
            completion(.failure(error.localizedDescription))
        }
    }

    func documentPickerWasCancelled(_ controller: UIDocumentPickerViewController) {
        completion(.failure("user cancelled"))
    }
}

private class ExportPickerDelegate: NSObject, UIDocumentPickerDelegate {
    private let completion: (Result<Void, String>) -> Void

    init(completion: @escaping (Result<Void, String>) -> Void) {
        self.completion = completion
        super.init()
    }

    func documentPicker(_ controller: UIDocumentPickerViewController, didPickDocumentsAt urls: [URL]) {
        completion(.success(()))
    }

    func documentPickerWasCancelled(_ controller: UIDocumentPickerViewController) {
        completion(.failure("user cancelled"))
    }
}
