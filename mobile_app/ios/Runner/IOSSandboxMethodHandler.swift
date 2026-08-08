import Flutter
import UIKit

class IOSSandboxMethodHandler: NSObject {
    private let bridge: IOSSandboxBridge

    init(bridge: IOSSandboxBridge) {
        self.bridge = bridge
        super.init()
    }

    func register(with registrar: FlutterPluginRegistrar) {
        let channel = FlutterMethodChannel(name: "com.amitia.ios_sandbox", binaryMessenger: registrar.messenger())
        channel.setMethodCallHandler { [weak self] call, result in
            self?.handleMethodCall(call: call, result: result)
        }
    }

    private func handleMethodCall(call: FlutterMethodCall, result: @escaping FlutterResult) {
        switch call.method {
        case "getAvailability":
            result(Int(self.bridge.availability.rawValue))
        case "start":
            guard let args = call.arguments as? [String: Any] else {
                result(FlutterError(code: "INVALID_ARGS", message: "start requires config arguments", details: nil))
                return
            }
            let config = self.parseConfig(from: args)
            var err: NSError?
            let ok = self.bridge.startWithConfig(config, error: &err)
            if !ok {
                result(FlutterError(code: "START_FAILED", message: err?.localizedDescription ?? "failed to start iSH backend", details: nil))
            } else {
                result(true)
            }
        case "stop":
            self.bridge.stop()
            result(true)
        case "execute":
            guard let args = call.arguments as? [String: Any] else {
                result(FlutterError(code: "INVALID_ARGS", message: "execute requires command arguments", details: nil))
                return
            }
            let command = self.parseCommand(from: args)
            var err: NSError?
            let execResult = self.bridge.executeCommand(command, error: &err)
            if let err = err {
                result(FlutterError(code: "EXECUTE_FAILED", message: err.localizedDescription, details: nil))
            } else {
                result([
                    "stdout": execResult.stdout,
                    "stderr": execResult.stderr,
                    "exitCode": execResult.exitCode,
                    "error": execResult.error ?? NSNull()
                ])
            }
        case "health":
            let health = self.bridge.health()
            result([
                "healthy": health.healthy,
                "message": health.message,
                "ishInitialized": health.ishInitialized,
                "rootfsInstalled": health.rootfsInstalled
            ])
        default:
            result(FlutterMethodNotImplemented)
        }
    }

    private func parseConfig(from dict: [String: Any]) -> ISHBridgeConfig {
        let config = ISHBridgeConfig()
        config.runtimeID = dict["runtimeID"] as? String
        config.workspaceURI = dict["workspaceURI"] as? String
        config.rootfsURI = dict["rootfsURI"] as? String
        config.environment = dict["environment"] as? [String: String]
        return config
    }

    private func parseCommand(from dict: [String: Any]) -> ISHBridgeCommand {
        let command = ISHBridgeCommand()
        command.command = dict["command"] as? [String] ?? []
        command.stdin = dict["stdin"] as? String
        command.timeout = dict["timeout"] as? Int ?? 0
        command.workDir = dict["workDir"] as? String
        return command
    }
}
