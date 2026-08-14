import Foundation
import HealthKit

public class HealthKitNativeHandler: NSObject, IOSNativeOperationHandler {
    public let operations: Set<String> = [
        "health.authorization.status",
        "health.authorization.request",
        "health.profile.read",
        "health.samples.query",
        "health.statistics.query",
        "health.workouts.query",
        "health.workouts.detail",
        "health.sleep.query",
        "health.activity.query"
    ]

    private let healthStore = HKHealthStore()

    public override init() {
        super.init()
    }

    public func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        switch request.operation {
        case "health.authorization.status":
            return await handleAuthorizationStatus(request)
        case "health.authorization.request":
            return await handleAuthorizationRequest(request)
        case "health.profile.read":
            return await handleProfileRead(request)
        case "health.samples.query":
            return await handleSamplesQuery(request)
        case "health.statistics.query":
            return await handleStatisticsQuery(request)
        case "health.workouts.query":
            return await handleWorkoutsQuery(request)
        case "health.workouts.detail":
            return await handleWorkoutsDetail(request)
        case "health.sleep.query":
            return await handleSleepQuery(request)
        case "health.activity.query":
            return await handleActivityQuery(request)
        default:
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "OPERATION_NOT_SUPPORTED", message: "unsupported operation: \(request.operation)")
            )
        }
    }

    private func handleAuthorizationStatus(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard HKHealthStore.isHealthDataAvailable() else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: ["available": false, "authorized": false, "message": "HealthKit not available"],
                error: nil
            )
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["available": true, "authorized": true, "message": "HealthKit available"],
            error: nil
        )
    }

    private func handleAuthorizationRequest(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard HKHealthStore.isHealthDataAvailable() else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "HealthKit not available")
            )
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["requested": true],
            error: nil
        )
    }

    private func handleProfileRead(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["profile": [:]],
            error: nil
        )
    }

    private func handleSamplesQuery(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["samples": []],
            error: nil
        )
    }

    private func handleStatisticsQuery(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["statistics": []],
            error: nil
        )
    }

    private func handleWorkoutsQuery(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["workouts": []],
            error: nil
        )
    }

    private func handleWorkoutsDetail(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["workout": [:]],
            error: nil
        )
    }

    private func handleSleepQuery(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["sleep": []],
            error: nil
        )
    }

    private func handleActivityQuery(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["activity": []],
            error: nil
        )
    }
}
