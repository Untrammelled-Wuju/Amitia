import Foundation
import Photos
import UIKit

public class MediaStaging {
    private static let stagingDir: URL = {
        let caches = FileManager.default.urls(for: .cachesDirectory, in: .userDomainMask).first!
        let dir = caches.appendingPathComponent("AmitiaMediaStaging", isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true, attributes: nil)
        return dir
    }()

    public static func urlForStagedResource(_ stagingId: String) -> URL? {
        if stagingId.hasPrefix("file://") {
            return nil
        }
        if stagingId.hasPrefix("nativeStaging:") {
            let filename = String(stagingId.dropFirst("nativeStaging:".count))
            let url = stagingDir.appendingPathComponent(filename)
            return FileManager.default.fileExists(atPath: url.path) ? url : nil
        }
        return nil
    }

    public static func deleteStagedResource(_ stagingId: String) {
        if let url = urlForStagedResource(stagingId) {
            try? FileManager.default.removeItem(at: url)
        }
    }

    public static func exportAsset(asset: PHAsset, representation: String, localId: String, request: IOSNativeRequest) async -> IOSNativeResponse {
        let filename = "\(localId)_\(UUID().uuidString).jpg"
        let targetURL = stagingDir.appendingPathComponent(filename)

        let imageManager = PHImageManager.default()
        let options = PHImageRequestOptions()
        options.isSynchronous = false
        options.deliveryMode = representation == "original" ? .highQualityFormat : .fastFormat
        options.isNetworkAccessAllowed = true

        return await withCheckedContinuation { continuation in
            let targetSize = representation == "original" ? PHImageManagerMaximumSize : CGSize(width: asset.pixelWidth, height: asset.pixelHeight)
            imageManager.requestImage(for: asset, targetSize: targetSize, contentMode: .aspectFit, options: options) { image, info in
                guard let image = image else {
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestId: request.requestId,
                        status: "error",
                        result: nil,
                        error: IOSNativeError(code: "QUERY_FAILED", message: "failed to load image")
                    ))
                    return
                }

                guard let data = image.jpegData(compressionQuality: 0.9) else {
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestId: request.requestId,
                        status: "error",
                        result: nil,
                        error: IOSNativeError(code: "QUERY_FAILED", message: "failed to encode image")
                    ))
                    return
                }

                do {
                    try data.write(to: targetURL)
                    let fileSize = (try? FileManager.default.attributesOfItem(atPath: targetURL.path)[.size] as? Int64) ?? 0
                    let resourceUri = targetURL.absoluteString.hasPrefix("file://") ? targetURL.absoluteString : "file://" + targetURL.absoluteString
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestId: request.requestId,
                        status: "ok",
                        result: [
                            "resourceUri": resourceUri,
                            "fileSize": fileSize,
                            "mimeType": "image/jpeg",
                            "width": image.size.width,
                            "height": image.size.height,
                            "filename": filename
                        ],
                        error: nil
                    ))
                } catch {
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestId: request.requestId,
                        status: "error",
                        result: nil,
                        error: IOSNativeError(code: "WRITE_FAILED", message: error.localizedDescription)
                    ))
                }
            }
        }
    }

    public static func cleanupStale(maxAgeHours: Int) -> (removed: Int, scanned: Int) {
        let cutoff = Date().addingTimeInterval(-Double(maxAgeHours) * 3600)
        var removed = 0
        var scanned = 0

        guard let enumerator = FileManager.default.enumerator(at: stagingDir, includingPropertiesForKeys: [.contentModificationDateKey, .isRegularFileKey]) else {
            return (0, 0)
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

        return (removed, scanned)
    }
}
