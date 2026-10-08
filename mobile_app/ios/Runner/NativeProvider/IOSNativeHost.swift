import Foundation
import ActivityKit
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


@objc public class IOSScreenAwakeNativeHandler: NSObject, IOSNativeOperationHandler {
    public let operations: Set<String> = ["display.keep_awake.status", "display.keep_awake.set"]
    private static let preferenceKey = "amitia.keep-screen-on.ios.v1"

    public func capabilitySnapshot() -> IOSNativeCapability {
        IOSNativeCapability(available: true, authorized: true)
    }

    @MainActor public static func apply(_ application: UIApplication, foreground: Bool) {
        application.isIdleTimerDisabled = foreground && UserDefaults.standard.bool(forKey: preferenceKey)
    }

    public func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard operations.contains(request.operation) else {
            return IOSNativeResponse(protocolVersion: request.protocolVersion, requestId: request.requestId, status: "error", result: nil, error: IOSNativeError(code: "OPERATION_NOT_SUPPORTED", message: "unsupported screen awake operation"))
        }
        if request.operation == "display.keep_awake.set", !(request.payload?["enabled"] is Bool) {
            return IOSNativeResponse(protocolVersion: request.protocolVersion, requestId: request.requestId, status: "error", result: nil, error: IOSNativeError(code: "INVALID_REQUEST", message: "enabled must be a boolean"))
        }
        return await MainActor.run {
            if let enabled = request.payload?["enabled"] as? Bool, request.operation == "display.keep_awake.set" {
                UserDefaults.standard.set(enabled, forKey: Self.preferenceKey)
            }
            Self.apply(UIApplication.shared, foreground: UIApplication.shared.applicationState == .active)
            return IOSNativeResponse(protocolVersion: request.protocolVersion, requestId: request.requestId, status: "success", result: ["enabled": UserDefaults.standard.bool(forKey: Self.preferenceKey)], error: nil)
        }
    }
}

@objc public class IOSLocalNotificationNativeHandler: NSObject, IOSNativeOperationHandler {
    public let operations: Set<String> = ["notification.status", "notification.request_permission", "notification.post", "notification.runtime_deliver"]
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
        case "notification.request_permission":
            return await handleRequestPermission(request)
        case "notification.post":
            return await handlePost(request)
        case "notification.runtime_deliver":
            return await handleRuntimeDeliver(request)
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

    private func handleRequestPermission(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        var settings = await currentSettings()
        cache(settings.authorizationStatus)
        if isAuthorized(settings.authorizationStatus) {
            return success(request, result: ["granted": true, "alreadyGranted": true, "canPost": true])
        }
        if settings.authorizationStatus == .denied {
            return error(request, code: "NOTIFICATION_POST_PERMISSION_REQUIRED", message: "notification permission was denied")
        }
        do {
            let granted = try await requestAuthorization()
            settings = await currentSettings()
            cache(settings.authorizationStatus)
            guard granted && isAuthorized(settings.authorizationStatus) else {
                return error(request, code: "NOTIFICATION_POST_PERMISSION_REQUIRED", message: "notification permission was not granted")
            }
            return success(request, result: ["granted": true, "alreadyGranted": false, "canPost": true])
        } catch {
            return self.error(request, code: "NOTIFICATION_POST_PERMISSION_REQUIRED", message: error.localizedDescription)
        }
    }

