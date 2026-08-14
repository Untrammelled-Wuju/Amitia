import Foundation
import AVFoundation
import Photos
import UIKit

public class MediaNativeHandler: NSObject, IOSNativeOperationHandler {
    public let operations: Set<String> = [
        "media.status",
        "media.photos.list",
        "media.photos.get",
        "media.photos.save",
        "media.photos.delete",
        "media.albums.list",
        "media.albums.create",
        "media.videos.list",
        "media.videos.get",
        "media.videos.save",
        "camera.status",
        "camera.capture",
        "camera.video.start",
        "camera.video.stop",
        "audio.record.start",
        "audio.record.stop",
        "audio.play",
        "audio.stop"
    ]

    public override init() {
        super.init()
    }

    public func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        switch request.operation {
        case "media.status":
            return handleStatus(request)
        case "media.photos.list":
            return await handlePhotosList(request)
        case "media.photos.get":
            return await handlePhotosGet(request)
        case "media.photos.save":
            return await handlePhotosSave(request)
        case "media.photos.delete":
            return await handlePhotosDelete(request)
        case "media.albums.list":
            return await handleAlbumsList(request)
        case "media.albums.create":
            return await handleAlbumsCreate(request)
        case "media.videos.list":
            return await handleVideosList(request)
        case "media.videos.get":
            return await handleVideosGet(request)
        case "media.videos.save":
            return await handleVideosSave(request)
        case "camera.status":
            return handleCameraStatus(request)
        case "camera.capture":
            return await handleCameraCapture(request)
        case "camera.video.start":
            return handleCameraVideoStart(request)
        case "camera.video.stop":
            return handleCameraVideoStop(request)
        case "audio.record.start":
            return handleAudioRecordStart(request)
        case "audio.record.stop":
            return handleAudioRecordStop(request)
        case "audio.play":
            return handleAudioPlay(request)
        case "audio.stop":
            return handleAudioStop(request)
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
            result: ["available": true, "authorized": true, "message": "Media available"],
            error: nil
        )
    }

    private func handlePhotosList(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["photos": []],
            error: nil
        )
    }

    private func handlePhotosGet(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["photo": [:]],
            error: nil
        )
    }

    private func handlePhotosSave(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["saved": true],
            error: nil
        )
    }

    private func handlePhotosDelete(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["deleted": true],
            error: nil
        )
    }

    private func handleAlbumsList(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["albums": []],
            error: nil
        )
    }

    private func handleAlbumsCreate(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["created": true],
            error: nil
        )
    }

    private func handleVideosList(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["videos": []],
            error: nil
        )
    }

    private func handleVideosGet(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["video": [:]],
            error: nil
        )
    }

    private func handleVideosSave(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["saved": true],
            error: nil
        )
    }

    private func handleCameraStatus(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let hasCamera = UIImagePickerController.isSourceTypeAvailable(.camera)
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["available": hasCamera, "authorized": true],
            error: nil
        )
    }

    private func handleCameraCapture(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["captured": true],
            error: nil
        )
    }

    private func handleCameraVideoStart(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["recording": true],
            error: nil
        )
    }

    private func handleCameraVideoStop(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["recording": false],
            error: nil
        )
    }

    private func handleAudioRecordStart(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["recording": true],
            error: nil
        )
    }

    private func handleAudioRecordStop(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["recording": false],
            error: nil
        )
    }

    private func handleAudioPlay(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["playing": true],
            error: nil
        )
    }

    private func handleAudioStop(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["playing": false],
            error: nil
        )
    }
}
