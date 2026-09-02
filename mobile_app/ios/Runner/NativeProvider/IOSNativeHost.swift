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
    public let protocolVersion: Int
    public let requestId: String
    public let platform: String
    public let operation: String
    public let payload: [String: Any]?

    public init(protocolVersion: Int, requestId: String, platform: String, operation: String, payload: [String: Any]?) {
        self.protocolVersion = protocolVersion
        self.requestId = requestId
        self.platform = platform
        self.operation = operation
        self.payload = payload
        super.init()
    }
}

@objc public class IOSNativeResponse: NSObject {
    public let protocolVersion: Int
    public let requestId: String
    public let status: String
    public let result: [String: Any]?
    public let error: IOSNativeError?

    public init(protocolVersion: Int, requestId: String, status: String, result: [String: Any]?, error: IOSNativeError?) {
        self.protocolVersion = protocolVersion
        self.requestId = requestId
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

private let supportedProtocolVersions: Set<Int> = [1]

@objc public class IOSNativeHost: NSObject {

    private var handlers: [String: IOSNativeOperationHandler] = [:]
    private var hostGeneration: UInt64 = 0
    private var isForeground: Bool = true
    private let queue = DispatchQueue(label: "com.amitia.iosnative.host", attributes: .concurrent)

    public private(set) var capabilities: [String: IOSNativeCapability] = [:]

    public override init() {
        super.init()
        setupNotifications()
        setupBackgroundTaskBridgeEventEmitter()
    }

    private func setupBackgroundTaskBridgeEventEmitter() {
        BackgroundTaskBridge.shared.eventEmitter = { [weak self] eventDict in
            guard let self = self else { return }
            let domain = eventDict["domain"] as? String ?? "background"
            let eventName = eventDict["event"] as? String ?? ""
            let timestamp = eventDict["timestamp"] as? String ?? ISO8601DateFormatter().string(from: Date())
            let data = eventDict["data"] as? [String: Any] ?? [:]

            let payload = NativeEventPayload(
                domain: domain,
                event: eventName,
                timestamp: timestamp,
                data: data,
                entityRef: data["identifier"] as? String ?? data["taskRunId"] as? String
            )
            NativeEventEmitter.shared.emit(payload)
        }
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
						requestId: request.requestId,
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
						requestId: request.requestId,
						status: "error",
						result: nil,
						error: error
					)
					continuation.resume(returning: response)
					return
				}

				guard !request.requestId.isEmpty else {
					let error = IOSNativeError(
						code: "INVALID_ARGUMENT",
						message: "requestId must not be empty",
						domainCode: nil
					)
					let response = IOSNativeResponse(
						protocolVersion: request.protocolVersion,
						requestId: request.requestId,
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
						requestId: request.requestId,
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
            var domains = Set<String>()
            for op in handlers.keys {
                let parts = op.split(separator: ".")
                if parts.count >= 2 {
                    domains.insert("\(parts[0]).\(parts[1])")
                } else if let first = parts.first {
                    domains.insert(String(first))
                }
            }

            return [
                "platform": "ios",
                "protocolVersion": 1,
                "hostGeneration": hostGeneration,
                "osVersion": UIDevice.current.systemVersion,
                "deviceFamily": UIDevice.current.userInterfaceIdiom == .pad ? "ipad" : "iphone",
                "capabilities": buildCapabilityDictionary(),
                "foreground": isForeground,
                "health": isForeground ? "ready" : "degraded",
                "registeredDomains": Array(domains)
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


@objc public class IOSLocalNotificationNativeHandler: NSObject, IOSNativeOperationHandler {
    public let operations: Set<String> = ["notification.status", "notification.post"]
    private let stateLock = NSLock()
    private var cachedAuthorizationStatus: UNAuthorizationStatus = .notDetermined

    public func capabilitySnapshot() -> IOSNativeCapability {
        stateLock.lock()
        let status = cachedAuthorizationStatus
        stateLock.unlock()
        let authorized = isAuthorized(status)
        return IOSNativeCapability(
            available: true,
            authorized: authorized,
            hardwareAvailable: true,
            platformSupported: true,
            foregroundRequired: false
        )
    }

    public func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        switch request.operation {
        case "notification.status":
            return await handleStatus(request)
        case "notification.post":
            return await handlePost(request)
        default:
            return error(request, code: "OPERATION_NOT_SUPPORTED", message: "unsupported operation: \(request.operation)")
        }
    }

    private func handleStatus(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let settings = await currentSettings()
        cache(settings.authorizationStatus)
        return success(request, result: [
            "authorized": isAuthorized(settings.authorizationStatus),
            "authorizationStatus": authorizationName(settings.authorizationStatus),
            "canPost": isAuthorized(settings.authorizationStatus)
        ])
    }

    private func handlePost(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let title = (request.payload?["title"] as? String ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
        let body = (request.payload?["body"] as? String ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
        let channel = (request.payload?["channel"] as? String ?? "amitia_agent").trimmingCharacters(in: .whitespacesAndNewlines)
        let silent = request.payload?["silent"] as? Bool ?? false

        guard !title.isEmpty || !body.isEmpty else {
            return error(request, code: "NOTIFICATION_POST_FAILED", message: "both title and body are empty")
        }

        var settings = await currentSettings()
        cache(settings.authorizationStatus)
        if settings.authorizationStatus == .notDetermined {
            do {
                let granted = try await requestAuthorization()
                guard granted else {
                    return error(request, code: "NOTIFICATION_POST_PERMISSION_REQUIRED", message: "notification permission was not granted")
                }
                settings = await currentSettings()
                cache(settings.authorizationStatus)
            } catch {
                return self.error(request, code: "NOTIFICATION_POST_PERMISSION_REQUIRED", message: error.localizedDescription)
            }
        }

        guard isAuthorized(settings.authorizationStatus) else {
            return error(request, code: "NOTIFICATION_POST_DISABLED", message: "notifications are disabled for this app")
        }

        let content = UNMutableNotificationContent()
        content.title = String(title.prefix(256))
        content.body = String(body.prefix(4096))
        content.categoryIdentifier = channel.isEmpty ? "amitia_agent" : String(channel.prefix(128))
        if !silent {
            content.sound = .default
        }

        let identifier = "amitia.local.\(UUID().uuidString)"
        let notificationRequest = UNNotificationRequest(identifier: identifier, content: content, trigger: nil)
        do {
            try await add(notificationRequest)
            return success(request, result: ["notificationRef": identifier, "posted": true])
        } catch {
            return self.error(request, code: "NOTIFICATION_POST_FAILED", message: error.localizedDescription)
        }
    }

    private func currentSettings() async -> UNNotificationSettings {
        await withCheckedContinuation { continuation in
            UNUserNotificationCenter.current().getNotificationSettings { settings in
                continuation.resume(returning: settings)
            }
        }
    }

    private func requestAuthorization() async throws -> Bool {
        try await withCheckedThrowingContinuation { continuation in
            UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .badge, .sound]) { granted, error in
                if let error = error {
                    continuation.resume(throwing: error)
                } else {
                    continuation.resume(returning: granted)
                }
            }
        }
    }

    private func add(_ request: UNNotificationRequest) async throws {
        try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
            UNUserNotificationCenter.current().add(request) { error in
                if let error = error {
                    continuation.resume(throwing: error)
                } else {
                    continuation.resume(returning: ())
                }
            }
        }
    }

    private func cache(_ status: UNAuthorizationStatus) {
        stateLock.lock()
        cachedAuthorizationStatus = status
        stateLock.unlock()
    }

    private func isAuthorized(_ status: UNAuthorizationStatus) -> Bool {
        return status.rawValue == UNAuthorizationStatus.authorized.rawValue
            || status.rawValue == UNAuthorizationStatus.provisional.rawValue
            || status.rawValue == 4
    }

    private func authorizationName(_ status: UNAuthorizationStatus) -> String {
        switch status.rawValue {
        case UNAuthorizationStatus.notDetermined.rawValue: return "notDetermined"
        case UNAuthorizationStatus.denied.rawValue: return "denied"
        case UNAuthorizationStatus.authorized.rawValue: return "authorized"
        case UNAuthorizationStatus.provisional.rawValue: return "provisional"
        case 4: return "ephemeral"
        default: return "unknown"
        }
    }

    private func success(_ request: IOSNativeRequest, result: [String: Any]) -> IOSNativeResponse {
        IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "success",
            result: result,
            error: nil
        )
    }

    private func error(_ request: IOSNativeRequest, code: String, message: String) -> IOSNativeResponse {
        IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "error",
            result: nil,
            error: IOSNativeError(code: code, message: message)
        )
    }
}


@objc public final class IOSDeviceTimeNativeHandler: NSObject, IOSNativeOperationHandler {
    public let operations: Set<String> = ["device.timezone.get"]

    public func capabilitySnapshot() -> IOSNativeCapability {
        IOSNativeCapability(
            available: true,
            authorized: true,
            hardwareAvailable: true,
            platformSupported: true,
            foregroundRequired: false
        )
    }

    public func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard request.operation == "device.timezone.get" else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(
                    code: "OPERATION_NOT_SUPPORTED",
                    message: "unknown device time operation: \(request.operation)"
                )
            )
        }
        let timezone = TimeZone.autoupdatingCurrent.identifier.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !timezone.isEmpty else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "TIMEZONE_UNAVAILABLE", message: "device timezone is unavailable")
            )
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: [
                "timezone": timezone,
                "ianaTimezone": timezone,
                "source": "ios.system"
            ],
            error: nil
        )
    }
}
