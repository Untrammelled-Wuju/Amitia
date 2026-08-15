import Foundation
import Flutter

final class IOSNativeBridgePlugin {
    static let channelName = "com.amitia.ios_native/bridge"

    private weak var host: IOSNativeHost?

    static func register(
        messenger: FlutterBinaryMessenger,
        host: IOSNativeHost
    ) {
        let plugin = IOSNativeBridgePlugin(host: host)
        let channel = FlutterMethodChannel(
            name: channelName,
            binaryMessenger: messenger
        )
        channel.setMethodCallHandler { call, result in
            plugin.handle(call: call, result: result)
        }
    }

    private init(host: IOSNativeHost) {
        self.host = host
    }

    private func handle(call: FlutterMethodCall, result: @escaping FlutterResult) {
        switch call.method {
        case "nativeBridge.health":
            handleHealth(call: call, result: result)
        case "nativeBridge.execute":
            handleExecute(call: call, result: result)
        default:
            result(FlutterMethodNotImplemented)
        }
    }

    private func handleHealth(call: FlutterMethodCall, result: @escaping FlutterResult) {
        guard let host = host else {
            result(["ready": false, "foreground": false])
            return
        }
        let handshake = host.handshake()
        var response: [String: Any] = [
            "ready": true,
            "foreground": host.foreground,
            "generation": host.currentGeneration
        ]
        if let health = handshake["health"] as? String {
            response["health"] = health
        }
        if let platform = handshake["platform"] as? String {
            response["platform"] = platform
        }
        result(response)
    }

    private func handleExecute(call: FlutterMethodCall, result: @escaping FlutterResult) {
        guard let host = host else {
            result(FlutterError(
                code: "HOST_UNAVAILABLE",
                message: "iOS Native Host not available",
                details: nil
            ))
            return
        }

        guard let args = call.arguments as? [String: Any] else {
            result(FlutterError(
                code: "INVALID_ARGUMENT",
                message: "execute requires a request payload",
                details: nil
            ))
            return
        }

        let protocolVersion = args["protocolVersion"] as? String ?? "1.0"
        let requestID = args["requestID"] as? String ?? ""
        let platform = args["platform"] as? String ?? "ios"
        let operation = args["operation"] as? String ?? ""
        let payload = args["payload"] as? [String: Any]

        guard !requestID.isEmpty else {
            result(FlutterError(
                code: "INVALID_ARGUMENT",
                message: "requestID must not be empty",
                details: nil
            ))
            return
        }

        guard !operation.isEmpty else {
            result(FlutterError(
                code: "INVALID_ARGUMENT",
                message: "operation must not be empty",
                details: nil
            ))
            return
        }

        let request = IOSNativeRequest(
            protocolVersion: protocolVersion,
            requestID: requestID,
            platform: platform,
            operation: operation,
            payload: payload
        )

        Task {
            let response = await host.execute(request)
            let map = self.serializeResponse(response)
            result(map)
        }
    }

    private func serializeResponse(_ response: IOSNativeResponse) -> [String: Any] {
        var map: [String: Any] = [
            "protocolVersion": response.protocolVersion,
            "requestID": response.requestID,
            "status": response.status
        ]
        if let result = response.result {
            map["result"] = result
        } else {
            map["result"] = NSNull()
        }
        if let error = response.error {
            var errorMap: [String: Any] = [
                "code": error.code,
                "message": error.message
            ]
            if let domainCode = error.domainCode {
                errorMap["domainCode"] = domainCode
            }
            map["error"] = errorMap
        } else {
            map["error"] = NSNull()
        }
        return map
    }
}
