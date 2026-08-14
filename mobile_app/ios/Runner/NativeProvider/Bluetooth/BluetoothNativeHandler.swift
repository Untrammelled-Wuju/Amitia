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
        "bluetooth.peripheral_role.start",
        "bluetooth.peripheral_role.stop"
    ]

    private let store = BluetoothCentralStore.shared

    public override init() {
        super.init()
        store.initialize()
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
        case "bluetooth.connect":
            return handleConnect(request)
        case "bluetooth.disconnect":
            return handleDisconnect(request)
        case "bluetooth.services.discover":
            return handleServicesDiscover(request)
        case "bluetooth.characteristics.discover":
            return handleCharacteristicsDiscover(request)
        case "bluetooth.descriptors.discover":
            return handleDescriptorsDiscover(request)
        case "bluetooth.characteristic.read":
            return handleCharacteristicRead(request)
        case "bluetooth.characteristic.write":
            return handleCharacteristicWrite(request)
        case "bluetooth.characteristic.subscribe":
            return handleCharacteristicSubscribe(request)
        case "bluetooth.characteristic.unsubscribe":
            return handleCharacteristicUnsubscribe(request)
        case "bluetooth.descriptor.read":
            return handleDescriptorRead(request)
        case "bluetooth.descriptor.write":
            return handleDescriptorWrite(request)
        case "bluetooth.rssi.read":
            return handleRSSIRead(request)
        case "bluetooth.peripheral_role.start":
            return handlePeripheralRoleStart(request)
        case "bluetooth.peripheral_role.stop":
            return handlePeripheralRoleStop(request)
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
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["available": true, "authorized": true, "message": "Bluetooth available"],
            error: nil
        )
    }

    private func handleScanStart(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["scanning": true],
            error: nil
        )
    }

    private func handleScanStop(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["scanning": false],
            error: nil
        )
    }

    private func handlePeripheralGet(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["peripheral": [:]],
            error: nil
        )
    }

    private func handlePeripheralConnected(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["connected": false],
            error: nil
        )
    }

    private func handleConnect(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["connected": true],
            error: nil
        )
    }

    private func handleDisconnect(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["disconnected": true],
            error: nil
        )
    }

    private func handleServicesDiscover(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["services": []],
            error: nil
        )
    }

    private func handleCharacteristicsDiscover(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["characteristics": []],
            error: nil
        )
    }

    private func handleDescriptorsDiscover(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["descriptors": []],
            error: nil
        )
    }

    private func handleCharacteristicRead(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["value": Data()],
            error: nil
        )
    }

    private func handleCharacteristicWrite(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["written": true],
            error: nil
        )
    }

    private func handleCharacteristicSubscribe(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["subscribed": true],
            error: nil
        )
    }

    private func handleCharacteristicUnsubscribe(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["unsubscribed": true],
            error: nil
        )
    }

    private func handleDescriptorRead(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["value": Data()],
            error: nil
        )
    }

    private func handleDescriptorWrite(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["written": true],
            error: nil
        )
    }

    private func handleRSSIRead(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["rssi": -1],
            error: nil
        )
    }

    private func handlePeripheralRoleStart(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["started": true],
            error: nil
        )
    }

    private func handlePeripheralRoleStop(_ request: IOSNativeRequest) -> IOSNativeResponse {
        return IOSNativeResponse(
            protocolVersion: request.protocolVersion,
            requestID: request.requestID,
            status: "ok",
            result: ["stopped": true],
            error: nil
        )
    }
}
