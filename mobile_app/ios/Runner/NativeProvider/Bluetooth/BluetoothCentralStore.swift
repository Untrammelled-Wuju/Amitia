import Foundation
import CoreBluetooth

public class BluetoothCentralStore: NSObject, CBCentralManagerDelegate, CBPeripheralDelegate {
    public static let shared = BluetoothCentralStore()

    private var centralManager: CBCentralManager?
    private var isScanning = false

    private override init() {
        super.init()
    }

    public func initialize() {
        if centralManager == nil {
            centralManager = CBCentralManager(delegate: self, queue: nil)
        }
    }

    public func centralManagerDidUpdateState(_ central: CBCentralManager) {
    }
}
