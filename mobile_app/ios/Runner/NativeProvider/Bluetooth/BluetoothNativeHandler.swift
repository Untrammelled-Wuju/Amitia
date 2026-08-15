import Foundation
import CoreBluetooth

public class BluetoothNativeHandler: NSObject, IOSNativeOperationHandler {
    public let operations: Set<String> = [
        "bluetooth.status",
        "bluetooth.scan.start",
        "bluetooth.scan.stop",
        "bluetooth.peripheral.get",
        "bluetooth.peripheral.connected",
        "bluetooth.connect",
        "bluetooth.disconnect",
        "bluetooth.services.discover",
        "bluetooth.characteristics.discover",
        "bluetooth.descriptors.discover",
        "bluetooth.characteristic.read",
        "bluetooth.characteristic.write",
        "bluetooth.characteristic.subscribe",
        "bluetooth.characteristic.unsubscribe",
        "bluetooth.descriptor.read",
        "bluetooth.descriptor.write",
        "bluetooth.rssi.read",
        "bluetooth.peripherals.list"
    ]

    private let store = BluetoothCentralStore.shared
    private let defaultScanDuration: TimeInterval = 10.0

    public override init() {
        super.init()
        store.initialize()
    }

    public func capabilitySnapshot() -> IOSNativeCapability {
        let state = store.adapterState()
        let available = state == .poweredOn
        let authorization = store.authorizationStatus()
        let authorized = authorization == "authorized"
        let hardwareAvailable = state != .unsupported
        let platformSupported = true
        return IOSNativeCapability(
            available: available,
            authorized: authorized,
            hardwareAvailable: hardwareAvailable,
            platformSupported: platformSupported,
            foregroundRequired: false
        )
    }

