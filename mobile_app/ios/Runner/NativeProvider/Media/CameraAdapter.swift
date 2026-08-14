import Foundation
import AVFoundation
import UIKit

public class CameraAdapter: NSObject {
    public static let shared = CameraAdapter()

    private override init() {
        super.init()
    }

    public func isCameraAvailable() -> Bool {
        return UIImagePickerController.isSourceTypeAvailable(.camera)
    }

    public func authorizationStatus() -> AVAuthorizationStatus {
        return AVCaptureDevice.authorizationStatus(for: .video)
    }

    public func requestPermission() async -> Bool {
        let status = AVCaptureDevice.authorizationStatus(for: .video)
        if status == .authorized {
            return true
        }
        if status == .notDetermined {
            return await AVCaptureDevice.requestAccess(for: .video)
        }
        return false
    }

    public func getAvailableDevices() -> [AVCaptureDevice] {
        let discoverySession = AVCaptureDevice.DiscoverySession(
            deviceTypes: [.builtInWideAngleCamera, .builtInTelephotoCamera, .builtInUltraWideCamera, .builtInTrueDepthCamera],
            mediaType: .video,
            position: .unspecified
        )
        return discoverySession.devices
    }

    public func hasFrontCamera() -> Bool {
        return getAvailableDevices().contains { $0.position == .front }
    }

    public func hasBackCamera() -> Bool {
        return getAvailableDevices().contains { $0.position == .back }
    }
}
