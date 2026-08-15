import Foundation
import HomeKit

public class HomeKitNativeHandler: NSObject, IOSNativeOperationHandler {
    public let operations: Set<String> = [
        "homekit.status",
        "homekit.authorization.status",
        "homekit.homes.list",
        "homekit.homes.get",
        "homekit.rooms.list",
        "homekit.zones.list",
        "homekit.accessories.list",
        "homekit.accessories.get",
        "homekit.services.list",
        "homekit.characteristics.list",
        "homekit.characteristics.read",
        "homekit.characteristics.write",
        "homekit.scenes.list",
        "homekit.scenes.get",
        "homekit.scenes.execute",
        "homekit.action_sets.list",
        "homekit.action_sets.get",
        "homekit.action_sets.execute",
        "homekit.automations.list"
    ]

    private let store = HomeKitStore.shared

    public override init() {
        super.init()
        store.initialize()
    }

    public func capabilitySnapshot() -> IOSNativeCapability {
        let status = homeKitAuthorizationStatusUnchecked()
        let authorized = status == .authorized
        let available = status != .restricted
        return IOSNativeCapability(
            available: available,
            authorized: authorized,
            hardwareAvailable: true,
            platformSupported: true,
            foregroundRequired: true
        )
    }

    private func ensureAuthorized(_ request: IOSNativeRequest) -> IOSNativeResponse? {
        let status = homeKitAuthorizationStatusUnchecked()
        if status == .restricted {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "HOMEKIT_RESTRICTED", message: "HomeKit access is restricted by system policy")
            )
        }
        if status == .notDetermined {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "HOMEKIT_NOT_DETERMINED", message: "HomeKit authorization not yet determined")
            )
        }
        return nil
    }

    public func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        switch request.operation {
        case "homekit.status":
            return handleStatus(request)
        case "homekit.authorization.status":
            return handleAuthorizationStatus(request)
        case "homekit.homes.list":
            return handleHomesList(request)
        case "homekit.homes.get":
            return handleHomesGet(request)
        case "homekit.rooms.list":
            return handleRoomsList(request)
        case "homekit.zones.list":
            return handleZonesList(request)
        case "homekit.accessories.list":
            return handleAccessoriesList(request)
        case "homekit.accessories.get":
            return handleAccessoriesGet(request)
        case "homekit.services.list":
            return handleServicesList(request)
        case "homekit.characteristics.list":
            return handleCharacteristicsList(request)
        case "homekit.characteristics.read":
            return await handleCharacteristicsRead(request)
        case "homekit.characteristics.write":
            return await handleCharacteristicsWrite(request)
        case "homekit.scenes.list", "homekit.action_sets.list":
            return handleActionSetsList(request)
        case "homekit.scenes.get", "homekit.action_sets.get":
            return handleActionSetsGet(request)
        case "homekit.scenes.execute", "homekit.action_sets.execute":
            return await handleActionSetsExecute(request)
        case "homekit.automations.list":
            return handleAutomationsList(request)
        default:
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "OPERATION_NOT_SUPPORTED", message: "unsupported operation: \(request.operation)")
            )
        }
    }

    private func handleStatus(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let status = homeKitAuthorizationStatusUnchecked()
        let authorized = status == .authorized
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: [
                "available": true,
                "authorized": authorized,
                "authorizationStatus": authorizationStatusString(status),
                "generation": store.currentGeneration
            ],
            error: nil
        )
    }

    private func handleAuthorizationStatus(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let status = homeKitAuthorizationStatusUnchecked()
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: [
                "authorized": status == .authorized,
                "authorizationStatus": authorizationStatusString(status)
            ],
            error: nil
        )
    }

    private func handleHomesList(_ request: IOSNativeRequest) -> IOSNativeResponse {
        if let rejection = ensureAuthorized(request) { return rejection }
        let generation = store.currentGeneration
        let homes = store.allHomes().map { home in
            [
                "id": home.uniqueIdentifier.uuidString,
                "name": home.name,
                "isPrimary": home.isPrimary,
                "generation": generation
            ] as [String: Any]
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: ["homes": homes, "count": homes.count],
            error: nil
        )
    }

    private func handleHomesGet(_ request: IOSNativeRequest) -> IOSNativeResponse {
        if let rejection = ensureAuthorized(request) { return rejection }
        guard let homeId = request.payload?["id"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing home id")
        }
        guard let home = store.home(withID: homeId) else {
            return errorResponse(request, code: "NOT_FOUND", message: "home not found: \(homeId)")
        }
        let generation = store.currentGeneration
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: [
                "home": [
                    "id": home.uniqueIdentifier.uuidString,
                    "name": home.name,
                    "isPrimary": home.isPrimary,
                    "generation": generation
                ]
            ],
            error: nil
        )
    }

    private func handleRoomsList(_ request: IOSNativeRequest) -> IOSNativeResponse {
        if let rejection = ensureAuthorized(request) { return rejection }
        guard let homeId = request.payload?["homeId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing homeId")
        }
        guard let home = store.home(withID: homeId) else {
            return errorResponse(request, code: "NOT_FOUND", message: "home not found: \(homeId)")
        }
        let generation = store.currentGeneration
        let rooms = home.rooms.map { room in
            [
                "id": room.uniqueIdentifier.uuidString,
                "name": room.name,
                "homeId": homeId,
                "generation": generation
            ] as [String: Any]
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: ["rooms": rooms, "count": rooms.count],
            error: nil
        )
    }

    private func handleZonesList(_ request: IOSNativeRequest) -> IOSNativeResponse {
        if let rejection = ensureAuthorized(request) { return rejection }
        guard let homeId = request.payload?["homeId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing homeId")
        }
        guard let home = store.home(withID: homeId) else {
            return errorResponse(request, code: "NOT_FOUND", message: "home not found: \(homeId)")
        }
        let generation = store.currentGeneration
        let zones = home.zones.map { zone in
            [
                "id": zone.uniqueIdentifier.uuidString,
                "name": zone.name,
                "homeId": homeId,
                "generation": generation
            ] as [String: Any]
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: ["zones": zones, "count": zones.count],
            error: nil
        )
    }

    private func handleAccessoriesList(_ request: IOSNativeRequest) -> IOSNativeResponse {
        if let rejection = ensureAuthorized(request) { return rejection }
        guard let homeId = request.payload?["homeId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing homeId")
        }
        guard let home = store.home(withID: homeId) else {
            return errorResponse(request, code: "NOT_FOUND", message: "home not found: \(homeId)")
        }
        let generation = store.currentGeneration
        let accessories = home.accessories.map { accessory in
            [
                "id": accessory.uniqueIdentifier.uuidString,
                "name": accessory.name,
                "category": accessory.category.localizedDescription,
                "isReachable": accessory.isReachable,
                "isBlocked": accessory.isBlocked,
                "homeId": homeId,
                "generation": generation
            ] as [String: Any]
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: ["accessories": accessories, "count": accessories.count],
            error: nil
        )
    }

    private func handleAccessoriesGet(_ request: IOSNativeRequest) -> IOSNativeResponse {
        if let rejection = ensureAuthorized(request) { return rejection }
        guard let homeId = request.payload?["homeId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing homeId")
        }
        guard let accessoryId = request.payload?["id"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing accessory id")
        }
        guard let home = store.home(withID: homeId) else {
            return errorResponse(request, code: "NOT_FOUND", message: "home not found: \(homeId)")
        }
        guard let accessory = store.accessory(withID: accessoryId, in: home) else {
            return errorResponse(request, code: "NOT_FOUND", message: "accessory not found: \(accessoryId)")
        }
        let generation = store.currentGeneration
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: [
                "accessory": [
                    "id": accessory.uniqueIdentifier.uuidString,
                    "name": accessory.name,
                    "category": accessory.category.localizedDescription,
                    "isReachable": accessory.isReachable,
                    "isBlocked": accessory.isBlocked,
                    "homeId": homeId,
                    "generation": generation
                ]
            ],
            error: nil
        )
    }

    private func handleServicesList(_ request: IOSNativeRequest) -> IOSNativeResponse {
        if let rejection = ensureAuthorized(request) { return rejection }
        guard let homeId = request.payload?["homeId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing homeId")
        }
        guard let accessoryId = request.payload?["accessoryId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing accessoryId")
        }
        guard let home = store.home(withID: homeId) else {
            return errorResponse(request, code: "NOT_FOUND", message: "home not found: \(homeId)")
        }
        guard let accessory = store.accessory(withID: accessoryId, in: home) else {
            return errorResponse(request, code: "NOT_FOUND", message: "accessory not found: \(accessoryId)")
        }
        let generation = store.currentGeneration
        let services = accessory.services.map { service in
            [
                "id": service.uniqueIdentifier.uuidString,
                "name": service.name,
                "serviceType": service.serviceType,
                "isPrimary": service.isPrimaryService,
                "isUserInteractive": service.isUserInteractive,
                "homeId": homeId,
                "accessoryId": accessoryId,
                "generation": generation
            ] as [String: Any]
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: ["services": services, "count": services.count],
            error: nil
        )
    }

    private func handleCharacteristicsList(_ request: IOSNativeRequest) -> IOSNativeResponse {
        if let rejection = ensureAuthorized(request) { return rejection }
        guard let homeId = request.payload?["homeId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing homeId")
        }
        guard let accessoryId = request.payload?["accessoryId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing accessoryId")
        }
        guard let serviceId = request.payload?["serviceId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing serviceId")
        }
        guard let service = store.service(withID: serviceId, homeID: homeId, accessoryID: accessoryId) else {
            return errorResponse(request, code: "NOT_FOUND", message: "service not found: \(serviceId)")
        }
        let generation = store.currentGeneration
        let characteristics = service.characteristics.map { characteristic in
            var props: [String: Any] = [
                "id": characteristic.uniqueIdentifier.uuidString,
                "characteristicType": characteristic.characteristicType,
                "isNotificationEnabled": characteristic.isNotificationEnabled,
                "isReadable": characteristic.properties.contains(HMCharacteristicPropertyReadable),
                "isWritable": characteristic.properties.contains(HMCharacteristicPropertyWritable),
                "supportsEventNotification": characteristic.properties.contains(HMCharacteristicPropertySupportsEventNotification),
                "homeId": homeId,
                "accessoryId": accessoryId,
                "serviceId": serviceId,
                "generation": generation
            ]
            if let meta = characteristic.metadata,
               let format = meta.format {
                props["format"] = format
            }
            return props
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: ["characteristics": characteristics, "count": characteristics.count],
            error: nil
        )
    }

    private func handleCharacteristicsRead(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        if let rejection = ensureAuthorized(request) { return rejection }
        guard let homeId = request.payload?["homeId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing homeId")
        }
        guard let accessoryId = request.payload?["accessoryId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing accessoryId")
        }
        guard let serviceId = request.payload?["serviceId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing serviceId")
        }
        guard let characteristicId = request.payload?["id"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing characteristic id")
        }
        guard let service = store.service(withID: serviceId, homeID: homeId, accessoryID: accessoryId) else {
            return errorResponse(request, code: "NOT_FOUND", message: "service not found: \(serviceId)")
        }
        guard let characteristic = store.characteristic(withID: characteristicId, in: service) else {
            return errorResponse(request, code: "NOT_FOUND", message: "characteristic not found: \(characteristicId)")
        }
        guard characteristic.properties.contains(HMCharacteristicPropertyReadable) else {
            return errorResponse(request, code: "NOT_WRITABLE", message: "characteristic is not readable")
        }

        do {
            let value = try await readCharacteristicValue(characteristic)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: [
                    "value": value,
                    "id": characteristicId,
                    "serviceId": serviceId,
                    "accessoryId": accessoryId,
                    "homeId": homeId
                ],
                error: nil
            )
        } catch {
            return errorResponse(request, code: "READ_FAILED", message: error.localizedDescription)
        }
    }

    private func handleCharacteristicsWrite(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        if let rejection = ensureAuthorized(request) { return rejection }
        guard let homeId = request.payload?["homeId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing homeId")
        }
        guard let accessoryId = request.payload?["accessoryId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing accessoryId")
        }
        guard let serviceId = request.payload?["serviceId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing serviceId")
        }
        guard let characteristicId = request.payload?["id"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing characteristic id")
        }
        guard let payloadValue = request.payload?["value"] else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing value to write")
        }
        guard let service = store.service(withID: serviceId, homeID: homeId, accessoryID: accessoryId) else {
            return errorResponse(request, code: "NOT_FOUND", message: "service not found: \(serviceId)")
        }
        guard let characteristic = store.characteristic(withID: characteristicId, in: service) else {
            return errorResponse(request, code: "NOT_FOUND", message: "characteristic not found: \(characteristicId)")
        }
        guard characteristic.properties.contains(HMCharacteristicPropertyWritable) else {
            return errorResponse(request, code: "NOT_WRITABLE", message: "characteristic is not writable")
        }

        let writeValue = convertToCharacteristicValue(payloadValue, for: characteristic)

        do {
            try await writeCharacteristicValue(characteristic, value: writeValue)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: [
                    "written": true,
                    "id": characteristicId,
                    "serviceId": serviceId,
                    "accessoryId": accessoryId,
                    "homeId": homeId,
                    "value": writeValue
                ],
                error: nil
            )
        } catch {
            return errorResponse(request, code: "WRITE_FAILED", message: error.localizedDescription)
        }
    }

    private func handleActionSetsList(_ request: IOSNativeRequest) -> IOSNativeResponse {
        if let rejection = ensureAuthorized(request) { return rejection }
        guard let homeId = request.payload?["homeId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing homeId")
        }
        guard let home = store.home(withID: homeId) else {
            return errorResponse(request, code: "NOT_FOUND", message: "home not found: \(homeId)")
        }
        let generation = store.currentGeneration
        let actionSets = home.actionSets.map { actionSet in
            [
                "id": actionSet.uniqueIdentifier.uuidString,
                "name": actionSet.name,
                "actionSetType": actionSet.actionSetType,
                "isExecuting": actionSet.isExecuting,
                "homeId": homeId,
                "generation": generation
            ] as [String: Any]
        }
        var result: [String: Any] = ["actionSets": actionSets, "count": actionSets.count]
        if request.operation.hasPrefix("homekit.scenes") {
            result["scenes"] = actionSets
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: result,
            error: nil
        )
    }

    private func handleActionSetsGet(_ request: IOSNativeRequest) -> IOSNativeResponse {
        if let rejection = ensureAuthorized(request) { return rejection }
        guard let homeId = request.payload?["homeId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing homeId")
        }
        guard let actionSetId = request.payload?["id"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing action set id")
        }
        guard let home = store.home(withID: homeId) else {
            return errorResponse(request, code: "NOT_FOUND", message: "home not found: \(homeId)")
        }
        guard let actionSet = store.actionSet(withID: actionSetId, in: home) else {
            return errorResponse(request, code: "NOT_FOUND", message: "action set not found: \(actionSetId)")
        }
        let generation = store.currentGeneration
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: [
                "actionSet": [
                    "id": actionSet.uniqueIdentifier.uuidString,
                    "name": actionSet.name,
                    "actionSetType": actionSet.actionSetType,
                    "isExecuting": actionSet.isExecuting,
                    "homeId": homeId,
                    "generation": generation
                ]
            ],
            error: nil
        )
    }

    private func handleActionSetsExecute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        if let rejection = ensureAuthorized(request) { return rejection }
        guard let homeId = request.payload?["homeId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing homeId")
        }
        guard let actionSetId = request.payload?["id"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing action set id")
        }
        guard let home = store.home(withID: homeId) else {
            return errorResponse(request, code: "NOT_FOUND", message: "home not found: \(homeId)")
        }
        guard let actionSet = store.actionSet(withID: actionSetId, in: home) else {
            return errorResponse(request, code: "NOT_FOUND", message: "action set not found: \(actionSetId)")
        }

        do {
            try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
                home.executeActionSet(actionSet) { error in
                    if let error = error {
                        continuation.resume(throwing: error)
                    } else {
                        continuation.resume()
                    }
                }
            }
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestId: request.requestId,
                status: "ok",
                result: [
                    "executed": true,
                    "id": actionSetId,
                    "homeId": homeId
                ],
                error: nil
            )
        } catch {
            return errorResponse(request, code: "EXECUTE_FAILED", message: error.localizedDescription)
        }
    }

    private func handleAutomationsList(_ request: IOSNativeRequest) -> IOSNativeResponse {
        if let rejection = ensureAuthorized(request) { return rejection }
        guard let homeId = request.payload?["homeId"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing homeId")
        }
        guard let home = store.home(withID: homeId) else {
            return errorResponse(request, code: "NOT_FOUND", message: "home not found: \(homeId)")
        }
        let generation = store.currentGeneration
        let triggers = home.triggers.map { trigger in
            [
                "id": trigger.uniqueIdentifier.uuidString,
                "name": trigger.name,
                "isEnabled": trigger.isEnabled,
                "homeId": homeId,
                "generation": generation
            ] as [String: Any]
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestId: request.requestId,
            status: "ok",
            result: ["automations": triggers, "count": triggers.count],
            error: nil
        )
    }

    private func isHomeKitAuthorized() -> Bool {
        if #available(iOS 16.1, *) {
            return homeKitAuthorizationStatus() == .authorized
        }
        return false
    }

    @available(iOS 16.1, *)
    private func homeKitAuthorizationStatus() -> HMAuthorizationStatus {
        return HMHomeManager.authorizationStatus
    }

    private func homeKitAuthorizationStatusUnchecked() -> HMAuthorizationStatus {
        if #available(iOS 16.1, *) {
            return HMHomeManager.authorizationStatus
        }
        return .notDetermined
    }

    private func authorizationStatusString(_ status: HMAuthorizationStatus) -> String {
        switch status {
        case .notDetermined: return "notDetermined"
        case .restricted: return "restricted"
        case .determined: return "determined"
        case .authorized: return "authorized"
        @unknown default: return "unknown"
        }
    }

    private func convertToCharacteristicValue(_ payloadValue: Any, for characteristic: HMCharacteristic) -> Any {
        guard let meta = characteristic.metadata, let format = meta.format else {
            return payloadValue
        }
        switch format {
        case "bool":
            if let num = payloadValue as? NSNumber {
                return num.boolValue
            }
            if let str = payloadValue as? String {
                return str.lowercased() == "true" || str == "1"
            }
            return payloadValue
        case "int", "uint8", "uint16", "uint32":
            if let num = payloadValue as? NSNumber {
                return num.intValue
            }
            if let str = payloadValue as? String, let intVal = Int(str) {
                return intVal
            }
            return payloadValue
        case "float":
            if let num = payloadValue as? NSNumber {
                return num.floatValue
            }
            if let str = payloadValue as? String, let floatVal = Float(str) {
                return floatVal
            }
            return payloadValue
        case "string":
            return "\(payloadValue)"
        default:
            return payloadValue
        }
    }

    private func readCharacteristicValue(_ characteristic: HMCharacteristic) async throws -> Any {
        try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Any, Error>) in
            characteristic.readValue { error in
                if let error = error {
                    continuation.resume(throwing: error)
                } else {
                    continuation.resume(returning: characteristic.value ?? NSNull())
                }
            }
        }
    }

    private func writeCharacteristicValue(_ characteristic: HMCharacteristic, value: Any) async throws {
        try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
            characteristic.writeValue(value) { error in
                if let error = error {
                    continuation.resume(throwing: error)
                } else {
                    continuation.resume()
                }
            }
        }
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
