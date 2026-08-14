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

    public func capabilitySnapshot() -> IOSNativeCapability {
        let available = HKHealthStore.isHealthDataAvailable()
        return IOSNativeCapability(
            available: available,
            authorized: false,
            hardwareAvailable: available,
            platformSupported: available,
            foregroundRequired: false
        )
    }

    public func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        switch request.operation {
        case "health.authorization.status":
            return handleAuthorizationStatus(request)
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

    private func handleAuthorizationStatus(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard HKHealthStore.isHealthDataAvailable() else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: ["available": false, "authorized": false, "message": "HealthKit not available"],
                error: nil
            )
        }

        let readTypes = getReadTypes(from: request.payload)
        var authorizationResults: [String: String] = [:]
        var allAuthorized = true

        for type in readTypes {
            let status = healthStore.authorizationStatus(for: type)
            let statusString = authorizationStatusString(status)
            authorizationResults[type.identifier] = statusString
            if status != .sharingAuthorized {
                allAuthorized = false
            }
        }

        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["available": true, "authorized": allAuthorized, "types": authorizationResults],
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

        let readTypes = getReadTypes(from: request.payload)
        let writeTypes = getWriteTypes(from: request.payload)

        let readSet = Set(readTypes.compactMap { $0 as? HKObjectType })
        let writeSet = Set(writeTypes.compactMap { $0 as? HKSampleType })

        do {
            try await healthStore.requestAuthorization(toShare: writeSet, read: readSet)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: ["requested": true],
                error: nil
            )
        } catch {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "AUTHORIZATION_FAILED", message: error.localizedDescription)
            )
        }
    }

    private func handleProfileRead(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard HKHealthStore.isHealthDataAvailable() else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "HealthKit not available")
            )
        }

        do {
            let dob = try healthStore.dateOfBirthComponents()
            let biologicalSex = healthStore.biologicalSex()
            let bloodType = healthStore.bloodType()

            var profile: [String: Any] = [:]
            if let dob = dob {
                profile["dateOfBirth": "\(dob.year ?? 0)-\(dob.month ?? 0)-\(dob.day ?? 0)"]
            }
            profile["biologicalSex": biologicalSex.biologicalSex.rawValue]
            profile["bloodType": bloodType.bloodType.rawValue]

            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: ["profile": profile],
                error: nil
            )
        } catch {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "READ_FAILED", message: error.localizedDescription)
            )
        }
    }

    private func handleSamplesQuery(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard HKHealthStore.isHealthDataAvailable() else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "HealthKit not available")
            )
        }

        guard let typeIdentifier = request.payload?["type"] as? String,
              let sampleType = HKNampleTypeIdentifier(rawValue: typeIdentifier),
              let hkType = HKQuantityType.quantityType(forIdentifier: sampleType) else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "invalid sample type")
            )
        }

        let limit = (request.payload?["limit"] as? Int) ?? 100
        let startDate = (request.payload?["startDate"] as? Date) ?? Date.distantPast
        let endDate = (request.payload?["endDate"] as? Date) ?? Date()

        let predicate = HKQuery.predicateForSamples(withStart: startDate, end: endDate, options: .strictStartDate)
        let sortDescriptor = NSSortDescriptor(key: HKSampleSortIdentifierStartDate, ascending: false)

        return await withCheckedContinuation { continuation in
            let query = HKSampleQuery(sampleType: hkType, predicate: predicate, limit: limit, sortDescriptors: [sortDescriptor]) { _, samples, error in
                if let error = error {
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestID: request.requestID,
                        status: "error",
                        result: nil,
                        error: IOSNativeError(code: "QUERY_FAILED", message: error.localizedDescription)
                    ))
                    return
                }

                let hkSamples = samples as? [HKQuantitySample] ?? []
                let results = hkSamples.map { sample -> [String: Any] in
                    [
                        "id": sample.uuid.uuidString,
                        "type": sample.quantityType.identifier,
                        "startDate": sample.startDate.timeIntervalSince1970,
                        "endDate": sample.endDate.timeIntervalSince1970,
                        "value": sample.quantity.doubleValue(for: HKUnit(from: "count"))
                    ]
                }

                continuation.resume(returning: IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestID: request.requestID,
                    status: "ok",
                    result: ["samples": results, "count": results.count],
                    error: nil
                ))
            }
            healthStore.execute(query)
        }
    }

    private func handleStatisticsQuery(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard HKHealthStore.isHealthDataAvailable() else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "HealthKit not available")
            )
        }

        guard let typeIdentifier = request.payload?["type"] as? String,
              let sampleType = HKNampleTypeIdentifier(rawValue: typeIdentifier),
              let hkType = HKQuantityType.quantityType(forIdentifier: sampleType) else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "invalid quantity type")
            )
        }

        let options = HKStatisticsOptions(rawValue: request.payload?["options"] as? UInt ?? 0)
        let startDate = (request.payload?["startDate"] as? Date) ?? Date.distantPast
        let endDate = (request.payload?["endDate"] as? Date) ?? Date()

        let predicate = HKQuery.predicateForSamples(withStart: startDate, end: endDate, options: .strictStartDate)

        return await withCheckedContinuation { continuation in
            let query = HKStatisticsQuery(quantityType: hkType, quantitySamplePredicate: predicate, options: options) { _, statistics, error in
                if let error = error {
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestID: request.requestID,
                        status: "error",
                        result: nil,
                        error: IOSNativeError(code: "QUERY_FAILED", message: error.localizedDescription)
                    ))
                    return
                }

                var result: [String: Any] = [:]
                if let stats = statistics {
                    if let sum = stats.sumQuantity() {
                        result["sum"] = sum.doubleValue(for: HKUnit(from: "count"))
                    }
                    if let avg = stats.averageQuantity() {
                        result["average"] = avg.doubleValue(for: HKUnit(from: "count"))
                    }
                    if let min = stats.minimumQuantity() {
                        result["minimum"] = min.doubleValue(for: HKUnit(from: "count"))
                    }
                    if let max = stats.maximumQuantity() {
                        result["maximum"] = max.doubleValue(for: HKUnit(from: "count"))
                    }
                }

                continuation.resume(returning: IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestID: request.requestID,
                    status: "ok",
                    result: ["statistics": result],
                    error: nil
                ))
            }
            healthStore.execute(query)
        }
    }

    private func handleWorkoutsQuery(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard HKHealthStore.isHealthDataAvailable() else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "HealthKit not available")
            )
        }

        let limit = (request.payload?["limit"] as? Int) ?? 100
        let startDate = (request.payload?["startDate"] as? Date) ?? Date.distantPast
        let endDate = (request.payload?["endDate"] as? Date) ?? Date()

        let predicate = HKQuery.predicateForSamples(withStart: startDate, end: endDate, options: .strictStartDate)
        let sortDescriptor = NSSortDescriptor(key: HKSampleSortIdentifierStartDate, ascending: false)

        return await withCheckedContinuation { continuation in
            let query = HKSampleQuery(sampleType: .workoutType(), predicate: predicate, limit: limit, sortDescriptors: [sortDescriptor]) { _, samples, error in
                if let error = error {
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestID: request.requestID,
                        status: "error",
                        result: nil,
                        error: IOSNativeError(code: "QUERY_FAILED", message: error.localizedDescription)
                    ))
                    return
                }

                let workouts = samples as? [HKWorkout] ?? []
                let results = workouts.map { workout -> [String: Any] in
                    [
                        "id": workout.uuid.uuidString,
                        "activityType": workout.workoutActivityType.rawValue,
                        "duration": workout.duration,
                        "startDate": workout.startDate.timeIntervalSince1970,
                        "endDate": workout.endDate.timeIntervalSince1970
                    ]
                }

                continuation.resume(returning: IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestID: request.requestID,
                    status: "ok",
                    result: ["workouts": results, "count": results.count],
                    error: nil
                ))
            }
            healthStore.execute(query)
        }
    }

    private func handleWorkoutsDetail(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let workoutId = request.payload?["id"] as? String else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "INVALID_ARGUMENT", message: "missing workout id")
            )
        }

        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["workout": ["id": workoutId]],
            error: nil
        )
    }

    private func handleSleepQuery(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard HKHealthStore.isHealthDataAvailable() else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "HealthKit not available")
            )
        }

        guard let sleepType = HKSampleType.categoryType(forIdentifier: .sleepAnalysis) else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "sleep analysis not available")
            )
        }

        let limit = (request.payload?["limit"] as? Int) ?? 100
        let startDate = (request.payload?["startDate"] as? Date) ?? Date.distantPast
        let endDate = (request.payload?["endDate"] as? Date) ?? Date()

        let predicate = HKQuery.predicateForSamples(withStart: startDate, end: endDate, options: .strictStartDate)
        let sortDescriptor = NSSortDescriptor(key: HKSampleSortIdentifierStartDate, ascending: false)

        return await withCheckedContinuation { continuation in
            let query = HKSampleQuery(sampleType: sleepType, predicate: predicate, limit: limit, sortDescriptors: [sortDescriptor]) { _, samples, error in
                if let error = error {
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestID: request.requestID,
                        status: "error",
                        result: nil,
                        error: IOSNativeError(code: "QUERY_FAILED", message: error.localizedDescription)
                    ))
                    return
                }

                let sleepSamples = samples as? [HKCategorySample] ?? []
                let results = sleepSamples.map { sample -> [String: Any] in
                    [
                        "id": sample.uuid.uuidString,
                        "value": sample.value,
                        "startDate": sample.startDate.timeIntervalSince1970,
                        "endDate": sample.endDate.timeIntervalSince1970
                    ]
                }

                continuation.resume(returning: IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestID: request.requestID,
                    status: "ok",
                    result: ["sleep": results, "count": results.count],
                    error: nil
                ))
            }
            healthStore.execute(query)
        }
    }

    private func handleActivityQuery(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let activityType = HKQuantityType.quantityType(forIdentifier: .activeEnergyBurned) else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "PLATFORM_NOT_SUPPORTED", message: "activity type not available")
            )
        }

        let calendar = Calendar.current
        let now = Date()
        let startOfDay = calendar.startOfDay(for: now)

        let predicate = HKQuery.predicateForSamples(withStart: startOfDay, end: now, options: .strictStartDate)

        return await withCheckedContinuation { continuation in
            let query = HKStatisticsQuery(quantityType: activityType, quantitySamplePredicate: predicate, options: .cumulativeSum) { _, statistics, error in
                if let error = error {
                    continuation.resume(returning: IOSNativeResponse(
                        protocolVersion: request.protocolVersion,
                        requestID: request.requestID,
                        status: "error",
                        result: nil,
                        error: IOSNativeError(code: "QUERY_FAILED", message: error.localizedDescription)
                    ))
                    return
                }

                var result: [String: Any] = [:]
                if let stats = statistics, let sum = stats.sumQuantity() {
                    result["activeEnergyBurned"] = sum.doubleValue(for: .kilocalorie())
                }

                continuation.resume(returning: IOSNativeResponse(
                    protocolVersion: request.protocolVersion,
                    requestID: request.requestID,
                    status: "ok",
                    result: ["activity": result],
                    error: nil
                ))
            }
            healthStore.execute(query)
        }
    }

    private func getReadTypes(from payload: [String: Any]?) -> [HKObjectType] {
        guard let typeIdentifiers = payload?["readTypes"] as? [String] else {
            return [
                HKQuantityType.quantityType(forIdentifier: .stepCount),
                HKQuantityType.quantityType(forIdentifier: .activeEnergyBurned)
            ].compactMap { $0 }
        }

        return typeIdentifiers.compactMap { identifier in
            if let quantityType = HKQuantityTypeIdentifier(rawValue: identifier) {
                return HKQuantityType.quantityType(forIdentifier: quantityType)
            }
            return nil
        }
    }

    private func getWriteTypes(from payload: [String: Any]?) -> [HKSampleType] {
        guard let typeIdentifiers = payload?["writeTypes"] as? [String] else {
            return []
        }

        return typeIdentifiers.compactMap { identifier in
            if let quantityType = HKQuantityTypeIdentifier(rawValue: identifier) {
                return HKQuantityType.quantityType(forIdentifier: quantityType)
            }
            return nil
        }
    }

    private func authorizationStatusString(_ status: HKAuthorizationStatus) -> String {
        switch status {
        case .notDetermined: return "notDetermined"
        case .sharingDenied: return "denied"
        case .sharingAuthorized: return "authorized"
        @unknown default: return "unknown"
        }
    }
}
