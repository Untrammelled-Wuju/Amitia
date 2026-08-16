import Foundation
import Photos
import UIKit

public struct StagedResourceInfo {
    public let nativeStagingId: String
    public let mimeType: String
    public let size: Int64
    public let width: Double?
    public let height: Double?
    public let filename: String
    public let createdAt: TimeInterval

    public var statDictionary: [String: Any] {
        var dict: [String: Any] = [
            "nativeStagingId": nativeStagingId,
            "mimeType": mimeType,
            "size": size,
            "filename": filename,
            "createdAt": createdAt
        ]
        if let w = width { dict["width"] = w }
        if let h = height { dict["height"] = h }
        return dict
    }
}

public class MediaStaging {
    private static let stagingDir: URL = {
        let caches = FileManager.default.urls(for: .cachesDirectory, in: .userDomainMask).first!
        let dir = caches.appendingPathComponent("AmitiaNativeStaging", isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true, attributes: nil)
        return dir
    }()

    private static let metadataKey = "AmitiaStagingMetadata"
    private static var metadata: [String: [String: Any]] = [:]
    private static let lock = NSLock()

    public static func nativeStagingURL(_ nativeStagingId: String) -> URL? {
        guard nativeStagingId.hasPrefix("nativeStaging:") else {
            return nil
        }
        let filename = String(nativeStagingId.dropFirst("nativeStaging:".count))
        guard !filename.isEmpty,
              !filename.contains("/"),
              !filename.contains(".."),
              !filename.contains(":") else {
            return nil
        }
        let url = stagingDir.appendingPathComponent(filename)
        return FileManager.default.fileExists(atPath: url.path) ? url : nil
    }

    public static func registerResource(nativeStagingId: String, info: [String: Any]) {
        lock.lock()
        metadata[nativeStagingId] = info
        lock.unlock()
    }

    public static func stat(nativeStagingId: String) -> StagedResourceInfo? {
        guard let url = nativeStagingURL(nativeStagingId) else {
            return nil
        }
        do {
            let attributes = try FileManager.default.attributesOfItem(atPath: url.path)
            let size = attributes[.size] as? Int64 ?? 0
            let createdAt = (attributes[.creationDate] as? Date)?.timeIntervalSince1970 ?? 0

            lock.lock()
            let meta = metadata[nativeStagingId]
            lock.unlock()

            return StagedResourceInfo(
                nativeStagingId: nativeStagingId,
                mimeType: meta?["mimeType"] as? String ?? "application/octet-stream",
                size: size,
                width: meta?["width"] as? Double,
                height: meta?["height"] as? Double,
                filename: url.lastPathComponent,
                createdAt: createdAt
            )
        } catch {
            return nil
        }
    }

    public static func readChunk(nativeStagingId: String, offset: Int64, length: Int64) -> Data? {
        guard let url = nativeStagingURL(nativeStagingId) else {
            return nil
        }
        guard length > 0 else {
            return nil
        }
        let boundedLength = min(length, Int64(MaxNativeChunk))
        do {
            let handle = try FileHandle(forReadingFrom: url)
            if offset > 0 {
                handle.seek(toOffset: UInt64(offset))
            }
            let data = handle.readData(ofLength: Int(boundedLength))
            handle.closeFile()
            return data
        } catch {
            return nil
        }
    }

    public static func release(nativeStagingId: String) -> Bool {
        guard let url = nativeStagingURL(nativeStagingId) else {
            return false
        }
        do {
            try FileManager.default.removeItem(at: url)
            lock.lock()
            metadata.removeValue(forKey: nativeStagingId)
            lock.unlock()
            return true
        } catch {
            lock.lock()
            metadata.removeValue(forKey: nativeStagingId)
            lock.unlock()
            return false
        }
    }

    public static func exportAsset(asset: PHAsset, representation: String, localId: String, request: IOSNativeRequest) async -> IOSNativeResponse {
        let stagingId = UUID().uuidString
        let filename = "\(localId)_\(stagingId).jpg"
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
                    let nativeStagingId = "nativeStaging:\(filename)"

                    MediaStaging.registerResource(nativeStagingId: nativeStagingId, info: [
                        "mimeType": "image/jpeg",
                        "width": Double(image.size.width),
                        "height": Double(image.size.height),
                        "filename": filename
                    ])

                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestId: request.requestId,
                        status: "ok",
                        result: [
                            "nativeStagingId": nativeStagingId,
                            "size": fileSize,
                            "mimeType": "image/jpeg",
                            "width": Double(image.size.width),
                            "height": Double(image.size.height),
                            "filename": filename,
                            "originalLocalId": localId
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

        lock.lock()
        let currentFilenames = Set(metadata.values.compactMap { $0["filename"] as? String })
        lock.unlock()

        for case let fileURL as URL in enumerator {
            scanned += 1
            do {
                let resourceValues = try fileURL.resourceValues(forKeys: [.contentModificationDateKey, .isRegularFileKey])
                if resourceValues.isRegularFile == true,
                   let modDate = resourceValues.contentModificationDate,
                   modDate < cutoff {
                    try FileManager.default.removeItem(at: fileURL)
                    let fn = fileURL.lastPathComponent
                    let toRemove = currentFilenames.contains(fn) ? metadata.keys.first(where: { key in
                        guard let m = metadata[key], let mfn = m["filename"] as? String else { return false }
                        return mfn == fn
                    }) : nil
                    if let key = toRemove {
                        lock.lock()
                        metadata.removeValue(forKey: key)
                        lock.unlock()
                    }
                    removed += 1
                }
            } catch {
                continue
            }
        }

        return (removed, scanned)
    }

    public static func isNativeStagingId(_ value: String) -> Bool {
        return value.hasPrefix("nativeStaging:")
    }

    public static func urlForStagedResource(_ resourceUri: String) -> URL? {
        if resourceUri.hasPrefix("file://") {
            return URL(string: resourceUri)
        }
        if isNativeStagingId(resourceUri) {
            return nativeStagingURL(resourceUri)
        }
        return nil
    }

    public static func deleteStagedResource(_ resourceUri: String) {
        if isNativeStagingId(resourceUri) {
            _ = release(resourceUri)
        }
    }
}
