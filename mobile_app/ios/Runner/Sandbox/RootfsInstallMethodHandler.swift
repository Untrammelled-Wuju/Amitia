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
    }

    private func handleMethodCall(call: FlutterMethodCall, result: @escaping FlutterResult) {
        switch call.method {
        case "getCurrentRootfs":
            if let desc = self.resolver.resolveCurrentRootfs() {
                result([
                    "version": desc.version,
                    "architecture": desc.architecture,
                    "digest": desc.digestSHA256,
                    "sourceType": RootfsDescriptor.string(from: desc.sourceType),
                    "state": RootfsDescriptor.string(from: desc.state),
                    "valid": desc.isValidDescriptor()
                ])
            } else {
                result(nil)
            }
        case "isInstalled":
            if let args = call.arguments as? [String: Any],
               let version = args["version"] as? String,
               let arch = args["architecture"] as? String {
                result(self.resolver.isInstalledVersion(version, architecture: arch))
            } else {
                result(false)
            }
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
                    result(FlutterError(code: "VERIFY_FAILED", message: err?.localizedDescription ?? "rootfs verification failed", details: nil))
                }
            } else {
                result(FlutterError(code: "NOT_INSTALLED", message: "no rootfs installed", details: nil))
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

    private func handleInstall(args: [String: Any], result: @escaping FlutterResult) {
        result(FlutterError(code: "NOT_IMPLEMENTED", message: "Rootfs installation requires macOS build environment", details: nil))
    }
}
