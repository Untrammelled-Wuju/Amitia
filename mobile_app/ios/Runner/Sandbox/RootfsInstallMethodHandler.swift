import Flutter
import UIKit

class RootfsInstallMethodHandler: NSObject {
    private let installer: RootfsInstaller
    private let resolver: RootfsResolver

    init(installer: RootfsInstaller, resolver: RootfsResolver) {
        self.installer = installer
        self.resolver = resolver
        super.init()
    }

    func register(with registrar: FlutterPluginRegistrar) {
        let channel = FlutterMethodChannel(name: "com.amitia.ios_rootfs", binaryMessenger: registrar.messenger())
        channel.setMethodCallHandler { [weak self] call, result in
            self?.handleMethodCall(call: call, result: result)
        }

        let events = FlutterEventChannel(name: "com.amitia.ios_rootfs/events", binaryMessenger: registrar.messenger())
        events.setStreamHandler(RootfsInstallEventHandler(installer: self.installer))
    }

    private func handleMethodCall(call: FlutterMethodCall, result: @escaping FlutterResult) {
        switch call.method {
        case "getCurrentRootfs":
            result(self.currentRootfsMap())
        case "isInstalled":
            if let args = call.arguments as? [String: Any],
               let version = args["version"] as? String,
               let arch = args["architecture"] as? String {
                result(self.resolver.isInstalledVersion(version, architecture: arch))
            } else {
                result(false)
            }
        case "listInstalled":
            result(self.listInstalledMaps())
        case "install":
            if let args = call.arguments as? [String: Any] {
                self.handleInstall(args: args, result: result)
            } else {
                result(FlutterError(code: "INVALID_ARGS", message: "install requires arguments", details: nil))
            }
        case "verify":
            if let desc = self.resolver.resolveCurrentRootfs() {
                var err: NSError?
                let ok = self.installer.verifyInstalledRootfs(desc, error: &err)
                if ok {
                    result(true)
                } else {
                    result(FlutterError(code: "VERIFY_FAILED", message: err?.localizedDescription ?? "rootfs verification failed", details: self.safeDetails(err)))
                }
            } else {
                result(FlutterError(code: "NOT_INSTALLED", message: "no active rootfs", details: nil))
            }
        case "deactivate":
            if let args = call.arguments as? [String: Any],
               let version = args["version"] as? String,
               let arch = args["architecture"] as? String {
                var err: NSError?
                let ok = self.installer.deactivateRootfsVersion(version, architecture: arch, error: &err)
                result(ok)
            } else {
                result(false)
            }
        default:
            result(FlutterMethodNotImplemented)
        }
    }

    private func currentRootfsMap() -> [String: Any]? {
        guard let desc = self.resolver.resolveCurrentRootfs() else { return nil }
        return [
            "version": desc.version,
            "architecture": desc.architecture,
            "format": RootfsDescriptor.string(from: desc.format),
            "packageDigest": desc.packageDigestSHA256 ?? "",
            "sourceType": RootfsDescriptor.string(from: desc.sourceType),
            "state": RootfsDescriptor.string(from: desc.state),
            "valid": desc.isValidDescriptor()
        ]
    }

    private func listInstalledMaps() -> [[String: Any]] {
        return self.resolver.listInstalledRootfs().map { desc in
            [
                "version": desc.version,
                "architecture": desc.architecture,
                "format": RootfsDescriptor.string(from: desc.format),
                "packageDigest": desc.packageDigestSHA256 ?? "",
                "sourceType": RootfsDescriptor.string(from: desc.sourceType),
                "state": RootfsDescriptor.string(from: desc.state)
            ]
        }
    }

