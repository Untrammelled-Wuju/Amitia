import Foundation
import UIKit
import HealthKit
import EventKit
import Contacts
import CoreBluetooth
import Photos
import AVFoundation
import BackgroundTasks
import Intents
import UserNotifications

@objc public protocol IOSNativeOperationHandler: AnyObject {
    var operations: Set<String> { get }
    func capabilitySnapshot() -> IOSNativeCapability
    func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse
}

@objc public class IOSNativeCapability: NSObject {
    public let available: Bool
    public let authorized: Bool
    public let hardwareAvailable: Bool
    public let platformSupported: Bool
    public let foregroundRequired: Bool

    public init(
        available: Bool,
        authorized: Bool = false,
        hardwareAvailable: Bool = true,
        platformSupported: Bool = true,
        foregroundRequired: Bool = false
    ) {
        self.available = available
        self.authorized = authorized
        self.hardwareAvailable = hardwareAvailable
        self.platformSupported = platformSupported
        self.foregroundRequired = foregroundRequired
        super.init()
    }
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

private let supportedProtocolVersions: Set<String> = ["1.0"]

@objc public class IOSNativeHost: NSObject {

    private var handlers: [String: IOSNativeOperationHandler] = [:]
    private var hostGeneration: UInt64 = 0
    private var isForeground: Bool = true
    private let queue = DispatchQueue(label: "com.amitia.iosnative.host", attributes: .concurrent)

    public private(set) var capabilities: [String: IOSNativeCapability] = [:]

    public override init() {
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
        refreshAuthorization()
    }

    @objc private func handleWillTerminate() {
        invalidateTransientRefs()
    }

    public func registerHandler(_ handler: IOSNativeOperationHandler) {
        queue.sync(flags: .barrier) {
            for operation in handler.operations {
                if self.handlers[operation] != nil {
                    preconditionFailure("IOSNativeHost: duplicate handler registration for operation \(operation)")
                }
                self.handlers[operation] = handler
            }
            self.updateCapabilitiesLocked()
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

                guard supportedProtocolVersions.contains(request.protocolVersion) else {
                    let error = IOSNativeError(
                        code: "BRIDGE_PROTOCOL_MISMATCH",
                        message: "unsupported protocol version: \(request.protocolVersion)",
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

                guard !request.requestID.isEmpty else {
                    let error = IOSNativeError(
                        code: "INVALID_ARGUMENT",
                        message: "requestID must not be empty",
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
            return [
                "platform": "ios",
                "protocolVersion": "1.0",
                "hostGeneration": hostGeneration,
                "osVersion": UIDevice.current.systemVersion,
                "deviceFamily": UIDevice.current.userInterfaceIdiom == .pad ? "ipad" : "iphone",
                "capabilities": buildCapabilityDictionary(),
                "foreground": isForeground
            ] as [String: Any]
        }
    }

    public func refreshAuthorization() {
        queue.async(flags: .barrier) {
            for (_, handler) in self.handlers {
                _ = handler.capabilitySnapshot()
            }
            self.updateCapabilitiesLocked()
        }
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

    public func invalidateTransientRefs() {
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

    private func updateCapabilitiesLocked() {
        var caps: [String: IOSNativeCapability] = [:]
        for (_, handler) in handlers {
            let snapshot = handler.capabilitySnapshot()
            for operation in handler.operations {
                let domain = operationDomain(operation)
                caps[domain] = snapshot
            }
        }
        capabilities = caps
    }

    private func buildCapabilityDictionary() -> [String: [String: Bool]] {
        var result: [String: [String: Bool]] = [:]
        for (domain, cap) in capabilities {
            result[domain] = [
                "available": cap.available,
                "authorized": cap.authorized,
                "hardwareAvailable": cap.hardwareAvailable,
                "platformSupported": cap.platformSupported,
                "foregroundRequired": cap.foregroundRequired
            ]
        }
        return result
    }

    private func operationDomain(_ operation: String) -> String {
        let parts = operation.split(separator: ".")
        if parts.count >= 2 {
            return "\(parts[0]).\(parts[1])"
        }
        return String(parts.first ?? "")
    }
}
