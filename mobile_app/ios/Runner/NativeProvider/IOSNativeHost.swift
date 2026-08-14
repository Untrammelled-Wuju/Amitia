import Foundation
import UIKit

@objc public protocol IOSNativeOperationHandler: AnyObject {
    var operations: Set<String> { get }
    func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse
}

@objc public class IOSNativeRequest: NSObject {
    public let protocolVersion: String
    public let requestID: String
    public let platform: String
    public let operation: String
    public let payload: [String: Any]?

    public init(protocolVersion: String, requestID: String, platform: String, operation: String, payload: [String: Any]?) {
        self.protocolVersion = protocolVersion
        self.requestID = requestID
        self.platform = platform
        self.operation = operation
        self.payload = payload
        super.init()
    }
}

@objc public class IOSNativeResponse: NSObject {
    public let protocolVersion: String
    public let requestID: String
    public let status: String
    public let result: [String: Any]?
    public let error: IOSNativeError?

    public init(protocolVersion: String, requestID: String, status: String, result: [String: Any]?, error: IOSNativeError?) {
        self.protocolVersion = protocolVersion
        self.requestID = requestID
        self.status = status
        self.result = result
        self.error = error
        super.init()
    }
}

@objc public class IOSNativeError: NSObject {
    public let code: String
    public let message: String
    public let domainCode: String?

    public init(code: String, message: String, domainCode: String? = nil) {
        self.code = code
        self.message = message
        self.domainCode = domainCode
        super.init()
    }
}

@objc public class IOSNativeHost: NSObject {

    private var handlers: [String: IOSNativeOperationHandler] = [:]
    private var hostGeneration: UInt64 = 0
    private var isForeground: Bool = true
    private let queue = DispatchQueue(label: "com.amitia.iosnative.host", attributes: .concurrent)

    public init(bridge: AnyObject) {
        super.init()
        setupNotifications()
    }

    private override init() {
        super.init()
        setupNotifications()
    }

    private func setupNotifications() {
        NotificationCenter.default.addObserver(
            self,
            selector: #selector(handleDidEnterBackground),
            name: UIApplication.didEnterBackgroundNotification,
            object: nil
        )
        NotificationCenter.default.addObserver(
            self,
            selector: #selector(handleWillEnterForeground),
            name: UIApplication.willEnterForegroundNotification,
            object: nil
        )
        NotificationCenter.default.addObserver(
            self,
            selector: #selector(handleWillTerminate),
            name: UIApplication.willTerminateNotification,
            object: nil
        )
    }

    @objc private func handleDidEnterBackground() {
        queue.async(flags: .barrier) {
            self.isForeground = false
        }
    }

    @objc private func handleWillEnterForeground() {
        queue.async(flags: .barrier) {
            self.isForeground = true
        }
    }

    @objc private func handleWillTerminate() {
        invalidateTransientRefs()
    }

    public func registerHandler(_ handler: IOSNativeOperationHandler) {
        queue.async(flags: .barrier) {
            for operation in handler.operations {
                if self.handlers[operation] != nil {
                    preconditionFailure("IOSNativeHost: duplicate handler registration for operation \(operation)")
                }
                self.handlers[operation] = handler
            }
        }
    }

    public func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return await withCheckedContinuation { continuation in
            queue.async {
                guard request.platform == "ios" else {
                    let error = IOSNativeError(
                        code: "INVALID_PLATFORM",
                        message: "unsupported platform: \(request.platform)",
                        domainCode: nil
                    )
                    let response = IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestID: request.requestID,
                        status: "error",
                        result: nil,
                        error: error
                    )
                    continuation.resume(returning: response)
                    return
                }

                guard let handler = self.handlers[request.operation] else {
                    let error = IOSNativeError(
                        code: "OPERATION_NOT_SUPPORTED",
                        message: "operation not supported: \(request.operation)",
                        domainCode: nil
                    )
                    let response = IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestID: request.requestID,
                        status: "error",
                        result: nil,
                        error: error
                    )
                    continuation.resume(returning: response)
                    return
                }

                Task {
                    let response = await handler.execute(request)
                    continuation.resume(returning: response)
                }
            }
        }
    }

    public func handshake() -> [String: Any] {
        return queue.sync {
            let capabilities = [
                "health": true,
                "calendar": true,
                "reminders": true,
                "contacts": true,
                "homekit": true,
                "bluetooth": true,
                "clipboard": true,
                "media": true,
                "alarms": true,
                "share": true,
                "shortcuts": true,
                "background": true,
                "file": true
            ]
            return [
                "platform": "ios",
                "protocolVersion": "1.0",
                "hostGeneration": hostGeneration,
                "osVersion": UIDevice.current.systemVersion,
                "deviceFamily": UIDevice.current.userInterfaceIdiom == .pad ? "ipad" : "iphone",
                "capabilities": capabilities,
                "foreground": isForeground
            ] as [String: Any]
        }
    }

    public func refreshAuthorization() {
    }

    public func didEnterBackground() {
        queue.async(flags: .barrier) {
            self.isForeground = false
        }
    }

    public func willEnterForeground() {
        queue.async(flags: .barrier) {
            self.isForeground = true
        }
        refreshAuthorization()
    }

    public func willTerminate() {
        invalidateTransientRefs()
    }

    private func invalidateTransientRefs() {
        queue.async(flags: .barrier) {
            self.hostGeneration += 1
        }
    }

    public var currentGeneration: UInt64 {
        return queue.sync { hostGeneration }
    }

    public var foreground: Bool {
        return queue.sync { isForeground }
    }
}
