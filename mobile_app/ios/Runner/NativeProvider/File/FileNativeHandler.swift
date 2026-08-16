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
            return handleMountReauthorize(request)
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
                requestId: request.requestId,
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
                requestId: request.requestId,
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
                case .success(let mountRecord):
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestId: request.requestId,
                        status: "ok",
                        result: [
                            "mountId": mountRecord.mountId,
                            "displayName": mountRecord.displayName,
                            "isSingleFile": mountRecord.isSingleFile,
                            "picked": true
                        ],
                        error: nil
                    ))
                case .failure(let error):
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestId: request.requestId,
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
                requestId: request.requestId,
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
                    case .success(let mountRecord):
                        continuation.resume(returning: IOSNativeResponse(
                            protocolVersion: request.protocolVersion,
                            requestId: request.requestId,
                            status: "ok",
                            result: [
                                "mountId": mountRecord.mountId,
                                "displayName": mountRecord.displayName,
                                "isSingleFile": mountRecord.isSingleFile,
                                "picked": true
                            ],
                            error: nil
                        ))
                    case .failure(let error):
                        continuation.resume(returning: IOSNativeResponse(
                            protocolVersion: request.protocolVersion,
                            requestId: request.requestId,
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
            requestId: request.requestId,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "iOS 14.0+ required")
        )
    }

    private func handleMountReauthorize(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let mountId = request.payload?["mountId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing mountId")
        }
        let reauthorized = SecurityScopedBookmarkStore.shared.reauthorize(mountId: mountId)
        if !reauthorized {
            return errorResponse(request, code: "REAUTHORIZE_FAILED", message: "failed to reauthorize mount: \(mountId)")
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: ["reauthorized": reauthorized, "mountId": mountId],
            error: nil
        )
    }

    private func handleAccessStat(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let mountId = request.payload?["mountId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing mountId")
        }
        let relativePath = request.payload?["relativePath"] as? String ?? ""
        do {
            let attributes = try SecurityScopedBookmarkStore.shared.stat(mountId: mountId, relativePath: relativePath)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["stat": attributes],
                error: nil
            )
        } catch let error as BookmarkStoreError {
            return errorResponse(request, code: error.errorCode, message: error.errorDescription ?? "unknown error")
        } catch {
            return errorResponse(request, code: "STAT_FAILED", message: error.localizedDescription)
        }
    }

    private func handleAccessList(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let mountId = request.payload?["mountId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing mountId")
        }
        let relativePath = request.payload?["relativePath"] as? String ?? ""
        do {
            let items = try SecurityScopedBookmarkStore.shared.list(mountId: mountId, relativePath: relativePath)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["items": items, "mountId": mountId, "relativePath": relativePath],
                error: nil
            )
        } catch let error as BookmarkStoreError {
            return errorResponse(request, code: error.errorCode, message: error.errorDescription ?? "unknown error")
        } catch {
            return errorResponse(request, code: "LIST_FAILED", message: error.localizedDescription)
        }
    }

    private func handleAccessRead(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let mountId = request.payload?["mountId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing mountId")
        }
        let relativePath = request.payload?["relativePath"] as? String ?? ""
        let offset = Int64(request.payload?["offset"] as? Int ?? 0)
        let length = Int64(request.payload?["length"] as? Int ?? 0)
        do {
            let data = try SecurityScopedBookmarkStore.shared.readFile(
                mountId: mountId,
                relativePath: relativePath,
                offset: offset,
                length: length
            )
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: [
                    "size": data.count,
                    "mountId": mountId,
                    "relativePath": relativePath,
                    "contentBase64": data.base64EncodedString()
                ],
                error: nil
            )
        } catch let error as BookmarkStoreError {
            return errorResponse(request, code: error.errorCode, message: error.errorDescription ?? "unknown error")
        } catch {
            return errorResponse(request, code: "READ_FAILED", message: error.localizedDescription)
        }
    }

    private func handleAccessWrite(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let mountId = request.payload?["mountId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing mountId")
        }
        let relativePath = request.payload?["relativePath"] as? String ?? ""
        guard let contentBase64 = request.payload?["contentBase64"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing contentBase64")
        }
        let offset = Int64(request.payload?["offset"] as? Int ?? 0)
        do {
            let success = try SecurityScopedBookmarkStore.shared.writeFile(
                mountId: mountId,
                relativePath: relativePath,
                contentBase64: contentBase64,
                offset: offset
            )
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["written": success, "mountId": mountId, "relativePath": relativePath],
                error: nil
            )
        } catch let error as BookmarkStoreError {
            return errorResponse(request, code: error.errorCode, message: error.errorDescription ?? "unknown error")
        } catch {
            return errorResponse(request, code: "WRITE_FAILED", message: error.localizedDescription)
        }
    }

    private func handleAccessMkdir(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let mountId = request.payload?["mountId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing mountId")
        }
        let parentRelativePath = request.payload?["parentRelativePath"] as? String ?? ""
        guard let name = request.payload?["name"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing name")
        }
        do {
            let newMountId = try SecurityScopedBookmarkStore.shared.mkdir(
                mountId: mountId,
                parentRelativePath: parentRelativePath,
                name: name
            )
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["mountId": newMountId, "name": name],
                error: nil
            )
        } catch let error as BookmarkStoreError {
            return errorResponse(request, code: error.errorCode, message: error.errorDescription ?? "unknown error")
        } catch {
            return errorResponse(request, code: "MKDIR_FAILED", message: error.localizedDescription)
        }
    }

    private func handleAccessRename(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let mountId = request.payload?["mountId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing mountId")
        }
        let relativePath = request.payload?["relativePath"] as? String ?? ""
        guard let newName = request.payload?["newName"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing newName")
        }
        do {
            try SecurityScopedBookmarkStore.shared.rename(
                mountId: mountId,
                relativePath: relativePath,
                newName: newName
            )
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["renamed": true, "mountId": mountId, "relativePath": relativePath, "newName": newName],
                error: nil
            )
        } catch let error as BookmarkStoreError {
            return errorResponse(request, code: error.errorCode, message: error.errorDescription ?? "unknown error")
        } catch {
            return errorResponse(request, code: "RENAME_FAILED", message: error.localizedDescription)
        }
    }

    private func handleAccessMove(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let mountId = request.payload?["mountId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing mountId")
        }
        guard let relativePath = request.payload?["relativePath"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing relativePath")
        }
        guard let newRelativePath = request.payload?["newRelativePath"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing newRelativePath")
        }
        do {
            try SecurityScopedBookmarkStore.shared.move(
                mountId: mountId,
                relativePath: relativePath,
                newRelativePath: newRelativePath
            )
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: [
                    "moved": true,
                    "mountId": mountId,
                    "relativePath": relativePath,
                    "newRelativePath": newRelativePath
                ],
                error: nil
            )
        } catch let error as BookmarkStoreError {
            return errorResponse(request, code: error.errorCode, message: error.errorDescription ?? "unknown error")
        } catch {
            return errorResponse(request, code: "MOVE_FAILED", message: error.localizedDescription)
        }
    }

    private func handleAccessCopy(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let mountId = request.payload?["mountId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing mountId")
        }
        guard let relativePath = request.payload?["relativePath"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing relativePath")
        }
        guard let newRelativePath = request.payload?["newRelativePath"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing newRelativePath")
        }
        do {
            let newMountId = try SecurityScopedBookmarkStore.shared.copy(
                mountId: mountId,
                relativePath: relativePath,
                newRelativePath: newRelativePath
            )
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: [
                    "copied": true,
                    "mountId": newMountId,
                    "relativePath": relativePath,
                    "newRelativePath": newRelativePath
                ],
                error: nil
            )
        } catch let error as BookmarkStoreError {
            return errorResponse(request, code: error.errorCode, message: error.errorDescription ?? "unknown error")
        } catch {
            return errorResponse(request, code: "COPY_FAILED", message: error.localizedDescription)
        }
    }

    private func handleAccessDelete(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let mountId = request.payload?["mountId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing mountId")
        }
        let relativePath = request.payload?["relativePath"] as? String ?? ""
        do {
            try SecurityScopedBookmarkStore.shared.delete(mountId: mountId, relativePath: relativePath)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["deleted": true, "mountId": mountId, "relativePath": relativePath],
                error: nil
            )
        } catch let error as BookmarkStoreError {
            return errorResponse(request, code: error.errorCode, message: error.errorDescription ?? "unknown error")
        } catch {
            return errorResponse(request, code: "DELETE_FAILED", message: error.localizedDescription)
        }
    }

    private func handleExport(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let mountId = request.payload?["mountId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing mountId")
        }
        let relativePath = request.payload?["relativePath"] as? String ?? ""
        do {
            let resolved = try SecurityScopedBookmarkStore.shared.resolvePath(mountId: mountId, relativePath: relativePath)

            guard let scene = UIApplication.shared.connectedScenes.first as? UIWindowScene,
                  let window = scene.windows.first,
                  let rootViewController = window.rootViewController else {
                return errorResponse(request, code: "FOREGROUND_REQUIRED", message: "No active foreground UIWindowScene for export")
            }

            _ = try SecurityScopedBookmarkStore.shared.resolveRootURL(mountId: mountId)

            return await withCheckedContinuation { continuation in
                let documentPicker = UIDocumentPickerViewController(forExporting: [resolved.resolvedURL], asCopy: true)
                documentPicker.allowsMultipleSelection = false

                let delegate = ExportPickerDelegate { result in
                    switch result {
                    case .success:
                        continuation.resume(returning: IOSNativeResponse(
                            protocolVersion: request.protocolVersion,
                            requestId: request.requestId,
                            status: "ok",
                            result: ["exported": true, "mountId": mountId, "relativePath": relativePath],
                            error: nil
                        ))
                    case .failure(let error):
                        continuation.resume(returning: IOSNativeResponse(
                            protocolVersion: request.protocolVersion,
                            requestId: request.requestId,
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
        } catch let error as BookmarkStoreError {
            return errorResponse(request, code: error.errorCode, message: error.errorDescription ?? "unknown error")
        } catch {
            return errorResponse(request, code: "EXPORT_FAILED", message: error.localizedDescription)
        }
    }

    private func handleMountGet(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let mountId = request.payload?["mountId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing mountId")
        }
        guard let record = SecurityScopedBookmarkStore.shared.getMountRecord(mountId: mountId) else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["mount": NSNull()],
                error: nil
            )
        }
        let mount: [String: Any] = [
            "mountId": record.mountId,
            "displayName": record.displayName,
            "readOnly": record.readOnly,
            "providerHint": record.providerHint,
            "createdAt": record.createdAt.timeIntervalSince1970,
            "isSingleFile": record.isSingleFile
        ]
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: ["mount": mount],
            error: nil
        )
    }

    private func handleMountList(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let mounts = SecurityScopedBookmarkStore.shared.listMounts()
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: ["mounts": mounts],
            error: nil
        )
    }

    private func handleMountRemove(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let mountId = request.payload?["mountId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing mountId")
        }
        SecurityScopedBookmarkStore.shared.remove(mountId: mountId)
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: ["removed": true, "mountId": mountId],
            error: nil
        )
    }

    private func handleGetCapabilities(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: [
                "canPickFiles": true,
                "canPickDirectories": true,
                "canRead": true,
                "canWrite": true,
                "canDelete": true,
                "canExport": true,
                "canMount": true,
                "pathModel": "mountId+relativePath"
            ],
            error: nil
        )
    }

    private func errorResponse(_ request: IOSNativeRequest, code: String, message: String) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "error",
            result: nil,
            error: IOSNativeError(code: code, message: message)
        )
    }
}

private var delegateKey: UInt8 = 0
private var exportDelegateKey: UInt8 = 0

private class DocumentPickerDelegate: NSObject, UIDocumentPickerDelegate {
    private let completion: (Result<MountRecord, String>) -> Void

    init(completion: @escaping (Result<MountRecord, String>) -> Void) {
        self.completion = completion
        super.init()
    }

    func documentPicker(_ controller: UIDocumentPickerViewController, didPickDocumentsAt urls: [URL]) {
        guard let url = urls.first else {
            completion(.failure("no file selected"))
            return
        }
        do {
            let record = try SecurityScopedBookmarkStore.shared.createBookmark(for: url)
            completion(.success(record))
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
