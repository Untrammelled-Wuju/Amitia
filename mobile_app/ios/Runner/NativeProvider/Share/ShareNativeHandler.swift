import Foundation
import UIKit
import Photos

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

    private let stagingDir: URL = {
        let caches = FileManager.default.urls(for: .cachesDirectory, in: .userDomainMask).first!
        let dir = caches.appendingPathComponent("AmitiaShareStaging", isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true, attributes: nil)
        return dir
    }()

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
            return await handleLimitedDelete(request)
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
        let backgroundCapable = UIApplication.shared.backgroundRefreshStatus == .available
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: [
                "available": true,
                "message": "ShareSheet available",
                "backgroundRefresh": backgroundCapable,
                "receivePending": true,
                "receiveConsume": true,
                "receivePeek": true,
                "receiveDismiss": true,
                "maxResources": 10,
                "maxSingleResourceBytes": 104857600,
                "maxTotalBytes": 524288000,
                "maxTextBytes": 5000
            ],
            error: nil
        )
    }

    private func handleSend(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard isAppInForeground() else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "FOREGROUND_REQUIRED", message: "Share Sheet requires app to be in foreground")
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

        return await presentShareSheet(activityItems: items, on: rootViewController)
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
        guard let files = try? FileManager.default.contentsOfDirectory(atPath: stagingDir.path) else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: [
                    "pendingShares": [],
                    "count": 0
                ],
                error: nil
            )
        }

        let pendingShares = files.map { filename -> [String: Any] in
            let fileURL = stagingDir.appendingPathComponent(filename)
            var createdAt: Date?
            if let attrs = try? FileManager.default.attributesOfItem(atPath: fileURL.path),
               let modDate = attrs[.modificationDate] as? Date {
                createdAt = modDate
            }

            var sizeBytes: Int64 = 0
            if let attrs = try? FileManager.default.attributesOfItem(atPath: fileURL.path),
               let size = attrs[.size] as? Int64 {
                sizeBytes = size
            }

            return [
                "shareId": filename,
                "filename": filename,
                "createdAt": ISO8601DateFormatter().string(from: createdAt ?? Date()),
                "sizeBytes": sizeBytes,
                "stale": false
            ]
        }

        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: [
                "pendingShares": pendingShares,
                "count": pendingShares.count
            ],
            error: nil
        )
    }

    private func handleReceiveConsume(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let shareId = request.payload?["shareId"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing shareId")
            )
        }

        let fileURL = stagingDir.appendingPathComponent(shareId)
        guard FileManager.default.fileExists(atPath: fileURL.path) else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "NOT_FOUND", message: "share not found: \(shareId)")
            )
        }

        var sizeBytes: Int64 = 0
        if let attrs = try? FileManager.default.attributesOfItem(atPath: fileURL.path),
           let size = attrs[.size] as? Int64 {
            sizeBytes = size
        }

        let resourceUri = "share:staging:\(shareId)"
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: [
                "consumed": true,
                "resourceUri": resourceUri,
                "filePath": fileURL.path,
                "sizeBytes": sizeBytes,
                "shareId": shareId
            ],
            error: nil
        )
    }

    private func handleReceivePeek(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let shareId = request.payload?["shareId"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing shareId")
            )
        }

        let fileURL = stagingDir.appendingPathComponent(shareId)
        let exists = FileManager.default.fileExists(atPath: fileURL.path)
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: [
                "found": exists,
                "shareId": shareId,
                "sizeBytes": exists ? (try? FileManager.default.attributesOfItem(atPath: fileURL.path)[.size] as? Int64) ?? 0 : 0
            ],
            error: nil
        )
    }

    private func handleReceiveDismiss(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let shareId = request.payload?["shareId"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing shareId")
            )
        }

        let fileURL = stagingDir.appendingPathComponent(shareId)
        if FileManager.default.fileExists(atPath: fileURL.path) {
            do {
                try FileManager.default.removeItem(at: fileURL)
                return IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestID: request.requestID,
                    status: "ok",
                    result: ["dismissed": true, "shareId": shareId, "removed": true],
                    error: nil
                )
            } catch {
                return IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestID: request.requestID,
                    status: "error",
                    result: nil,
                    error: IOSNativeError(code: "DISMISS_FAILED", message: error.localizedDescription)
                )
            }
        }

        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["dismissed": true, "shareId": shareId, "removed": false],
            error: nil
        )
    }

    private func handleStagingCleanup(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let maxAgeHours = request.payload?["maxStaleAgeHours"] as? Int ?? 24
        let cutoff = Date().addingTimeInterval(-Double(maxAgeHours) * 3600)
        var removed = 0
        var scanned = 0

        guard let enumerator = FileManager.default.enumerator(at: stagingDir, includingPropertiesForKeys: [.contentModificationDateKey, .isRegularFileKey]) else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: ["removed": 0, "scanned": 0],
                error: nil
            )
        }

        for case let fileURL as URL in enumerator {
            scanned += 1
            do {
                let resourceValues = try fileURL.resourceValues(forKeys: [.contentModificationDateKey, .isRegularFileKey])
                if resourceValues.isRegularFile == true,
                   let modDate = resourceValues.contentModificationDate,
                   modDate < cutoff {
                    try FileManager.default.removeItem(at: fileURL)
                    removed += 1
                }
            } catch {
                continue
            }
        }

        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["removed": removed, "scanned": scanned],
            error: nil
        )
    }

    private func handleLimitedDelete(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let photoIds = request.payload?["photoIds"] as? [String], !photoIds.isEmpty else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing photoIds")
            )
        }

        let confirm = request.payload?["confirm"] as? Bool ?? false
        if !confirm {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "USER_CONFIRMATION_REQUIRED", message: "confirm must be true for limited delete")
            )
        }

        let status = PHPhotoLibrary.authorizationStatus(for: .readWrite)
        guard status == .authorized else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PERMISSION_DENIED", message: "photo library access denied")
            )
        }

        let assets = PHAsset.fetchAssets(withLocalIdentifiers: photoIds, options: nil)
        guard assets.count > 0 else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "NOT_FOUND", message: "no matching photos found")
            )
        }

        do {
            try await PHPhotoLibrary.shared().performChanges {
                PHAssetChangeRequest.deleteAssets(assets)
            }
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: ["deleted": true, "count": photoIds.count],
                error: nil
            )
        } catch {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "DELETE_FAILED", message: error.localizedDescription)
            )
        }
    }

    private func buildShareItems(from payload: [String: Any]?) -> [Any] {
        guard let payload = payload else { return [] }
        var items: [Any] = []

        if let text = payload["text"] as? String {
            items.append(text)
        }

        if let urlString = payload["url"] as? String {
            if urlString.hasPrefix("resource:") || urlString.hasPrefix("media:") {
                if let stagingURL = materializeResource(urlString) {
                    items.append(stagingURL)
                }
            } else if let url = URL(string: urlString) {
                items.append(url)
            }
        }

        if let resources = payload["resources"] as? [Any] {
            for res in resources {
                if let resString = res as? String {
                    if resString.hasPrefix("resource:") || resString.hasPrefix("media:") {
                        if let stagingURL = materializeResource(resString) {
                            items.append(stagingURL)
                        }
                    } else if let url = URL(string: resString) {
                        items.append(url)
                    }
                }
            }
        }

        if let shareTitle = payload["shareTitle"] as? String {
            items.append(shareTitle)
        }

        if let itemsArray = payload["items"] as? [Any] {
            items.append(contentsOf: itemsArray)
        }

        return items
    }

    private func materializeResource(_ uri: String) -> URL? {
        if uri.hasPrefix("file://") {
            return URL(string: uri)
        }

        var filename = ""
        if uri.hasPrefix("media:staging:") {
            filename = String(uri.dropFirst("media:staging:".count))
        } else if uri.hasPrefix("resource:") {
            filename = String(uri.dropFirst("resource:".count))
        } else {
            filename = uri
        }

        if filename.isEmpty { return nil }

        let mediaStagingDir = FileManager.default.urls(for: .cachesDirectory, in: .userDomainMask).first!
            .appendingPathComponent("AmitiaMediaStaging", isDirectory: true)
        let shareStagingDir = stagingDir

        let mediaURL = mediaStagingDir.appendingPathComponent(filename)
        if FileManager.default.fileExists(atPath: mediaURL.path) {
            return mediaURL
        }

        let shareURL = shareStagingDir.appendingPathComponent(filename)
        if FileManager.default.fileExists(atPath: shareURL.path) {
            return shareURL
        }

        return URL(string: uri)
    }

    private func presentShareSheet(activityItems: [Any], on viewController: UIViewController) async -> IOSNativeResponse {
        return await withCheckedContinuation { continuation in
            let activityVC = UIActivityViewController(activityItems: activityItems, applicationActivities: nil)

            activityVC.completionWithItemsHandler = { activityType, completed, returnedItems, error in
                DispatchQueue.main.async {
                    if let error = error {
                        continuation.resume(returning: IOSNativeResponse(
                            protocolVersion: activityVC.protocolVersion,
                            requestID: activityVC.requestID,
                            status: "error",
                            result: nil,
                            error: IOSNativeError(code: "SHARE_FAILED", message: error.localizedDescription)
                        ))
                        return
                    }

                    if completed {
                        let activityName = activityType?.rawValue ?? "unknown"
                        continuation.resume(returning: IOSNativeResponse(
                            protocolVersion: activityVC.protocolVersion,
                            requestID: activityVC.requestID,
                            status: "ok",
                            result: [
                                "shared": true,
                                "activityType": activityName,
                                "cancelled": false,
                                "userActionRequired": true,
                                "resourceCount": activityItems.count
                            ],
                            error: nil
                        ))
                    } else {
                        continuation.resume(returning: IOSNativeResponse(
                            protocolVersion: activityVC.protocolVersion,
                            requestID: activityVC.requestID,
                            status: "ok",
                            result: [
                                "shared": false,
                                "activityType": NSNull(),
                                "cancelled": true,
                                "userActionRequired": true,
                                "resourceCount": activityItems.count
                            ],
                            error: nil
                        ))
                    }
                }
            }

            DispatchQueue.main.async {
                var topController = viewController
                while let presentedViewController = topController.presentedViewController {
                    topController = presentedViewController
                }
                topController.present(activityVC, animated: true)
            }
        }
    }

    private func isAppInForeground() -> Bool {
        var inForeground = false
        if Thread.isMainThread {
            inForeground = UIApplication.shared.applicationState == .active
        } else {
            DispatchQueue.main.sync {
                inForeground = UIApplication.shared.applicationState == .active
            }
        }
        return inForeground
    }
}

private extension UIActivityViewController {
    var protocolVersion: String { "1.0" }
    var requestID: String { "share-\(UUID().uuidString)" }
}
