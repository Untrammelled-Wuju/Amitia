import Foundation
import AVFoundation
import Photos
import UIKit

public class MediaNativeHandler: NSObject, IOSNativeOperationHandler {
    public let operations: Set<String> = [
        "media.status",
        "media.photos.pick",
        "media.photos.status",
        "media.photos.list",
        "media.photos.get",
        "media.photos.export",
        "media.photos.save",
        "media.photos.delete",
        "media.photos.manage_limited_access",
        "media.camera.status",
        "media.camera.devices",
        "media.camera.capture_photo",
        "media.camera.record_video",
        "media.audio.status",
        "media.audio.record",
        "native.resource.stat",
        "native.resource.read_chunk",
        "native.resource.release"
    ]

    public override init() {
        super.init()
    }

    public func capabilitySnapshot() -> IOSNativeCapability {
        return IOSNativeCapability(
            available: true,
            authorized: false,
            hardwareAvailable: true,
            platformSupported: true,
            foregroundRequired: true
        )
    }

    public func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        switch request.operation {
        case "media.status":
            return handleStatus(request)
        case "media.photos.pick":
            return await handlePhotosPick(request)
        case "media.photos.status":
            return handlePhotosStatus(request)
        case "media.photos.list":
            return await handlePhotosList(request)
        case "media.photos.get":
            return await handlePhotosGet(request)
        case "media.photos.export":
            return await handlePhotosExport(request)
        case "media.photos.save":
            return await handlePhotosSave(request)
        case "media.photos.delete":
            return await handlePhotosDelete(request)
        case "media.photos.manage_limited_access":
            return handlePhotosManageLimitedAccess(request)
        case "media.camera.status":
            return handleCameraStatus(request)
        case "media.camera.devices":
            return handleCameraDevices(request)
        case "media.camera.capture_photo":
            return await handleCameraCapturePhoto(request)
        case "media.camera.record_video":
            return await handleCameraRecordVideo(request)
        case "media.audio.status":
            return handleAudioStatus(request)
        case "media.audio.record":
            return await handleAudioRecord(request)
        case "native.resource.stat":
            return handleResourceStat(request)
        case "native.resource.read_chunk":
            return handleResourceReadChunk(request)
        case "native.resource.release":
            return handleResourceRelease(request)
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

    private func handleStatus(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let photoStatus = PHPhotoLibrary.authorizationStatus(for: .readWrite)
        let cameraAuth = AVCaptureDevice.authorizationStatus(for: .video)
        let micAuth = AVCaptureDevice.authorizationStatus(for: .audio)

        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: [
                "available": true,
                "photosAuthorized": photoStatus == .authorized || photoStatus == .limited,
                "cameraAuthorized": cameraAuth == .authorized,
                "microphoneAuthorized": micAuth == .authorized
            ],
            error: nil
        )
    }

    private func handlePhotosPick(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let status = PHPhotoLibrary.authorizationStatus(for: .readWrite)
        if status == .denied || status == .restricted {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PERMISSION_DENIED", message: "photo library access denied")
            )
        }

