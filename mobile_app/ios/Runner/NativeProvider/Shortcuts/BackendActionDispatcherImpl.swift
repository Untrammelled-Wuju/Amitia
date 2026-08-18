import Foundation

public class BackendActionDispatcherImpl: BackendActionDispatcher {
    public static let shared = BackendActionDispatcherImpl()

    private var isReady: Bool = true
    private var nativeHost: IOSNativeHost?

    private init() {}

    public func configure(host: IOSNativeHost) {
        self.nativeHost = host
    }

    public func executeAction(actionId: String, payload: [String: Any]?) async -> [String: Any] {
        guard isReady else {
            return ["error": "BACKEND_DISPATCHER_NOT_READY", "actionId": actionId]
        }

        guard let host = nativeHost else {
            return ["error": "BACKEND_DISPATCHER_NOT_READY", "actionId": actionId]
        }

        let requestId = UUID().uuidString
        let operation = mapActionIdToOperation(actionId)

        let requestPayload: [String: Any] = [
            "actionId": actionId,
            "payload": payload ?? [:],
            "requestedAt": ISO8601DateFormatter().string(from: Date())
        ]

        let request = IOSNativeRequest(
            protocolVersion: 1,
            requestId: requestId,
            platform: "ios",
            operation: operation,
            payload: requestPayload
        )

        let response = await host.execute(request)

        if response.status == "ok" {
            return response.result ?? [:]
        } else {
            let errorCode = response.error?.code ?? "UNKNOWN_ERROR"
            let errorMessage = response.error?.message ?? "Action execution failed"
            return [
                "error": errorCode,
                "message": errorMessage,
                "actionId": actionId
            ]
        }
    }

    private func mapActionIdToOperation(_ actionId: String) -> String {
        if actionId.hasPrefix("com.amitia.action.chat") {
            return "shortcuts.execute.chat"
        }
        if actionId.hasPrefix("com.amitia.action.reminder") {
            return "shortcuts.execute.reminder"
        }
        if actionId.hasPrefix("com.amitia.action.alarm") {
            return "shortcuts.execute.alarm"
        }
        if actionId.hasPrefix("com.amitia.action.media") {
            return "shortcuts.execute.media"
        }
        return "shortcuts.execute.generic"
    }
}
