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
            result(Int(self.bridge.health.lifecycleState.rawValue))
        case "start":
            guard let args = call.arguments as? [String: Any] else {
                result(FlutterError(code: "INVALID_ARGS", message: "start requires config arguments", details: nil))
                return
            }
            let config = self.parseConfig(from: args)
            self.bridge.start(withConfig: config) { success, err in
                if !success {
                    result(FlutterError(code: self.mapLifecycleErrorCode(err), message: err?.localizedDescription ?? "start failed", details: nil))
                } else {
                    result(self.bridge.lifecycleState.rawValue)
                }
            }
        case "stop":
            self.bridge.stop { success, err in
                if !success {
                    result(FlutterError(code: "STOP_FAILED", message: err?.localizedDescription ?? "stop failed", details: nil))
                } else {
                    result(true)
                }
            }
        case "restart":
            var reason = "user_restart"
            if let args = call.arguments as? [String: Any], let r = args["reason"] as? String, r.count > 0 {
                reason = r
            }
            self.bridge.restart(withReason: reason) { success, err in
                if !success {
                    result(FlutterError(code: self.mapLifecycleErrorCode(err), message: err?.localizedDescription ?? "restart failed", details: nil))
                } else {
                    result(true)
                }
            }
        case "execute":
            guard let args = call.arguments as? [String: Any] else {
                result(FlutterError(code: "INVALID_ARGS", message: "execute requires command arguments", details: nil))
                return
            }
            let command = self.parseCommand(from: args)
            var err: NSError?
            let execResult = self.bridge.executeCommand(command, error: &err)
            if let err = err, execResult.stale == false {
                result(FlutterError(code: self.mapExecErrorCode(err), message: err.localizedDescription, details: nil))
            } else {
                var response: [String: Any] = [
                    "stdout": execResult.stdout,
                    "stderr": execResult.stderr,
                    "exitCode": execResult.exitCode,
                    "generation": execResult.generation,
                    "executionID": execResult.executionID ?? NSNull(),
                    "stale": execResult.stale,
                ]
                if let error = execResult.error {
                    response["error"] = error
                }
                result(response)
            }
        case "health":
            result(self.serializeHealth(self.bridge.health))
        case "getLifecycle":
            result(self.serializeLifecycle(self.bridge.health))
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

    private func serializeHealth(_ health: ISHBridgeHealth) -> [String: Any] {
        return [
            "healthy": health.healthy,
            "message": health.message,
            "ishInitialized": health.ishInitialized,
            "rootfsInstalled": health.rootfsInstalled,
            "lifecycleState": Int(health.lifecycleState.rawValue),
            "lifecycleStateName": kIOSSandboxLifecycleStateName[health.lifecycleState],
            "generation": health.generation,
            "desiredRunning": health.desiredRunning,
            "restartRequired": health.restartRequired,
            "recoveryPending": health.recoveryPending,
            "activeExecutionID": health.activeExecutionID ?? NSNull(),
            "runningRootfsVersion": health.runningRootfsVersion ?? NSNull(),
            "runningRootfsDigest": health.runningRootfsDigest ?? NSNull(),
            "lastErrorCode": health.lastErrorCode ?? NSNull(),
        ]
    }

    private func serializeLifecycle(_ health: ISHBridgeHealth) -> [String: Any] {
        return [
            "state": kIOSSandboxLifecycleStateName[health.lifecycleState],
            "generation": health.generation,
            "desiredRunning": health.desiredRunning,
            "restartRequired": health.restartRequired,
            "recoveryPending": health.recoveryPending,
            "activeExecution": health.activeExecutionID != nil,
            "runningRootfsVersion": health.runningRootfsVersion ?? NSNull(),
            "lastErrorCode": health.lastErrorCode ?? NSNull(),
        ]
    }

    private func mapLifecycleErrorCode(_ err: NSError?) -> String {
        guard let err = err else { return "UNKNOWN" }
        if err.domain == kIOSSandboxBridgeErrorDomain {
            switch err.code {
            case IOSSandboxBridgeErrorRestartRequired: return "RESTART_REQUIRED"
            case IOSSandboxBridgeErrorLifecycleStarting: return "START_IN_PROGRESS"
            case IOSSandboxBridgeErrorLifecycleStopping: return "STOP_IN_PROGRESS"
            case IOSSandboxBridgeErrorLifecycleNotRunning: return "LIFECYCLE_NOT_RUNNING"
            case IOSSandboxBridgeErrorLifecycleQuiesced: return "LIFECYCLE_QUIESCED"
            case IOSSandboxBridgeErrorRuntimeFailed: return "RUNTIME_FAILED"
            default: return "START_FAILED"
            }
        }
        if err.domain == kAmitiaISHRuntimeErrorDomain {
            return "RUNTIME_FAILED"
        }
        return "START_FAILED"
    }

    private func mapExecErrorCode(_ err: NSError) -> String {
        if err.domain == kIOSSandboxBridgeErrorDomain {
            switch err.code {
            case IOSSandboxBridgeErrorStaleExecutionResult: return "STALE_EXECUTION_RESULT"
            case IOSSandboxBridgeErrorLifecycleStarting: return "LIFECYCLE_STARTING"
            case IOSSandboxBridgeErrorLifecycleStopping: return "LIFECYCLE_STOPPING"
            case IOSSandboxBridgeErrorLifecycleQuiesced: return "LIFECYCLE_QUIESCED"
            case IOSSandboxBridgeErrorRuntimeFailed: return "RUNTIME_FAILED"
            default: return "EXECUTE_FAILED"
            }
        }
        if err.domain == kAmitiaISHRuntimeErrorDomain {
            switch err.code {
            case AmitiaISHRuntimeErrorCodeExecTimeout: return "EXEC_TIMEOUT"
            case AmitiaISHRuntimeErrorCodeExecCancelled: return "EXEC_CANCELLED"
            default: return "EXECUTE_FAILED"
            }
        }
        return "EXECUTE_FAILED"
    }
}