        if status == .notDetermined {
            let newStatus = await PHPhotoLibrary.requestAuthorization(for: .readWrite)
            if newStatus == .denied || newStatus == .restricted {
                return IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestId: request.requestId,
                    status: "error",
                    result: nil,
                    error: IOSNativeError(code: "PERMISSION_DENIED", message: "photo library access denied")
                )
            }
        }

        let limit = request.payload?["limit"] as? Int ?? 1
        let mediaType = request.payload?["mediaType"] as? String ?? "image"

        do {
            let fetchOptions = PHFetchOptions()
            fetchOptions.sortDescriptors = [NSSortDescriptor(key: "creationDate", ascending: false)]
            fetchOptions.fetchLimit = limit

            let assetType: PHAssetMediaType = mediaType == "video" ? .video : .image
            let result = PHAsset.fetchAssets(with: assetType, options: fetchOptions)

            var assets: [[String: Any]] = []
            result.enumerateObjects { asset, _, _ in
                assets.append([
                    "localIdentifier": asset.localIdentifier,
                    "mediaType": asset.mediaType.rawValue,
                    "creationDate": asset.creationDate?.timeIntervalSince1970 ?? 0
                ])
            }

            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["photos": assets, "count": assets.count],
                error: nil
            )
        }
    }

    private func handlePhotosStatus(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let status = PHPhotoLibrary.authorizationStatus(for: .readWrite)
        let statusString: String
        switch status {
        case .authorized: statusString = "authorized"
        case .limited: statusString = "limited"
        case .denied: statusString = "denied"
        case .restricted: statusString = "restricted"
        case .notDetermined: statusString = "notDetermined"
        @unknown default: statusString = "unknown"
        }

        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: ["status": statusString, "authorized": status == .authorized || status == .limited],
            error: nil
        )
    }

    private func handlePhotosList(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let status = PHPhotoLibrary.authorizationStatus(for: .readWrite)
        guard status == .authorized || status == .limited else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PERMISSION_DENIED", message: "photo library access denied")
            )
        }

        let limit = request.payload?["limit"] as? Int ?? 100
        let offset = request.payload?["offset"] as? Int ?? 0

        let fetchOptions = PHFetchOptions()
        fetchOptions.sortDescriptors = [NSSortDescriptor(key: "creationDate", ascending: false)]
        fetchOptions.fetchLimit = limit
        fetchOptions.fetchOffset = offset

        let result = PHAsset.fetchAssets(with: fetchOptions)
        var photos: [[String: Any]] = []
        result.enumerateObjects { asset, _, _ in
            photos.append([
                "localIdentifier": asset.localIdentifier,
                "mediaType": asset.mediaType.rawValue,
                "creationDate": asset.creationDate?.timeIntervalSince1970 ?? 0,
                "pixelWidth": asset.pixelWidth,
                "pixelHeight": asset.pixelHeight
            ])
        }

        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: ["photos": photos, "count": photos.count],
            error: nil
        )
    }

    private func handlePhotosGet(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let localId = request.payload?["localIdentifier"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing localIdentifier")
            )
        }

        let status = PHPhotoLibrary.authorizationStatus(for: .readWrite)
        guard status == .authorized || status == .limited else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PERMISSION_DENIED", message: "photo library access denied")
            )
        }

        guard let asset = PHAsset.fetchAssets(withLocalIdentifiers: [localId], options: nil).firstObject else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "NOT_FOUND", message: "photo not found: \(localId)")
            )
        }

        let representation = request.payload?["representation"] as? String ?? "current"
        return await MediaStaging.exportAsset(asset: asset, representation: representation, localId: localId, request: request)
    }

    private func handlePhotosExport(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let localId = request.payload?["localIdentifier"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing localIdentifier")
            )
        }

        let status = PHPhotoLibrary.authorizationStatus(for: .readWrite)
        guard status == .authorized || status == .limited else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PERMISSION_DENIED", message: "photo library access denied")
            )
        }

        guard let asset = PHAsset.fetchAssets(withLocalIdentifiers: [localId], options: nil).firstObject else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "NOT_FOUND", message: "photo not found: \(localId)")
            )
        }

        let representation = request.payload?["representation"] as? String ?? "current"
        return await MediaStaging.exportAsset(asset: asset, representation: representation, localId: localId, request: request)
    }

    private func handlePhotosSave(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let resourceUri = request.payload?["resourceUri"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing resourceUri")
            )
        }

        let fileURL = MediaStaging.urlForStagedResource(resourceUri)
        guard fileURL != nil || resourceUri.hasPrefix("file://") else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "resource not staged: \(resourceUri)")
            )
        }

        let sourceURL: URL
        if let staged = fileURL {
            sourceURL = staged
        } else {
            sourceURL = URL(string: resourceUri)!
        }

        guard let imageData = readDataChunked(from: sourceURL),
              let image = UIImage(data: imageData) else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "failed to load image data from \(sourceURL.path)")
            )
        }

        let authStatus = PHPhotoLibrary.authorizationStatus(for: .readWrite)
        if authStatus == .notDetermined {
            let newStatus = await PHPhotoLibrary.requestAuthorization(for: .readWrite)
            if newStatus == .denied || newStatus == .restricted {
                return IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestId: request.requestId,
                    status: "error",
                    result: nil,
                    error: IOSNativeError(code: "PERMISSION_DENIED", message: "photo library access denied")
                )
            }
        } else if authStatus == .denied || authStatus == .restricted {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PERMISSION_DENIED", message: "photo library access denied")
            )
        }

        do {
            try await PHPhotoLibrary.shared().performChanges {
                let creationRequest = PHAssetCreationRequest.forAsset()
                creationRequest.addResource(with: .photo, data: imageData, options: nil)
            }
            MediaStaging.deleteStagedResource(resourceUri)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["saved": true],
                error: nil
            )
        } catch {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "SAVE_FAILED", message: error.localizedDescription)
            )
        }
    }

    private func handlePhotosDelete(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let localIds = request.payload?["localIdentifiers"] as? [String], !localIds.isEmpty else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing localIdentifiers")
            )
        }

        let status = PHPhotoLibrary.authorizationStatus(for: .readWrite)
        guard status == .authorized else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PERMISSION_DENIED", message: "photo library access denied")
            )
        }

        let assets = PHAsset.fetchAssets(withLocalIdentifiers: localIds, options: nil)

        do {
            try await PHPhotoLibrary.shared().performChanges {
                PHAssetChangeRequest.deleteAssets(assets)
            }
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["deleted": true, "count": localIds.count],
                error: nil
            )
        } catch {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "DELETE_FAILED", message: error.localizedDescription)
            )
        }
    }

    private func handlePhotosManageLimitedAccess(_ request: IOSNativeRequest) -> IOSNativeResponse {
        if #available(iOS 15, *) {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: ["presentLimitedLibraryPicker": true],
                error: nil
            )
        } else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "limited library picker requires iOS 15+")
            )
        }
    }

    private func handleCameraStatus(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let hasCamera = UIImagePickerController.isSourceTypeAvailable(.camera)
        let authStatus = AVCaptureDevice.authorizationStatus(for: .video)
        let authorized = authStatus == .authorized

        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: ["available": hasCamera, "authorized": authorized, "status": "\(authStatus.rawValue)"],
            error: nil
        )
    }

    private func handleCameraDevices(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let discoverySession = AVCaptureDevice.DiscoverySession(
            deviceTypes: [.builtInWideAngleCamera, .builtInTelephotoCamera, .builtInUltraWideCamera],
            mediaType: .video,
            position: .unspecified
        )

        let devices = discoverySession.devices.map { device in
            [
                "uniqueId": device.uniqueID,
                "localizedName": device.localizedName,
                "position": device.position.rawValue,
                "deviceType": device.deviceType.rawValue
            ]
        }

        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: ["devices": devices, "count": devices.count],
            error: nil
        )
    }

    private func handleCameraCapturePhoto(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let authStatus = AVCaptureDevice.authorizationStatus(for: .video)
        var authorized = authStatus == .authorized

        if authStatus == .notDetermined {
            if #available(iOS 16.0, *) {
                authorized = await AVCaptureDevice.requestAccess(for: .video)
            } else {
                return IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestId: request.requestId,
                    status: "error",
                    result: nil,
                    error: IOSNativeError(code: "FOREGROUND_REQUIRED", message: "camera capture requires explicit user authorization prompt in foreground")
                )
            }
        }

        if !authorized {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PERMISSION_DENIED", message: "camera access denied")
            )
        }

        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "FOREGROUND_REQUIRED", message: "photo capture requires presenting system camera UI in foreground scene")
        )
    }

    private func handleCameraRecordVideo(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let videoAuth = AVCaptureDevice.authorizationStatus(for: .video)
        var videoAuthorized = videoAuth == .authorized

        if videoAuth == .notDetermined {
            if #available(iOS 16.0, *) {
                videoAuthorized = await AVCaptureDevice.requestAccess(for: .video)
            } else {
                return IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestId: request.requestId,
                    status: "error",
                    result: nil,
                    error: IOSNativeError(code: "FOREGROUND_REQUIRED", message: "video recording requires explicit user authorization prompt in foreground")
                )
            }
        }

        if !videoAuthorized {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PERMISSION_DENIED", message: "camera access denied")
            )
        }

        let micAuth = AVCaptureDevice.authorizationStatus(for: .audio)
        if micAuth == .notDetermined {
            if #available(iOS 16.0, *) {
                _ = await AVCaptureDevice.requestAccess(for: .audio)
            }
        }

        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "FOREGROUND_REQUIRED", message: "video recording requires presenting system camera UI in foreground scene")
        )
    }

    private func handleAudioStatus(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let authStatus = AVCaptureDevice.authorizationStatus(for: .audio)
        let authorized = authStatus == .authorized

        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: ["authorized": authorized, "status": "\(authStatus.rawValue)"],
            error: nil
        )
    }

    private func handleAudioRecord(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let authStatus = AVCaptureDevice.authorizationStatus(for: .audio)
        var authorized = authStatus == .authorized

        if authStatus == .notDetermined {
            if #available(iOS 16.0, *) {
                authorized = await AVCaptureDevice.requestAccess(for: .audio)
            } else {
                return IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestId: request.requestId,
                    status: "error",
                    result: nil,
                    error: IOSNativeError(code: "FOREGROUND_REQUIRED", message: "audio recording requires explicit user authorization prompt in foreground")
                )
            }
        }

        if !authorized {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PERMISSION_DENIED", message: "microphone access denied")
            )
        }

        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "error",
            result: nil,
            error: IOSNativeError(code: "FOREGROUND_REQUIRED", message: "audio recording requires presenting recording UI in foreground scene")
        )
    }

    private func handleResourceStat(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let nativeStagingId = request.payload?["nativeStagingId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing nativeStagingId")
        }
        guard let info = MediaStaging.stat(nativeStagingId: nativeStagingId) else {
            return errorResponse(request, code: "NOT_FOUND", message: "staged resource not found: \(nativeStagingId)")
        }
        return successResponse(request, result: info.statDictionary)
    }

    private func handleResourceReadChunk(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let nativeStagingId = request.payload?["nativeStagingId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing nativeStagingId")
        }
        let offset = Int64(request.payload?["offset"] as? Int ?? 0)
        let length = Int64(request.payload?["length"] as? Int ?? 1048576)
        guard let data = MediaStaging.readChunk(nativeStagingId: nativeStagingId, offset: offset, length: length) else {
            return errorResponse(request, code: "READ_FAILED", message: "failed to read chunk from \(nativeStagingId)")
        }
        return successResponse(request, result: [
            "nativeStagingId": nativeStagingId,
            "offset": offset,
            "length": data.count,
            "data": data.base64EncodedString()
        ])
    }

    private func handleResourceRelease(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let nativeStagingId = request.payload?["nativeStagingId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing nativeStagingId")
        }
        let released = MediaStaging.release(nativeStagingId: nativeStagingId)
        return successResponse(request, result: ["released": released, "nativeStagingId": nativeStagingId])
    }

    private func readDataChunked(from url: URL, maxBytes: Int64 = 104857600) -> Data? {
        guard let handle = try? FileHandle(forReadingFrom: url) else {
            return nil
        }
        defer { handle.closeFile() }
        var result = Data()
        var totalRead: Int64 = 0
        while totalRead < maxBytes {
            let chunk = handle.readData(ofLength: 1048576)
            if chunk.isEmpty { break }
            result.append(chunk)
            totalRead += Int64(chunk.count)
        }
        return result.isEmpty ? nil : result
    }
}
