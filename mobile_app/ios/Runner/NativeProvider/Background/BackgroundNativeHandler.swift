import Foundation
import BackgroundTasks
import UIKit

public class BackgroundNativeHandler: NSObject, IOSNativeOperationHandler {
    public let operations: Set<String> = [
        "background.status",
        "background.task.register",
        "background.task.submit",
        "background.task.cancel",
        "background.task.cancel_all",
        "background.task.get_pending",
        "background.task.progress",
        "background.task.expire",
        "background.task.complete",
        "background.task.reconcile",
        "background.runtime.readiness",
        "background.runtime.ensure",
        "background.checkpoint.get",
        "background.checkpoint.set",
        "background.checkpoint.clear",
        "background.binding.get"
    ]

    private static let checkpointKeyPrefix = "com.amitia.background.checkpoint."
    private var activeTaskRunIds: Set<String> = []
    private var registeredContinuedIdentifiers: Set<String> = []
    private let queue = DispatchQueue(label: "com.amitia.backgroundnative", attributes: .concurrent)
    private let checkpointLock = NSLock()

    public static func registerBGTaskHandlers() {
        if #available(iOS 13.0, *) {
            BGTaskScheduler.shared.register(forTaskWithIdentifier: "com.amitia.app.refresh", using: nil) { task in
                BackgroundTaskBridge.shared.handleBackgroundTask(task)
            }
            BGTaskScheduler.shared.register(forTaskWithIdentifier: "com.amitia.app.processing", using: nil) { task in
                BackgroundTaskBridge.shared.handleBackgroundTask(task)
            }
            BGTaskScheduler.shared.register(forTaskWithIdentifier: "com.amitia.app.cleanup", using: nil) { task in
                BackgroundTaskBridge.shared.handleBackgroundTask(task)
            }
        }
    }

    public override init() {
        super.init()
        BackgroundTaskBridge.shared.delegate = self
    }

    public func capabilitySnapshot() -> IOSNativeCapability {
        if #available(iOS 13.0, *) {
            return IOSNativeCapability(
                available: true,
                authorized: true,
                hardwareAvailable: true,
                platformSupported: true,
                foregroundRequired: false
            )
        }
        return IOSNativeCapability(
            available: false,
            authorized: false,
            hardwareAvailable: false,
            platformSupported: false,
            foregroundRequired: false
        )
    }

    public func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        switch request.operation {
        case "background.status":
            return await handleStatus(request)
        case "background.task.register":
            return await handleTaskRegister(request)
        case "background.task.submit":
            return await handleTaskSubmit(request)
        case "background.task.cancel":
            return await handleTaskCancel(request)
        case "background.task.cancel_all":
            return await handleTaskCancelAll(request)
        case "background.task.get_pending":
            return await handleTaskGetPending(request)
        case "background.task.progress":
            return await handleTaskProgress(request)
        case "background.task.expire":
            return handleTaskExpire(request)
        case "background.task.complete":
            return handleTaskComplete(request)
        case "background.task.reconcile":
            return await handleTaskReconcile(request)
        case "background.runtime.readiness":
            return await handleRuntimeReadiness(request)
        case "background.runtime.ensure":
            return handleRuntimeEnsure(request)
        case "background.checkpoint.get":
            return handleCheckpointGet(request)
        case "background.checkpoint.set":
            return await handleCheckpointSet(request)
        case "background.checkpoint.clear":
            return handleCheckpointClear(request)
        case "background.binding.get":
            return await handleBindingGet(request)
        default:
            return errorResponse(request, code: "OPERATION_NOT_SUPPORTED", message: "unsupported operation: \(request.operation)")
        }
    }

    private func handleStatus(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard #available(iOS 13.0, *) else {
            return successResponse(request, result: [
                "supported": false,
                "appRefreshSupported": false,
                "processingSupported": false,
                "continuedSupported": false,
                "pendingRequests": 0,
                "pendingCount": 0,
                "activeTaskCount": 0,
                "backgroundRefreshEnabled": "disabled",
                "runtimePersistence": "not_supported",
                "state": "disabled"
            ])
        }
        let pending = await BGTaskScheduler.shared.pendingTaskRequests()
        let activeCount = activeTaskCount()
        let refreshStatus = await MainActor.run {
            UIApplication.shared.backgroundRefreshStatus
        }
        let refreshValue: String
        switch refreshStatus {
        case .available:
            refreshValue = "enabled"
        case .denied:
            refreshValue = "disabled"
        case .restricted:
            refreshValue = "restricted"
        @unknown default:
            refreshValue = "limited"
        }
        let state =
            refreshValue == "disabled" || refreshValue == "restricted"
            ? "disabled"
            : (activeCount > 0 ? "busy" : "ready")
        return successResponse(request, result: [
            "supported": true,
            "appRefreshSupported": true,
            "processingSupported": true,
            "continuedSupported": ProcessInfo.processInfo.operatingSystemVersion.majorVersion >= 26,
            "pendingRequests": pending.count,
            "pendingCount": pending.count,
            "activeTaskCount": activeCount,
            "backgroundRefreshEnabled": refreshValue,
            "runtimePersistence": "supported",
            "state": state
        ])
    }

    private func handleTaskRegister(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard #available(iOS 13.0, *) else {
            return errorResponse(request, code: "PLATFORM_NOT_SUPPORTED", message: "iOS 13.0+ required")
        }
        let systemClass = (request.payload?["systemClass"] as? String ?? "app_refresh")
            .trimmingCharacters(in: .whitespacesAndNewlines)
        guard let canonicalIdentifier = await BGTaskIdentifierRegistry.shared.resolveIdentifier(
            systemClass: systemClass
        ) else {
            return errorResponse(
                request,
                code: "BACKGROUND_CLASS_NOT_SUPPORTED",
                message: "unsupported background systemClass: \(systemClass)"
            )
        }
        let logicalIdentifier = (request.payload?["identifier"] as? String ?? "")
            .trimmingCharacters(in: .whitespacesAndNewlines)
        return successResponse(request, result: [
            "success": true,
            "alreadyRegisteredAtLaunch": systemClass != "continued",
            "registeredAtSubmission": systemClass == "continued",
            "identifier": canonicalIdentifier,
            "logicalIdentifier": logicalIdentifier,
            "systemClass": systemClass
        ])
    }

    private func handleTaskSubmit(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let taskRunId = (request.payload?["taskRunId"] as? String ?? "")
            .trimmingCharacters(in: .whitespacesAndNewlines)
        guard !taskRunId.isEmpty else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing taskRunId")
        }
        let systemClass = (request.payload?["systemClass"] as? String ?? "app_refresh")
            .trimmingCharacters(in: .whitespacesAndNewlines)
        let logicalIdentifier = (request.payload?["identifier"] as? String ?? "")
            .trimmingCharacters(in: .whitespacesAndNewlines)

        if systemClass == "continued" {
            return await handleContinuedTaskSubmit(
                request,
                taskRunId: taskRunId,
                logicalIdentifier: logicalIdentifier
            )
        }

        guard #available(iOS 13.0, *) else {
            return errorResponse(request, code: "PLATFORM_NOT_SUPPORTED", message: "iOS 13.0+ required")
        }
        guard let resolvedIdentifier = await BGTaskIdentifierRegistry.shared.resolveIdentifier(
            systemClass: systemClass
        ), systemClass == "app_refresh" || systemClass == "processing" || systemClass == "cleanup" else {
            return errorResponse(
                request,
                code: "BACKGROUND_CLASS_NOT_SUPPORTED",
                message: "unsupported background systemClass: \(systemClass)"
            )
        }

        let bgRequest: BGTaskRequest
        if systemClass == "processing" || systemClass == "cleanup" {
            let procRequest = BGProcessingTaskRequest(identifier: resolvedIdentifier)
            if let earliestBeginAt = parseDate(request.payload?["earliestBeginAt"]) {
                procRequest.earliestBeginDate = earliestBeginAt
            }
            let defaultExternalPower = systemClass == "cleanup"
            procRequest.requiresExternalPower =
                request.payload?["externalPowerRequired"] as? Bool ?? defaultExternalPower
            procRequest.requiresNetworkConnectivity =
                request.payload?["networkRequired"] as? Bool ?? false
            bgRequest = procRequest
        } else {
            let refreshRequest = BGAppRefreshTaskRequest(identifier: resolvedIdentifier)
            if let earliestBeginAt = parseDate(request.payload?["earliestBeginAt"]) {
                refreshRequest.earliestBeginDate = earliestBeginAt
            }
            bgRequest = refreshRequest
        }

        await BGTaskIdentifierRegistry.shared.createMapping(
            taskRunId: taskRunId,
            systemClass: systemClass,
            identifier: resolvedIdentifier
        )
        do {
            try BGTaskScheduler.shared.submit(bgRequest)
            return successResponse(request, result: [
                "submitted": true,
                "taskRunId": taskRunId,
                "identifier": resolvedIdentifier,
                "logicalIdentifier": logicalIdentifier,
                "systemClass": systemClass
            ])
        } catch {
            await BGTaskIdentifierRegistry.shared.removeMapping(taskRunId: taskRunId)
            return errorResponse(request, code: "SUBMIT_FAILED", message: error.localizedDescription)
        }
    }

    private func handleContinuedTaskSubmit(
        _ request: IOSNativeRequest,
        taskRunId: String,
        logicalIdentifier: String
    ) async -> IOSNativeResponse {
        guard #available(iOS 26.0, *) else {
            return errorResponse(
                request,
                code: "CONTINUED_PROCESSING_UNAVAILABLE",
                message: "continued processing requires iOS 26.0+"
            )
        }

        let appIsActive = await MainActor.run {
            UIApplication.shared.applicationState == .active
        }
        guard appIsActive else {
            return errorResponse(
                request,
                code: "BACKGROUND_NOT_USER_INITIATED",
                message: "continued processing must be submitted while the app is foregrounded"
            )
        }

        let initiator = (request.payload?["initiator"] as? String ?? "")
            .trimmingCharacters(in: .whitespacesAndNewlines)
        let allowedInitiators = Set(["user", "foreground_shortcut", "explicit_app_intent"])
        guard allowedInitiators.contains(initiator) else {
            return errorResponse(
                request,
                code: "BACKGROUND_NOT_USER_INITIATED",
                message: "continued processing requires an explicit user initiator"
            )
        }

        let concreteIdentifier = continuedIdentifier(taskRunId: taskRunId)
        let needsRegistration = queue.sync(flags: .barrier) {
            if registeredContinuedIdentifiers.contains(concreteIdentifier) {
                return false
            }
            registeredContinuedIdentifiers.insert(concreteIdentifier)
            return true
        }
        if needsRegistration {
            let registered = await MainActor.run {
                BGTaskScheduler.shared.register(
                    forTaskWithIdentifier: concreteIdentifier,
                    using: nil
                ) { task in
                    BackgroundTaskBridge.shared.handleBackgroundTask(task)
                }
            }
            if !registered {
                queue.sync(flags: .barrier) {
                    registeredContinuedIdentifiers.remove(concreteIdentifier)
                }
                return errorResponse(
                    request,
                    code: "REGISTRATION_FAILED",
                    message: "failed to register continued processing task"
                )
            }
        }

        let title = nonEmpty(
            request.payload?["title"] as? String,
            fallback: "Amitia 正在执行任务"
        )
        let subtitle = nonEmpty(
            request.payload?["subtitle"] as? String,
            fallback: "任务可在后台继续"
        )
        let continuedRequest = BGContinuedProcessingTaskRequest(
            identifier: concreteIdentifier,
            title: String(title.prefix(64)),
            subtitle: String(subtitle.prefix(128))
        )
        let strategy = (request.payload?["strategy"] as? String ?? "queue_if_needed")
            .trimmingCharacters(in: .whitespacesAndNewlines)
        continuedRequest.strategy =
            strategy == "fail_if_not_immediate" ? .fail : .queue

        let gpuRequired = request.payload?["gpuRequired"] as? Bool ?? false
        if gpuRequired {
            guard BGTaskScheduler.supportedResources.contains(.gpu) else {
                return errorResponse(
                    request,
                    code: "BACKGROUND_GPU_UNAVAILABLE",
                    message: "background GPU access is unavailable on this device"
                )
            }
            continuedRequest.requiredResources = .gpu
        }

        await BGTaskIdentifierRegistry.shared.createMapping(
            taskRunId: taskRunId,
            systemClass: "continued",
            identifier: concreteIdentifier
        )
        do {
            try BGTaskScheduler.shared.submit(continuedRequest)
            return successResponse(request, result: [
                "submitted": true,
                "taskRunId": taskRunId,
                "identifier": concreteIdentifier,
                "logicalIdentifier": logicalIdentifier,
                "systemClass": "continued",
                "strategy": strategy,
                "gpuRequired": gpuRequired
            ])
        } catch {
            await BGTaskIdentifierRegistry.shared.removeMapping(taskRunId: taskRunId)
            return errorResponse(
                request,
                code: "SUBMIT_FAILED",
                message: error.localizedDescription
            )
        }
    }

    private func handleTaskCancel(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let taskRunId = request.payload?["taskRunId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing taskRunId")
        }
        if let identifier = await BGTaskIdentifierRegistry.shared.identifier(forTaskRunId: taskRunId),
           #available(iOS 13.0, *) {
            BGTaskScheduler.shared.cancel(taskRequestWithIdentifier: identifier)
        }
        let runningTaskEnded = await BackgroundTaskBridge.shared.cancelTaskRun(taskRunId)
        await BGTaskIdentifierRegistry.shared.removeMapping(taskRunId: taskRunId)
        markTaskInactive(taskRunId)
        return successResponse(request, result: [
            "cancelled": true,
            "taskRunId": taskRunId,
            "runningTaskEnded": runningTaskEnded
        ])
    }

    private func handleTaskCancelAll(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let systemClass = (request.payload?["systemClass"] as? String ?? "")
            .trimmingCharacters(in: .whitespacesAndNewlines)
        guard !systemClass.isEmpty else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing systemClass")
        }
        let mappings = await BGTaskIdentifierRegistry.shared.mappings(systemClass: systemClass)
        var runningEnded = 0
        for mapping in mappings {
            if #available(iOS 13.0, *) {
                BGTaskScheduler.shared.cancel(taskRequestWithIdentifier: mapping.identifier)
            }
            if await BackgroundTaskBridge.shared.cancelTaskRun(mapping.taskRunId) {
                runningEnded += 1
            }
            markTaskInactive(mapping.taskRunId)
        }
        await BGTaskIdentifierRegistry.shared.removeMappings(systemClass: systemClass)
        return successResponse(request, result: [
            "cancelled": true,
            "systemClass": systemClass,
            "cancelledCount": mappings.count,
            "runningEndedCount": runningEnded
        ])
    }

    private func handleTaskGetPending(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard #available(iOS 13.0, *) else {
            return errorResponse(request, code: "PLATFORM_NOT_SUPPORTED", message: "iOS 13.0+ required")
        }
        let filterClass = (request.payload?["systemClass"] as? String ?? "")
            .trimmingCharacters(in: .whitespacesAndNewlines)
        let requests = await BGTaskScheduler.shared.pendingTaskRequests()
        var pending: [[String: Any]] = []
        for taskRequest in requests {
            let mapping = await BGTaskIdentifierRegistry.shared.mappingForIdentifier(
                taskRequest.identifier
            )
            if !filterClass.isEmpty && mapping?.systemClass != filterClass {
                continue
            }
            var info: [String: Any] = ["identifier": taskRequest.identifier]
            if let mapping {
                info["taskRunId"] = mapping.taskRunId
                info["systemClass"] = mapping.systemClass
                info["generation"] = mapping.generation
                info["submittedAt"] = ISO8601DateFormatter().string(
                    from: mapping.submittedAt
                )
            }
            pending.append(info)
        }
        return successResponse(request, result: [
            "systemClass": filterClass,
            "pending": pending,
            "pendingCount": pending.count
        ])
    }

    private func handleTaskProgress(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let taskRunId = (request.payload?["taskRunId"] as? String ?? "")
            .trimmingCharacters(in: .whitespacesAndNewlines)
        guard !taskRunId.isEmpty,
              let completedUnits = int64(request.payload?["completedUnits"]),
              let totalUnits = int64(request.payload?["totalUnits"]),
              totalUnits > 0,
              completedUnits >= 0,
              completedUnits <= totalUnits else {
            return errorResponse(
                request,
                code: "INVALID_ARGUMENT",
                message: "invalid task progress parameters"
            )
        }
        let phase = (request.payload?["phase"] as? String ?? "")
            .trimmingCharacters(in: .whitespacesAndNewlines)
        let nativeProgressUpdated =
            await BackgroundTaskBridge.shared.updateTaskRunProgress(
                taskRunId,
                totalUnits: totalUnits,
                completedUnits: completedUnits,
                phase: phase
            )
        return successResponse(request, result: [
            "taskRunId": taskRunId,
            "completedUnits": completedUnits,
            "totalUnits": totalUnits,
            "phase": phase,
            "recorded": true,
            "nativeProgressUpdated": nativeProgressUpdated
        ])
    }

    private func handleTaskExpire(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let taskRunId = request.payload?["taskRunId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing taskRunId")
        }
        markTaskInactive(taskRunId)
        BackgroundTaskBridge.shared.markTaskRunExpired(taskRunId)
        return successResponse(request, result: [
            "expired": true,
            "taskRunId": taskRunId,
            "success": false
        ])
    }

    private func handleTaskComplete(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let taskRunId = request.payload?["taskRunId"] as? String,
              let success = request.payload?["success"] as? Bool else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing taskRunId or success flag")
        }
        markTaskInactive(taskRunId)
        BackgroundTaskBridge.shared.markTaskRunCompleted(taskRunId, success: success)
        return successResponse(request, result: [
            "completed": true,
            "taskRunId": taskRunId,
            "success": success
        ])
    }

    private func handleTaskReconcile(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let taskRunId = request.payload?["taskRunId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing taskRunId")
        }
        let active = isTaskActive(taskRunId)
        var scheduled = false
        if #available(iOS 13.0, *),
           let identifier = await BGTaskIdentifierRegistry.shared.identifier(forTaskRunId: taskRunId) {
            let pending = await BGTaskScheduler.shared.pendingTaskRequests()
            scheduled = pending.contains { $0.identifier == identifier }
        }
        return successResponse(request, result: [
            "taskRunId": taskRunId,
            "active": active,
            "scheduled": scheduled,
            "stillPending": active || scheduled
        ])
    }

    private func handleRuntimeReadiness(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard #available(iOS 13.0, *) else {
            return successResponse(request, result: [
                "ready": false,
                "pendingCount": 0,
                "activeTaskCount": 0,
                "canAcceptNewTask": false
            ])
        }
        let pending = await BGTaskScheduler.shared.pendingTaskRequests()
        let activeCount = activeTaskCount()
        return successResponse(request, result: [
            "ready": true,
            "pendingCount": pending.count,
            "activeTaskCount": activeCount,
            "canAcceptNewTask": pending.count + activeCount < 4
        ])
    }

    private func handleRuntimeEnsure(_ request: IOSNativeRequest) -> IOSNativeResponse {
        var available = false
        if #available(iOS 13.0, *) { available = true }
        return successResponse(request, result: [
            "ensured": available,
            "platformSupported": available
        ])
    }

    private func handleCheckpointGet(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let taskRunId = (request.payload?["taskRunId"] as? String ?? "")
            .trimmingCharacters(in: .whitespacesAndNewlines)
        guard !taskRunId.isEmpty else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing taskRunId")
        }
        checkpointLock.lock()
        let data = UserDefaults.standard.data(forKey: checkpointKey(taskRunId))
        checkpointLock.unlock()
        guard let data,
              let decoded = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else {
            return successResponse(request, result: [
                "taskRunId": taskRunId,
                "found": false
            ])
        }
        var result = decoded
        result["taskRunId"] = taskRunId
        result["found"] = true
        return successResponse(request, result: result)
    }

    private func handleCheckpointSet(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        let taskRunId = (request.payload?["taskRunId"] as? String ?? "")
            .trimmingCharacters(in: .whitespacesAndNewlines)
        guard !taskRunId.isEmpty else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing taskRunId")
        }
        let generation = int64(request.payload?["generation"]) ?? 0
        guard generation >= 0 else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "generation must be non-negative")
        }
        let lastUnit = int64(request.payload?["lastUnit"]) ?? 0
        let phase = (request.payload?["phase"] as? String ?? "")
            .trimmingCharacters(in: .whitespacesAndNewlines)
        var record: [String: Any] = [
            "generation": generation,
            "lastUnit": lastUnit,
            "phase": phase,
            "updatedAt": ISO8601DateFormatter().string(from: Date())
        ]
        if let checkpointData = request.payload?["checkpointData"] as? [String: Any],
           !checkpointData.isEmpty {
            record["checkpointData"] = checkpointData
        }
        guard JSONSerialization.isValidJSONObject(record),
              let encoded = try? JSONSerialization.data(withJSONObject: record) else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "checkpointData is not JSON serializable")
        }
        checkpointLock.lock()
        UserDefaults.standard.set(encoded, forKey: checkpointKey(taskRunId))
        checkpointLock.unlock()
        await BGTaskIdentifierRegistry.shared.updateGeneration(
            taskRunId: taskRunId,
            generation: generation
        )
        var result = record
        result["taskRunId"] = taskRunId
        return successResponse(request, result: result)
    }

    private func handleCheckpointClear(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let taskRunId = (request.payload?["taskRunId"] as? String ?? "")
            .trimmingCharacters(in: .whitespacesAndNewlines)
        guard !taskRunId.isEmpty else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing taskRunId")
        }
        checkpointLock.lock()
        UserDefaults.standard.removeObject(forKey: checkpointKey(taskRunId))
        checkpointLock.unlock()
        return successResponse(request, result: [
            "taskRunId": taskRunId,
            "cleared": true
        ])
    }

    private func handleBindingGet(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let taskRunId = request.payload?["taskRunId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing taskRunId")
        }
        let isActive = isTaskActive(taskRunId)
        var result: [String: Any] = [
            "taskRunId": taskRunId,
            "active": isActive
        ]
        if let mapping = await BGTaskIdentifierRegistry.shared.mappingForTaskRun(taskRunId) {
            result["identifier"] = mapping.identifier
            result["systemClass"] = mapping.systemClass
            result["submittedAt"] = ISO8601DateFormatter().string(from: mapping.submittedAt)
            result["generation"] = mapping.generation
        }
        return successResponse(request, result: result)
    }

    private func continuedIdentifier(taskRunId: String) -> String {
        let bundleID = Bundle.main.bundleIdentifier ?? "com.amitia.amitiaApp"
        var hash: UInt64 = 14_695_981_039_346_656_037
        for byte in taskRunId.utf8 {
            hash ^= UInt64(byte)
            hash &*= 1_099_511_628_211
        }
        return String(
            format: "%@.continued.%016llx",
            bundleID,
            hash
        )
    }

    private func nonEmpty(_ value: String?, fallback: String) -> String {
        let normalized = value?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        return normalized.isEmpty ? fallback : normalized
    }

    private func checkpointKey(_ taskRunId: String) -> String {
        let encoded = Data(taskRunId.utf8).base64EncodedString()
            .replacingOccurrences(of: "/", with: "_")
            .replacingOccurrences(of: "+", with: "-")
            .replacingOccurrences(of: "=", with: "")
        return Self.checkpointKeyPrefix + encoded
    }

    private func int64(_ value: Any?) -> Int64? {
        switch value {
        case let value as Int:
            return Int64(value)
        case let value as Int64:
            return value
        case let value as NSNumber:
            return value.int64Value
        case let value as String:
            return Int64(value.trimmingCharacters(in: .whitespacesAndNewlines))
        default:
            return nil
        }
    }

    private func parseDate(_ value: Any?) -> Date? {
        if let date = value as? Date {
            return date
        }
        if let timestamp = value as? Double {
            return Date(timeIntervalSince1970: timestamp)
        }
        if let timestamp = value as? Int {
            return Date(timeIntervalSince1970: TimeInterval(timestamp))
        }
        if let isoString = value as? String {
            let formatter = ISO8601DateFormatter()
            return formatter.date(from: isoString)
        }
        return nil
    }

    private func markTaskActive(_ taskRunId: String) {
        queue.async(flags: .barrier) {
            self.activeTaskRunIds.insert(taskRunId)
        }
    }

    private func markTaskInactive(_ taskRunId: String) {
        queue.async(flags: .barrier) {
            self.activeTaskRunIds.remove(taskRunId)
        }
    }

    private func clearAllActiveTasks() {
        queue.async(flags: .barrier) {
            self.activeTaskRunIds.removeAll()
        }
    }

    private func isTaskActive(_ taskRunId: String) -> Bool {
        return queue.sync { activeTaskRunIds.contains(taskRunId) }
    }

    private func activeTaskCount() -> Int {
        return queue.sync { activeTaskRunIds.count }
    }

    private func successResponse(_ request: IOSNativeRequest, result: [String: Any]) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: result,
            error: nil
        )
    }

    private func errorResponse(_ request: IOSNativeRequest, code: String, message: String) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "error",
            result: nil,
            error: IOSNativeError(code: code, message: message)
        )
    }
}

extension BackgroundNativeHandler: BackgroundTaskBridgeDelegate {
    public func backgroundTaskBridge(_ bridge: BackgroundTaskBridge, didReceiveTask task: BGTask) {
        Task {
            if let taskRunId = await BGTaskIdentifierRegistry.shared.taskRunId(forIdentifier: task.identifier) {
                markTaskActive(taskRunId)
            }
        }
    }

    public func backgroundTaskBridge(_ bridge: BackgroundTaskBridge, taskDidExpire task: BGTask) {
        Task {
            if let taskRunId = await BGTaskIdentifierRegistry.shared.taskRunId(forIdentifier: task.identifier) {
                markTaskInactive(taskRunId)
            }
        }
    }
}