    public func execute(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        switch request.operation {
        case "bluetooth.status":
            return handleStatus(request)
        case "bluetooth.scan.start":
            return handleScanStart(request)
        case "bluetooth.scan.stop":
            return handleScanStop(request)
        case "bluetooth.peripheral.get":
            return handlePeripheralGet(request)
        case "bluetooth.peripheral.connected":
            return handlePeripheralConnected(request)
        case "bluetooth.peripherals.list":
            return handlePeripheralsList(request)
        case "bluetooth.connect":
            return await handleConnect(request)
        case "bluetooth.disconnect":
            return await handleDisconnect(request)
        case "bluetooth.services.discover":
            return await handleServicesDiscover(request)
        case "bluetooth.characteristics.discover":
            return await handleCharacteristicsDiscover(request)
        case "bluetooth.descriptors.discover":
            return await handleDescriptorsDiscover(request)
        case "bluetooth.characteristic.read":
            return await handleCharacteristicRead(request)
        case "bluetooth.characteristic.write":
            return await handleCharacteristicWrite(request)
        case "bluetooth.characteristic.subscribe":
            return await handleCharacteristicSubscribe(request)
        case "bluetooth.characteristic.unsubscribe":
            return await handleCharacteristicUnsubscribe(request)
        case "bluetooth.descriptor.read":
            return await handleDescriptorRead(request)
        case "bluetooth.descriptor.write":
            return await handleDescriptorWrite(request)
        case "bluetooth.rssi.read":
            return await handleRSSIRead(request)
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

    private func handleStatus(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let state = store.adapterState()
        let stateString = adapterStateString(state)
        let authorization = store.authorizationStatus()
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: [
                "available": state == .poweredOn,
                "authorized": authorization == "authorized",
                "authorizationStatus": authorization,
                "adapterState": stateString,
                "generation": store.currentGeneration
            ],
            error: nil
        )
    }

    private func handleScanStart(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let state = store.adapterState()
        guard state == .poweredOn else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "ADAPTER_NOT_AVAILABLE", message: "Bluetooth adapter not powered on: \(adapterStateString(state))")
            )
        }

        var serviceUUIDs: [CBUUID]? = nil
        if let uuidStrings = request.payload?["serviceUUIDs"] as? [String] {
            serviceUUIDs = uuidStrings.map { CBUUID(string: $0) }
        }

        let duration = (request.payload?["duration"] as? Double) ?? defaultScanDuration
        let boundedDuration = min(max(duration, 1.0), 60.0)

        let started = store.startScan(withServiceUUIDs: serviceUUIDs, duration: boundedDuration)
        if started {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: ["scanning": true, "duration": boundedDuration],
                error: nil
            )
        } else {
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "error",
                result: nil,
                error: IOSNativeError(code: "SCAN_FAILED", message: "Failed to start scan")
            )
        }
    }

    private func handleScanStop(_ request: IOSNativeRequest) -> IOSNativeResponse {
        store.stopScan()
        let peripherals = store.getScannedPeripherals().map { ref in
            ["id": ref.opaqueID, "name": ref.name ?? NSNull(), "generation": ref.generation] as [String: Any]
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["scanning": false, "peripherals": peripherals, "count": peripherals.count],
            error: nil
        )
    }

    private func handlePeripheralGet(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let peripheralId = request.payload?["id"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing peripheral id")
        }
        guard let ref = store.getPeripheral(withID: peripheralId) else {
            return errorResponse(request, code: "NOT_FOUND", message: "peripheral not found: \(peripheralId)")
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: [
                "peripheral": [
                    "id": ref.opaqueID,
                    "name": ref.name ?? NSNull(),
                    "connected": store.isPeripheralConnected(ref.opaqueID),
                    "generation": ref.generation
                ]
            ],
            error: nil
        )
    }

    private func handlePeripheralConnected(_ request: IOSNativeRequest) -> IOSNativeResponse {
        guard let peripheralId = request.payload?["id"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing peripheral id")
        }
        let connected = store.isPeripheralConnected(peripheralId)
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["connected": connected, "id": peripheralId],
            error: nil
        )
    }

    private func handlePeripheralsList(_ request: IOSNativeRequest) -> IOSNativeResponse {
        let peripherals = store.getScannedPeripherals().map { ref in
            ["id": ref.opaqueID, "name": ref.name ?? NSNull(), "connected": store.isPeripheralConnected(ref.opaqueID), "generation": ref.generation] as [String: Any]
        }
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["peripherals": peripherals, "count": peripherals.count, "generation": store.currentGeneration],
            error: nil
        )
    }

    private func handleConnect(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let peripheralId = request.payload?["id"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing peripheral id")
        }
        do {
            try await store.connectPeripheral(withID: peripheralId)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: ["connected": true, "id": peripheralId],
                error: nil
            )
        } catch {
            return errorResponse(request, code: "CONNECT_FAILED", message: error.localizedDescription)
        }
    }

    private func handleDisconnect(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let peripheralId = request.payload?["id"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing peripheral id")
        }
        do {
            try await store.disconnectPeripheral(withID: peripheralId)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: ["disconnected": true, "id": peripheralId],
                error: nil
            )
        } catch {
            return errorResponse(request, code: "DISCONNECT_FAILED", message: error.localizedDescription)
        }
    }

    private func handleServicesDiscover(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let peripheralId = request.payload?["id"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing peripheral id")
        }
        do {
            let services = try await store.discoverServices(forPeripheralID: peripheralId, serviceUUIDs: nil)
            let gen = store.currentGeneration
            let result = services.map { service in
                [
                    "id": peripheralId + "-" + service.uuid.uuidString,
                    "uuid": service.uuid.uuidString,
                    "peripheralId": peripheralId,
                    "isPrimary": service.isPrimary,
                    "generation": gen
                ] as [String: Any]
            }
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: ["services": result, "count": result.count],
                error: nil
            )
        } catch {
            return errorResponse(request, code: "DISCOVERY_FAILED", message: error.localizedDescription)
        }
    }

    private func handleCharacteristicsDiscover(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let peripheralId = request.payload?["id"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing peripheral id")
        }
        guard let serviceUUIDString = request.payload?["serviceUUID"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing serviceUUID")
        }
        let serviceUUID = CBUUID(string: serviceUUIDString)
        let services = store.getPeripheralServices(forPeripheralID: peripheralId)
        guard let service = services.first(where: { $0.uuid == serviceUUID }) else {
            return errorResponse(request, code: "NOT_FOUND", message: "service not found: \(serviceUUIDString)")
        }
        do {
            let characteristics = try await store.discoverCharacteristics(forPeripheralID: peripheralId, service: service, characteristicUUIDs: nil)
            let gen = store.currentGeneration
            let result = characteristics.map { char in
                var props: [String: Any] = [
                    "id": peripheralId + "-" + serviceUUIDString + "-" + char.uuid.uuidString,
                    "uuid": char.uuid.uuidString,
                    "serviceUUID": serviceUUIDString,
                    "peripheralId": peripheralId,
                    "properties": characteristicPropertiesString(char.properties),
                    "generation": gen
                ]
                if let data = char.value {
                    props["value"] = data.base64EncodedString()
                }
                return props
            }
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: ["characteristics": result, "count": result.count],
                error: nil
            )
        } catch {
            return errorResponse(request, code: "DISCOVERY_FAILED", message: error.localizedDescription)
        }
    }

    private func handleDescriptorsDiscover(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let peripheralId = request.payload?["id"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing peripheral id")
        }
        guard let serviceUUIDString = request.payload?["serviceUUID"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing serviceUUID")
        }
        guard let charUUIDString = request.payload?["characteristicUUID"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing characteristicUUID")
        }
        let services = store.getPeripheralServices(forPeripheralID: peripheralId)
        guard let service = services.first(where: { $0.uuid.uuidString == serviceUUIDString }) else {
            return errorResponse(request, code: "NOT_FOUND", message: "service not found: \(serviceUUIDString)")
        }
        guard let characteristic = service.characteristics?.first(where: { $0.uuid.uuidString == charUUIDString }) else {
            return errorResponse(request, code: "NOT_FOUND", message: "characteristic not found: \(charUUIDString)")
        }
        do {
            let descriptors = try await store.discoverDescriptors(forPeripheralID: peripheralId, characteristic: characteristic)
            let gen = store.currentGeneration
            let result = descriptors.map { desc in
                [
                    "id": peripheralId + "-" + serviceUUIDString + "-" + charUUIDString + "-" + desc.uuid.uuidString,
                    "uuid": desc.uuid.uuidString,
                    "serviceUUID": serviceUUIDString,
                    "characteristicUUID": charUUIDString,
                    "peripheralId": peripheralId,
                    "generation": gen
                ] as [String: Any]
            }
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: ["descriptors": result, "count": result.count],
                error: nil
            )
        } catch {
            return errorResponse(request, code: "DISCOVERY_FAILED", message: error.localizedDescription)
        }
    }

    private func handleCharacteristicRead(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let peripheralId = request.payload?["id"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing peripheral id")
        }
        guard let serviceUUIDString = request.payload?["serviceUUID"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing serviceUUID")
        }
        guard let charUUIDString = request.payload?["characteristicUUID"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing characteristicUUID")
        }
        let services = store.getPeripheralServices(forPeripheralID: peripheralId)
        guard let service = services.first(where: { $0.uuid.uuidString == serviceUUIDString }) else {
            return errorResponse(request, code: "NOT_FOUND", message: "service not found: \(serviceUUIDString)")
        }
        guard let characteristic = service.characteristics?.first(where: { $0.uuid.uuidString == charUUIDString }) else {
            return errorResponse(request, code: "NOT_FOUND", message: "characteristic not found: \(charUUIDString)")
        }
        do {
            let data = try await store.readCharacteristic(forPeripheralID: peripheralId, characteristic: characteristic)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: [
                    "value": data.base64EncodedString(),
                    "uuid": charUUIDString,
                    "serviceUUID": serviceUUIDString,
                    "peripheralId": peripheralId
                ],
                error: nil
            )
        } catch {
            return errorResponse(request, code: "READ_FAILED", message: error.localizedDescription)
        }
    }

    private func handleCharacteristicWrite(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let peripheralId = request.payload?["id"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing peripheral id")
        }
        guard let serviceUUIDString = request.payload?["serviceUUID"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing serviceUUID")
        }
        guard let charUUIDString = request.payload?["characteristicUUID"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing characteristicUUID")
        }
        guard let valueString = request.payload?["value"] as? String,
              let data = Data(base64Encoded: valueString) else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing or invalid base64 value")
        }
        let services = store.getPeripheralServices(forPeripheralID: peripheralId)
        guard let service = services.first(where: { $0.uuid.uuidString == serviceUUIDString }) else {
            return errorResponse(request, code: "NOT_FOUND", message: "service not found: \(serviceUUIDString)")
        }
        guard let characteristic = service.characteristics?.first(where: { $0.uuid.uuidString == charUUIDString }) else {
            return errorResponse(request, code: "NOT_FOUND", message: "characteristic not found: \(charUUIDString)")
        }
        let writeType: CBCharacteristicWriteType = (request.payload?["withResponse"] as? Bool == false) ? .withoutResponse : .withResponse
        do {
            try await store.writeCharacteristic(forPeripheralID: peripheralId, characteristic: characteristic, data: data, writeType: writeType)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: [
                    "written": true,
                    "uuid": charUUIDString,
                    "serviceUUID": serviceUUIDString,
                    "peripheralId": peripheralId
                ],
                error: nil
            )
        } catch {
            return errorResponse(request, code: "WRITE_FAILED", message: error.localizedDescription)
        }
    }

    private func handleCharacteristicSubscribe(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let peripheralId = request.payload?["id"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing peripheral id")
        }
        guard let serviceUUIDString = request.payload?["serviceUUID"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing serviceUUID")
        }
        guard let charUUIDString = request.payload?["characteristicUUID"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing characteristicUUID")
        }
        let services = store.getPeripheralServices(forPeripheralID: peripheralId)
        guard let service = services.first(where: { $0.uuid.uuidString == serviceUUIDString }) else {
            return errorResponse(request, code: "NOT_FOUND", message: "service not found: \(serviceUUIDString)")
        }
        guard let characteristic = service.characteristics?.first(where: { $0.uuid.uuidString == charUUIDString }) else {
            return errorResponse(request, code: "NOT_FOUND", message: "characteristic not found: \(charUUIDString)")
        }
        do {
            try await store.setNotify(forPeripheralID: peripheralId, characteristic: characteristic, enabled: true)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: [
                    "subscribed": true,
                    "notifying": characteristic.isNotifying,
                    "uuid": charUUIDString,
                    "serviceUUID": serviceUUIDString,
                    "peripheralId": peripheralId
                ],
                error: nil
            )
        } catch {
            return errorResponse(request, code: "SUBSCRIBE_FAILED", message: error.localizedDescription)
        }
    }

    private func handleCharacteristicUnsubscribe(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let peripheralId = request.payload?["id"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing peripheral id")
        }
        guard let serviceUUIDString = request.payload?["serviceUUID"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing serviceUUID")
        }
        guard let charUUIDString = request.payload?["characteristicUUID"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing characteristicUUID")
        }
        let services = store.getPeripheralServices(forPeripheralID: peripheralId)
        guard let service = services.first(where: { $0.uuid.uuidString == serviceUUIDString }) else {
            return errorResponse(request, code: "NOT_FOUND", message: "service not found: \(serviceUUIDString)")
        }
        guard let characteristic = service.characteristics?.first(where: { $0.uuid.uuidString == charUUIDString }) else {
            return errorResponse(request, code: "NOT_FOUND", message: "characteristic not found: \(charUUIDString)")
        }
        do {
            try await store.setNotify(forPeripheralID: peripheralId, characteristic: characteristic, enabled: false)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: [
                    "unsubscribed": true,
                    "notifying": characteristic.isNotifying,
                    "uuid": charUUIDString,
                    "serviceUUID": serviceUUIDString,
                    "peripheralId": peripheralId
                ],
                error: nil
            )
        } catch {
            return errorResponse(request, code: "UNSUBSCRIBE_FAILED", message: error.localizedDescription)
        }
    }

    private func handleDescriptorRead(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let peripheralId = request.payload?["id"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing peripheral id")
        }
        guard let serviceUUIDString = request.payload?["serviceUUID"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing serviceUUID")
        }
        guard let charUUIDString = request.payload?["characteristicUUID"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing characteristicUUID")
        }
        guard let descUUIDString = request.payload?["descriptorUUID"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing descriptorUUID")
        }
        let services = store.getPeripheralServices(forPeripheralID: peripheralId)
        guard let service = services.first(where: { $0.uuid.uuidString == serviceUUIDString }) else {
            return errorResponse(request, code: "NOT_FOUND", message: "service not found: \(serviceUUIDString)")
        }
        guard let characteristic = service.characteristics?.first(where: { $0.uuid.uuidString == charUUIDString }) else {
            return errorResponse(request, code: "NOT_FOUND", message: "characteristic not found: \(charUUIDString)")
        }
        guard let descriptor = characteristic.descriptors?.first(where: { $0.uuid.uuidString == descUUIDString }) else {
            return errorResponse(request, code: "NOT_FOUND", message: "descriptor not found: \(descUUIDString)")
        }
        do {
            let value = try await store.readDescriptor(forPeripheralID: peripheralId, descriptor: descriptor)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: [
                    "value": descriptorValueString(value),
                    "uuid": descUUIDString,
                    "characteristicUUID": charUUIDString,
                    "serviceUUID": serviceUUIDString,
                    "peripheralId": peripheralId
                ],
                error: nil
            )
        } catch {
            return errorResponse(request, code: "READ_FAILED", message: error.localizedDescription)
        }
    }

    private func handleDescriptorWrite(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let peripheralId = request.payload?["id"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing peripheral id")
        }
        guard let serviceUUIDString = request.payload?["serviceUUID"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing serviceUUID")
        }
        guard let charUUIDString = request.payload?["characteristicUUID"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing characteristicUUID")
        }
        guard let descUUIDString = request.payload?["descriptorUUID"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing descriptorUUID")
        }
        guard let valueString = request.payload?["value"] as? String,
              let data = Data(base64Encoded: valueString) else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing or invalid base64 value")
        }
        let services = store.getPeripheralServices(forPeripheralID: peripheralId)
        guard let service = services.first(where: { $0.uuid.uuidString == serviceUUIDString }) else {
            return errorResponse(request, code: "NOT_FOUND", message: "service not found: \(serviceUUIDString)")
        }
        guard let characteristic = service.characteristics?.first(where: { $0.uuid.uuidString == charUUIDString }) else {
            return errorResponse(request, code: "NOT_FOUND", message: "characteristic not found: \(charUUIDString)")
        }
        guard let descriptor = characteristic.descriptors?.first(where: { $0.uuid.uuidString == descUUIDString }) else {
            return errorResponse(request, code: "NOT_FOUND", message: "descriptor not found: \(descUUIDString)")
        }
        do {
            try await store.writeDescriptor(forPeripheralID: peripheralId, descriptor: descriptor, data: data)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: [
                    "written": true,
                    "uuid": descUUIDString,
                    "characteristicUUID": charUUIDString,
                    "serviceUUID": serviceUUIDString,
                    "peripheralId": peripheralId
                ],
                error: nil
            )
        } catch {
            return errorResponse(request, code: "WRITE_FAILED", message: error.localizedDescription)
        }
    }

    private func handleRSSIRead(_ request: IOSNativeRequest) async -> IOSNativeResponse {
        guard let peripheralId = request.payload?["id"] as? String else {
            return errorResponse(request, code: "INVALID_ARGUMENT", message: "missing peripheral id")
        }
        do {
            let rssi = try await store.readRSSI(forPeripheralID: peripheralId)
            return IOSNativeResponse(
                protocolVersion: request.protocolVersion,
                requestID: request.requestID,
                status: "ok",
                result: ["rssi": rssi.intValue, "id": peripheralId],
                error: nil
            )
        } catch {
            return errorResponse(request, code: "RSSI_READ_FAILED", message: error.localizedDescription)
        }
    }

    private func adapterStateString(_ state: BluetoothAdapterState) -> String {
        switch state {
        case .unknown: return "unknown"
        case .resetting: return "resetting"
        case .unsupported: return "unsupported"
        case .unauthorized: return "unauthorized"
        case .poweredOff: return "poweredOff"
        case .poweredOn: return "poweredOn"
        }
    }

    private func characteristicPropertiesString(_ properties: CBCharacteristicProperties) -> [String] {
        var result: [String] = []
        if properties.contains(.broadcast) { result.append("broadcast") }
        if properties.contains(.read) { result.append("read") }
        if properties.contains(.writeWithoutResponse) { result.append("writeWithoutResponse") }
        if properties.contains(.write) { result.append("write") }
        if properties.contains(.notify) { result.append("notify") }
        if properties.contains(.indicate) { result.append("indicate") }
        if properties.contains(.authenticatedSignedWrites) { result.append("authenticatedSignedWrites") }
        if properties.contains(.extendedProperties) { result.append("extendedProperties") }
        if properties.contains(.notifyEncryptionRequired) { result.append("notifyEncryptionRequired") }
        if properties.contains(.indicateEncryptionRequired) { result.append("indicateEncryptionRequired") }
        return result
    }

    private func descriptorValueString(_ value: Any) -> Any {
        if let data = value as? Data {
            return data.base64EncodedString()
        }
        if let string = value as? String {
            return string
        }
        if let number = value as? NSNumber {
            return number
        }
        return "\(value)"
    }

    private func errorResponse(_ request: IOSNativeRequest, code: String, message: String) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "error",
            result: nil,
            error: IOSNativeError(code: code, message: message)
        )
    }
}