    private func handleInstall(args: [String: Any], result: @escaping FlutterResult) {
        guard let version = args["version"] as? String, !version.isEmpty else {
            result(FlutterError(code: "INVALID_REQUEST", message: "version required", details: self.safeDetails(nil)))
            return
        }
        let arch = (args["architecture"] as? String) ?? "aarch64"
        let request = RootfsInstallRequest()
        request.version = version
        request.architecture = arch
        request.expectedDigestSHA256 = (args["expectedDigestSHA256"] as? String) ?? ""
        request.forceReplace = (args["forceReplace"] as? Bool) ?? false
        request.allowCellularDownload = (args["allowCellularDownload"] as? Bool) ?? false
        request.maxArchiveBytes = (args["maxArchiveBytes"] as? Int64) ?? (512 * 1024 * 1024)

        if let asset = args["bundledAsset"] as? String, !asset.isEmpty {
            if let url = Bundle.main.url(forResource: asset, withExtension: "zip") {
                request.localBundleURL = url
            } else {
                result(FlutterError(code: "SOURCE_UNAVAILABLE", message: "bundled rootfs asset not found", details: self.safeDetails(nil)))
                return
            }
        } else if let remote = args["remoteURL"] as? String {
            if let url = URL(string: remote), url.scheme == "https" {
                request.remoteURL = url
            } else {
                result(FlutterError(code: "SOURCE_UNAVAILABLE", message: "invalid remoteURL", details: self.safeDetails(nil)))
                return
            }
        } else {
            result(FlutterError(code: "INVALID_REQUEST", message: "either bundledAsset or remoteURL required", details: self.safeDetails(nil)))
            return
        }

        self.installer.installRootfsWithRequest(request, progress: nil) { [weak self] success, res, err in
            if success, let r = res {
                result([
                    "version": r.descriptor.version,
                    "architecture": r.descriptor.architecture,
                    "packageDigest": r.descriptor.packageDigestSHA256 ?? "",
                    "state": RootfsDescriptor.string(from: r.descriptor.state),
                    "activated": r.activated,
                    "requiresSandboxRestart": r.requiresSandboxRestart
                ])
            } else {
                result(FlutterError(code: self?.errorCode(err) ?? "INSTALL_FAILED", message: err?.localizedDescription ?? "installation failed", details: self?.safeDetails(err)))
            }
        }
    }

    private func errorCode(_ err: NSError?) -> String {
        guard let err = err else { return "INSTALL_FAILED" }
        switch err.code {
        case RootfsInstallerErrorInvalidRequest.rawValue: return "INVALID_REQUEST"
        case RootfsInstallerErrorSourceUnavailable.rawValue: return "SOURCE_UNAVAILABLE"
        case RootfsInstallerErrorIntegrityMismatch.rawValue: return "INTEGRITY_MISMATCH"
        case RootfsInstallerErrorExtractionFailed.rawValue: return "EXTRACTION_FAILED"
        case RootfsInstallerErrorTraversalDetected.rawValue: return "TRAVERSAL_DETECTED"
        case RootfsInstallerErrorSymlinkEscapeDetected.rawValue: return "SYMLINK_ESCAPE"
        case RootfsInstallerErrorLayoutInvalid.rawValue: return "ROOTFS_INVALID"
        case RootfsInstallerErrorArchitectureMismatch.rawValue: return "ARCHITECTURE_MISMATCH"
        case RootfsInstallerErrorInsufficientStorage.rawValue: return "INSUFFICIENT_STORAGE"
        case RootfsInstallerErrorActivationFailed.rawValue: return "ACTIVATION_FAILED"
        case RootfsInstallerErrorCancelled.rawValue: return "CANCELLED"
        case RootfsInstallerErrorConcurrentInstallation.rawValue: return "CONCURRENT_INSTALLATION"
        case RootfsInstallerErrorVersionConflict.rawValue: return "VERSION_CONFLICT"
        default: return "INSTALL_FAILED"
        }
    }

    private func safeDetails(_ err: NSError?) -> [String: String]? {
        guard let err = err else { return nil }
        return [
            "step": "install",
            "code": "\(err.code)"
        ]
    }
}

class RootfsInstallEventHandler: NSObject, FlutterStreamHandler {
    private let installer: RootfsInstaller
    private var sink: FlutterEventSink?

    init(installer: RootfsInstaller) {
        self.installer = installer
    }

    func onListen(withArguments arguments: Any?, eventSink events: @escaping FlutterEventSink) -> FlutterError? {
        self.sink = events
        return nil
    }

    func onCancel(withArguments arguments: Any?) -> FlutterError? {
        self.sink = nil
        return nil
    }

    func emit(step: String, fraction: Double, message: String) {
        DispatchQueue.main.async {
            self.sink?([
                "installID": self.installer.currentInstallID ?? "",
                "step": step,
                "fraction": fraction,
                "message": message
            ])
        }
    }
}