    private func handlePost(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let title = (request.payload?["title"] as? String ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
        let body = (request.payload?["body"] as? String ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
        let channel = (request.payload?["channel"] as? String ?? "amitia_agent").trimmingCharacters(in: .whitespacesAndNewlines)
        let silent = request.payload?["silent"] as? Bool ?? false
        let soundOnly = request.payload?["soundOnly"] as? Bool ?? false

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
        content.title = soundOnly ? "" : String(title.prefix(256))
        content.body = soundOnly ? "" : String(body.prefix(4096))
        content.categoryIdentifier = channel.isEmpty ? "amitia_agent" : String(channel.prefix(128))
        if !silent {
            content.sound = .default
        }

        let identifier = "amitia.local.\(UUID().uuidString)"
        if request.payload?["backgroundOnly"] as? Bool == true {
            let foreground = await MainActor.run { UIApplication.shared.applicationState != .background }
            if foreground { return success(request, result: ["posted": false, "reason": "app_foreground"]) }
        }
        let notificationRequest = UNNotificationRequest(identifier: identifier, content: content, trigger: nil)
        do {
            try await add(notificationRequest)
            return success(request, result: ["notificationRef": identifier, "posted": true])
        } catch {
            return self.error(request, code: "NOTIFICATION_POST_FAILED", message: error.localizedDescription)
        }
    }

    private func handleRuntimeDeliver(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let payload = request.payload ?? [:]
        let type = (payload["type"] as? String ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
        if type.hasPrefix("call."),
           let platform = IOSNotificationPlatform.active,
           let result = await platform.deliverLocalCall(type: type, payload: payload) {
            return success(request, result: result)
        }
        if type.hasPrefix("run."), #available(iOS 16.1, *) {
            if let response = await handleRuntimeActivity(request, payload: payload, type: type) {
                return response
            }
        }

        let settings = await currentSettings()
        cache(settings.authorizationStatus)
        guard isAuthorized(settings.authorizationStatus) else {
            return error(request, code: "NOTIFICATION_POST_DISABLED", message: "notifications are disabled for this app")
        }

        let content = UNMutableNotificationContent()
        content.title = String((payload["title"] as? String ?? "Amitia").prefix(256))
        content.body = String((payload["body"] as? String ?? payload["summary"] as? String ?? "").prefix(4096))
        content.threadIdentifier = (payload["conversationId"] as? String ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
        if type.hasPrefix("message.") {
            content.categoryIdentifier = "AMITIA_MESSAGE"
        } else if type.hasPrefix("reminder.") || type.hasPrefix("proactive.") {
            content.categoryIdentifier = "AMITIA_REMINDER"
        } else if type.hasPrefix("run.") {
            content.categoryIdentifier = "AMITIA_EXECUTION"
        }
        if payload["sound"] as? Bool != false {
            content.sound = .default
        }
        var userInfo: [String: Any] = [:]
        for key in ["notificationId", "type", "spaceId", "conversationId", "characterId", "messageId", "runId", "deepLink"] {
            if let value = payload[key] {
                userInfo[key] = value
            }
        }
        content.userInfo = userInfo
        let deliveredContent: UNNotificationContent
        if type.hasPrefix("message.") {
            deliveredContent = await communicationNotificationContent(
                base: content,
                payload: payload
            )
        } else {
            deliveredContent = content
        }
        let runId = (payload["runId"] as? String ?? "")
            .trimmingCharacters(in: .whitespacesAndNewlines)
        let identifier = type.hasPrefix("run.") && !runId.isEmpty
            ? "amitia.runtime.run." + runId
            : "amitia.runtime." + request.requestId
        if type.hasPrefix("run.") {
            UNUserNotificationCenter.current()
                .removeDeliveredNotifications(withIdentifiers: [identifier])
        }
        do {
            try await add(UNNotificationRequest(identifier: identifier, content: deliveredContent, trigger: nil))
            return success(request, result: [
                "posted": true,
                "notificationRef": identifier,
                "revision": int64Value(payload["revision"])
            ])
        } catch {
            return self.error(request, code: "NOTIFICATION_POST_FAILED", message: error.localizedDescription)
        }
    }

    private func communicationNotificationContent(
        base: UNMutableNotificationContent,
        payload: [String: Any]
    ) async -> UNNotificationContent {
        let previewMode = (payload["previewMode"] as? String ?? "full")
            .trimmingCharacters(in: .whitespacesAndNewlines)
        guard previewMode != "hidden" else {
            return base
        }
        let conversationId = (payload["conversationId"] as? String ?? "")
            .trimmingCharacters(in: .whitespacesAndNewlines)
        let characterId = (payload["characterId"] as? String ?? "")
            .trimmingCharacters(in: .whitespacesAndNewlines)
        let senderName = (payload["senderName"] as? String ?? base.title)
            .trimmingCharacters(in: .whitespacesAndNewlines)
        let body = base.body.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !conversationId.isEmpty, !senderName.isEmpty, !body.isEmpty else {
            return base
        }

        let handle = INPersonHandle(
            value: characterId.isEmpty ? senderName : characterId,
            type: .unknown
        )
        let sender = INPerson(
            personHandle: handle,
            nameComponents: nil,
            displayName: senderName,
            image: nil,
            contactIdentifier: nil,
            customIdentifier: characterId.isEmpty ? senderName : characterId
        )
        let intent = INSendMessageIntent(
            recipients: nil,
            outgoingMessageType: .outgoingMessageText,
            content: body,
            speakableGroupName: nil,
            conversationIdentifier: conversationId,
            serviceName: "Amitia",
            sender: sender,
            attachments: nil
        )
        let interaction = INInteraction(intent: intent, response: nil)
        interaction.direction = .incoming

        return await withCheckedContinuation {
            (continuation: CheckedContinuation<UNNotificationContent, Never>) in
            interaction.donate { _ in
                do {
                    let updated = try base.updating(from: intent)
                    if let merged = updated.mutableCopy() as? UNMutableNotificationContent {
                        merged.categoryIdentifier = base.categoryIdentifier
                        merged.threadIdentifier = base.threadIdentifier
                        merged.userInfo = base.userInfo
                        merged.sound = base.sound
                        merged.badge = base.badge
                        continuation.resume(returning: merged)
                    } else {
                        continuation.resume(returning: base)
                    }
                } catch {
                    continuation.resume(returning: base)
                }
            }
        }
    }

    @available(iOS 16.1, *)
    private func handleRuntimeActivity(
        _ request: IOSNativeRequest,
        payload: [String: Any],
        type: String
    ) async -> IOSNativeResponse? {
        let runId = (payload["runId"] as? String ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
        guard !runId.isEmpty else { return nil }
        let conversationId = (payload["conversationId"] as? String ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
        let characterId = (payload["characterId"] as? String ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
        let agentName = (payload["agentId"] as? String ?? "Amitia").trimmingCharacters(in: .whitespacesAndNewlines)
        let startedAt = isoTimestamp(payload["startedAt"]) ?? Int64(Date().timeIntervalSince1970)
        let state = AmitiaRunAttributes.ContentState(
            revision: int64Value(payload["revision"]),
            phase: (payload["phase"] as? String ?? "running"),
            title: (payload["title"] as? String ?? "Amitia 正在执行"),
            summary: (payload["summary"] as? String ?? ""),
            currentStep: intValue(payload["currentStep"]),
            totalSteps: intValue(payload["totalSteps"]),
            progress: doubleValue(payload["progress"]),
            updatedAt: isoTimestamp(payload["updatedAt"]) ?? Int64(Date().timeIntervalSince1970),
            locale: (payload["locale"] as? String)?.trimmingCharacters(in: .whitespacesAndNewlines),
            appearance: (payload["appearance"] as? String)?.trimmingCharacters(in: .whitespacesAndNewlines),
            agentName: agentName.isEmpty ? "Amitia" : agentName,
            totalTokens: intValue(payload["totalTokens"]) > 0
                ? intValue(payload["totalTokens"])
                : nil
        )
        let existing = Activity<AmitiaRunAttributes>.activities.first { $0.attributes.runId == runId }
        do {
            if type == "run.start" || type == "run.started" {
                if let existing {
                    if #available(iOS 16.2, *) {
                        await existing.update(activityContent(state))
                    } else {
                        await existing.update(using: state)
                    }
                    return success(request, result: ["posted": true, "notificationRef": existing.id, "revision": state.revision])
                }
                let attributes = AmitiaRunAttributes(
                    runId: runId,
                    conversationId: conversationId,
                    characterId: characterId,
                    agentName: agentName.isEmpty ? "Amitia" : agentName,
                    startedAt: startedAt
                )
                let activity: Activity<AmitiaRunAttributes>
                if #available(iOS 16.2, *) {
                    activity = try Activity<AmitiaRunAttributes>.request(
                        attributes: attributes,
                        content: activityContent(state),
                        pushType: nil
                    )
                } else {
                    activity = try Activity<AmitiaRunAttributes>.request(
                        attributes: attributes,
                        contentState: state,
                        pushType: nil
                    )
                }
                return success(request, result: ["posted": true, "notificationRef": activity.id, "revision": state.revision])
            }
            if type == "run.end" || type == "run.completed" || type == "run.failed" || type == "run.cancelled" || type == "run.interrupted" || type == "run.dismiss" {
                if let existing {
                    let immediate =
                        type == "run.dismiss" ||
                        type == "run.cancelled" ||
                        type == "run.interrupted" ||
                        state.phase == "cancelled" ||
                        state.phase == "interrupted"
                    let policy: ActivityUIDismissalPolicy = immediate
                        ? .immediate
                        : .after(Date().addingTimeInterval(60))
                    if #available(iOS 16.2, *) {
                        await existing.end(
                            activityContent(state, terminal: true),
                            dismissalPolicy: policy
                        )
                    } else {
                        await existing.end(using: state, dismissalPolicy: policy)
                    }
                    return success(request, result: ["posted": true, "notificationRef": existing.id, "revision": state.revision])
                }
                if type == "run.dismiss" {
                    return success(request, result: [
                        "posted": false,
                        "reason": "activity_not_active",
                        "revision": state.revision
                    ])
                }
                return nil
            }
            if let existing {
                if #available(iOS 16.2, *) {
                    await existing.update(activityContent(state))
                } else {
                    await existing.update(using: state)
                }
                return success(request, result: ["posted": true, "notificationRef": existing.id, "revision": state.revision])
            }
            return nil
        } catch {
            return nil
        }
    }

    @available(iOS 16.2, *)
    private func activityContent(
        _ state: AmitiaRunAttributes.ContentState,
        terminal: Bool = false
    ) -> ActivityContent<AmitiaRunAttributes.ContentState> {
        let score = terminal
            ? 0
            : Double(
                state.updatedAt > 0
                    ? state.updatedAt
                    : Int64(Date().timeIntervalSince1970)
            )
        return ActivityContent(
            state: state,
            staleDate: terminal ? nil : Date().addingTimeInterval(300),
            relevanceScore: score
        )
    }

    private func intValue(_ value: Any?) -> Int {
        if let value = value as? Int { return value }
        if let value = value as? Int64 { return Int(value) }
        if let value = value as? Double { return Int(value) }
        if let value = value as? String { return Int(value) ?? 0 }
        return 0
    }

    private func int64Value(_ value: Any?) -> Int64 {
        if let value = value as? Int64 { return value }
        if let value = value as? Int { return Int64(value) }
        if let value = value as? Double { return Int64(value) }
        if let value = value as? String { return Int64(value) ?? 0 }
        return 0
    }

    private func doubleValue(_ value: Any?) -> Double {
        if let value = value as? Double { return value }
        if let value = value as? Int { return Double(value) }
        if let value = value as? Int64 { return Double(value) }
        if let value = value as? String { return Double(value) ?? 0 }
        return 0
    }

    private func isoTimestamp(_ value: Any?) -> Int64? {
        guard let raw = value as? String, !raw.isEmpty else { return nil }
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        if let date = formatter.date(from: raw) {
            return Int64(date.timeIntervalSince1970)
        }
        formatter.formatOptions = [.withInternetDateTime]
        return formatter.date(from: raw).map { Int64($0.timeIntervalSince1970) }
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
