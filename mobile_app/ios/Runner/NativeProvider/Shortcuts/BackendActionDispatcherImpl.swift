import Flutter
import Foundation

public class BackendActionDispatcherImpl: BackendActionDispatcher {
    public static let shared = BackendActionDispatcherImpl()

    private var channel: FlutterMethodChannel?

    private init() {}

    public func configure(messenger: FlutterBinaryMessenger) {
        channel = FlutterMethodChannel(
            name: "com.amitia.ios_native/backend_action",
            binaryMessenger: messenger
        )
    }

    public func executeAction(actionId: String, payload: [String: Any]?) async -> [String: Any] {
        guard let channel else {
            return [
                "status": "error",
                "error": [
                    "code": "BACKEND_DISPATCHER_NOT_READY",
                    "message": "Backend action channel is not configured"
                ],
                "actionId": actionId
            ]
        }

        return await withCheckedContinuation { continuation in
            DispatchQueue.main.async {
                channel.invokeMethod(
                    "backendAction.execute",
                    arguments: [
                        "actionId": actionId,
                        "payload": payload ?? [:]
                    ]
                ) { result in
                    if let flutterError = result as? FlutterError {
                        continuation.resume(returning: [
                            "status": "error",
                            "error": [
                                "code": flutterError.code,
                                "message": flutterError.message ?? "Backend action failed"
                            ],
                            "actionId": actionId
                        ])
                        return
                    }
                    guard let response = result as? [String: Any] else {
                        continuation.resume(returning: [
                            "status": "error",
                            "error": [
                                "code": "INVALID_RESPONSE",
                                "message": "Backend action returned an invalid response"
                            ],
                            "actionId": actionId
                        ])
                        return
                    }
                    continuation.resume(returning: response)
                }
            }
        }
    }
}
