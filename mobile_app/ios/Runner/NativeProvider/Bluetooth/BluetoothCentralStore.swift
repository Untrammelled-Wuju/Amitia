import Foundation
import CoreBluetooth

public struct BluetoothPeripheralRef: Sendable {
    public let opaqueID: String
    public let generation: UInt64
    public let peripheralIdentifier: UUID
    public let name: String?

    public init(opaqueID: String, generation: UInt64, peripheralIdentifier: UUID, name: String?) {
        self.opaqueID = opaqueID
        self.generation = generation
        self.peripheralIdentifier = peripheralIdentifier
        self.name = name
    }
}

public struct BluetoothServiceRef: Sendable {
    public let opaqueID: String
    public let generation: UInt64
    public let peripheralID: String
    public let serviceUUID: CBUUID

    public init(opaqueID: String, generation: UInt64, peripheralID: String, serviceUUID: CBUUID) {
        self.opaqueID = opaqueID
        self.generation = generation
        self.peripheralID = peripheralID
        self.serviceUUID = serviceUUID
    }
}

public struct BluetoothCharacteristicRef: Sendable {
    public let opaqueID: String
    public let generation: UInt64
    public let peripheralID: String
    public let serviceID: String
    public let characteristicUUID: CBUUID

    public init(opaqueID: String, generation: UInt64, peripheralID: String, serviceID: String, characteristicUUID: CBUUID) {
        self.opaqueID = opaqueID
        self.generation = generation
        self.peripheralID = peripheralID
        self.serviceID = serviceID
        self.characteristicUUID = characteristicUUID
    }
}

public struct BluetoothDescriptorRef: Sendable {
    public let opaqueID: String
    public let generation: UInt64
    public let peripheralID: String
    public let serviceID: String
    public let characteristicID: String
    public let descriptorUUID: CBUUID

    public init(opaqueID: String, generation: UInt64, peripheralID: String, serviceID: String, characteristicID: String, descriptorUUID: CBUUID) {
        self.opaqueID = opaqueID
        self.generation = generation
        self.peripheralID = peripheralID
        self.serviceID = serviceID
        self.characteristicID = characteristicID
        self.descriptorUUID = descriptorUUID
    }
}

public enum BluetoothAdapterState: Sendable {
    case unknown
    case resetting
    case unsupported
    case unauthorized
    case poweredOff
    case poweredOn
}

final class PendingOperation: @unchecked Sendable {
    var type: String
    var peripheral: CBPeripheral?
    var service: CBService?
    var characteristic: CBCharacteristic?
    var descriptor: CBDescriptor?
    var data: Data?
    var writeType: CBCharacteristicWriteType = .withResponse
    var continuation: CheckedContinuation<Any?, Error>?

    init(type: String) {
        self.type = type
    }
}

public final class BluetoothCentralStore: NSObject, CBCentralManagerDelegate, CBPeripheralDelegate, @unchecked Sendable {
    public static let shared = BluetoothCentralStore()

    private var centralManager: CBCentralManager?
    private var isScanning = false
    private var scanEndDate: Date?
    private var scanTimer: DispatchSourceTimer?
    private let queue = DispatchQueue(label: "com.amitia.bluetooth.central")
    private let lock = NSLock()

    private var knownPeripherals: [UUID: CBPeripheral] = [:]
    private var peripheralNames: [UUID: String] = [:]
    private var peripheralRSSI: [UUID: NSNumber] = [:]
    private var advertisementData: [UUID: [String: Any]] = [:]
    private var connectedPeripherals: Set<UUID> = []
    private var generation: UInt64 = 0

    private var pendingOperations: [String: PendingOperation] = [:]
    private var peripheralDelegateEnabled: Set<UUID> = []

    private override init() {
        super.init()
        NotificationCenter.default.addObserver(
            self,
            selector: #selector(handleWillTerminate),
            name: UIApplication.willTerminateNotification,
            object: nil
        )
    }

    @objc private func handleWillTerminate() {
        invalidate()
    }

    public func initialize() {
        lock.lock()
        if centralManager != nil {
            lock.unlock()
            return
        }
        lock.unlock()
        let manager = CBCentralManager(delegate: self, queue: queue)
        lock.lock()
        centralManager = manager
        lock.unlock()
    }

    public var currentGeneration: UInt64 {
        lock.lock()
        defer { lock.unlock() }
        return generation
    }

    public func invalidate() {
        lock.lock()
        generation += 1
        for (_, peripheral) in knownPeripherals {
            if peripheral.state != .disconnected {
                centralManager?.cancelPeripheralConnection(peripheral)
            }
        }
        knownPeripherals.removeAll()
        connectedPeripherals.removeAll()
        peripheralNames.removeAll()
        peripheralRSSI.removeAll()
        advertisementData.removeAll()
        lock.unlock()
    }

    public func adapterState() -> BluetoothAdapterState {
        lock.lock()
        let manager = centralManager
        lock.unlock()
        guard let manager = manager else { return .unknown }
        switch manager.state {
        case .unknown: return .unknown
        case .resetting: return .resetting
        case .unsupported: return .unsupported
        case .unauthorized: return .unauthorized
        case .poweredOff: return .poweredOff
        case .poweredOn: return .poweredOn
        @unknown default: return .unknown
        }
    }

    public func isBluetoothAvailable() -> Bool {
        return adapterState() == .poweredOn
    }

    public func authorizationStatus() -> String {
        if #available(iOS 13.1, *) {
            let status = CBCentralManager.authorization
            switch status {
            case .notDetermined: return "notDetermined"
            case .restricted: return "restricted"
            case .denied: return "denied"
            case .allowedAlways: return "authorized"
            @unknown default: return "unknown"
            }
        }
        let state = adapterState()
        if state == .poweredOn {
            return "authorized"
        }
        return "notDetermined"
    }

    public func startScan(withServiceUUIDs uuids: [CBUUID]?, duration: TimeInterval) -> Bool {
        lock.lock()
        guard let manager = centralManager, manager.state == .poweredOn else {
            lock.unlock()
            return false
        }
        if isScanning {
            lock.unlock()
            return true
        }
        isScanning = true
        lock.unlock()

        let options: [String: Any] = [
            CBCentralManagerScanOptionAllowDuplicatesKey: false
        ]
        centralManager?.scanForPeripherals(withServices: uuids, options: options)

        scheduleScanEnd(duration: duration)
        return true
    }

    public func stopScan() {
        lock.lock()
        guard isScanning else {
            lock.unlock()
            return
        }
        isScanning = false
        lock.unlock()

        centralManager?.stopScan()
        cancelScanTimer()
    }

    public func getScannedPeripherals() -> [BluetoothPeripheralRef] {
        lock.lock()
        defer { lock.unlock() }
        let gen = generation
        return knownPeripherals.map { (uuid, peripheral) in
            BluetoothPeripheralRef(
                opaqueID: uuid.uuidString,
                generation: gen,
                peripheralIdentifier: uuid,
                name: peripheral.name ?? peripheralNames[uuid]
            )
        }
    }

    public func getPeripheral(withID id: String) -> BluetoothPeripheralRef? {
        lock.lock()
        defer { lock.unlock() }
        guard let uuid = UUID(uuidString: id) else { return nil }
        guard let peripheral = knownPeripherals[uuid] else { return nil }
        let gen = generation
        return BluetoothPeripheralRef(
            opaqueID: uuid.uuidString,
            generation: gen,
            peripheralIdentifier: uuid,
            name: peripheral.name ?? peripheralNames[uuid]
        )
    }

    public func isPeripheralConnected(_ id: String) -> Bool {
        lock.lock()
        defer { lock.unlock() }
        guard let uuid = UUID(uuidString: id) else { return false }
        return connectedPeripherals.contains(uuid)
    }

    public func connectPeripheral(withID id: String) async throws {
        lock.lock()
        guard let uuid = UUID(uuidString: id) else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 400, userInfo: [NSLocalizedDescriptionKey: "invalid peripheral id"])
        }
        guard let peripheral = knownPeripherals[uuid] else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 404, userInfo: [NSLocalizedDescriptionKey: "peripheral not found"])
        }
        guard peripheral.state != .connected else {
            lock.unlock()
            return
        }
        guard let manager = centralManager else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 503, userInfo: [NSLocalizedDescriptionKey: "central manager unavailable"])
        }
        lock.unlock()

        try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
            let op = PendingOperation(type: "connect")
            op.peripheral = peripheral
            op.continuation = continuation as! CheckedContinuation<Any?, Error>?
            let key = "connect-\(id)"
            self.lock.lock()
            self.pendingOperations[key] = op
            self.lock.unlock()
            manager.connect(peripheral, options: nil)
        }
    }

    public func disconnectPeripheral(withID id: String) async throws {
        lock.lock()
        guard let uuid = UUID(uuidString: id) else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 400, userInfo: [NSLocalizedDescriptionKey: "invalid peripheral id"])
        }
        guard let peripheral = knownPeripherals[uuid] else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 404, userInfo: [NSLocalizedDescriptionKey: "peripheral not found"])
        }
        guard let manager = centralManager else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 503, userInfo: [NSLocalizedDescriptionKey: "central manager unavailable"])
        }
        lock.unlock()

        try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
            let op = PendingOperation(type: "disconnect")
            op.peripheral = peripheral
            op.continuation = continuation as! CheckedContinuation<Any?, Error>?
            let key = "disconnect-\(id)"
            self.lock.lock()
            self.pendingOperations[key] = op
            self.lock.unlock()
            manager.cancelPeripheralConnection(peripheral)
        }
    }

    public func discoverServices(forPeripheralID id: String, serviceUUIDs: [CBUUID]?) async throws -> [CBService] {
        lock.lock()
        guard let uuid = UUID(uuidString: id) else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 400, userInfo: [NSLocalizedDescriptionKey: "invalid peripheral id"])
        }
        guard let peripheral = knownPeripherals[uuid] else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 404, userInfo: [NSLocalizedDescriptionKey: "peripheral not found"])
        }
        guard peripheral.state == .connected else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 409, userInfo: [NSLocalizedDescriptionKey: "peripheral not connected"])
        }
        ensurePeripheralDelegate(peripheral)
        lock.unlock()

        return try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<[CBService], Error>) in
            let op = PendingOperation(type: "discoverServices")
            op.peripheral = peripheral
            let key = "discoverServices-\(id)"
            self.lock.lock()
            let existing = self.pendingOperations[key]
            self.lock.unlock()
            if existing != nil {
                continuation.resume(throwing: NSError(domain: "BluetoothCentralStore", code: 409, userInfo: [NSLocalizedDescriptionKey: "operation already in progress"]))
                return
            }
            op.continuation = continuation as! CheckedContinuation<Any?, Error>?
            self.lock.lock()
            self.pendingOperations[key] = op
            self.lock.unlock()
            peripheral.discoverServices(serviceUUIDs)
        }
    }

    public func discoverCharacteristics(forPeripheralID id: String, service: CBService, characteristicUUIDs: [CBUUID]?) async throws -> [CBCharacteristic] {
        lock.lock()
        guard let uuid = UUID(uuidString: id) else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 400, userInfo: [NSLocalizedDescriptionKey: "invalid peripheral id"])
        }
        guard let peripheral = knownPeripherals[uuid] else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 404, userInfo: [NSLocalizedDescriptionKey: "peripheral not found"])
        }
        guard peripheral.state == .connected else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 409, userInfo: [NSLocalizedDescriptionKey: "peripheral not connected"])
        }
        ensurePeripheralDelegate(peripheral)
        lock.unlock()

        let serviceUUID = service.uuid.uuidString
        return try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<[CBCharacteristic], Error>) in
            let op = PendingOperation(type: "discoverCharacteristics")
            op.peripheral = peripheral
            op.service = service
            let key = "discoverCharacteristics-\(id)-\(serviceUUID)"
            self.lock.lock()
            let existing = self.pendingOperations[key]
            self.lock.unlock()
            if existing != nil {
                continuation.resume(throwing: NSError(domain: "BluetoothCentralStore", code: 409, userInfo: [NSLocalizedDescriptionKey: "operation already in progress"]))
                return
            }
            op.continuation = continuation as! CheckedContinuation<Any?, Error>?
            self.lock.lock()
            self.pendingOperations[key] = op
            self.lock.unlock()
            peripheral.discoverCharacteristics(characteristicUUIDs, for: service)
        }
    }

    public func discoverDescriptors(forPeripheralID id: String, characteristic: CBCharacteristic) async throws -> [CBDescriptor] {
        lock.lock()
        guard let uuid = UUID(uuidString: id) else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 400, userInfo: [NSLocalizedDescriptionKey: "invalid peripheral id"])
        }
        guard let peripheral = knownPeripherals[uuid] else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 404, userInfo: [NSLocalizedDescriptionKey: "peripheral not found"])
        }
        guard peripheral.state == .connected else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 409, userInfo: [NSLocalizedDescriptionKey: "peripheral not connected"])
        }
        ensurePeripheralDelegate(peripheral)
        lock.unlock()

        let charUUID = characteristic.uuid.uuidString
        return try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<[CBDescriptor], Error>) in
            let op = PendingOperation(type: "discoverDescriptors")
            op.peripheral = peripheral
            op.characteristic = characteristic
            let key = "discoverDescriptors-\(id)-\(charUUID)"
            self.lock.lock()
            let existing = self.pendingOperations[key]
            self.lock.unlock()
            if existing != nil {
                continuation.resume(throwing: NSError(domain: "BluetoothCentralStore", code: 409, userInfo: [NSLocalizedDescriptionKey: "operation already in progress"]))
                return
            }
            op.continuation = continuation as! CheckedContinuation<Any?, Error>?
            self.lock.lock()
            self.pendingOperations[key] = op
            self.lock.unlock()
            peripheral.discoverDescriptors(for: characteristic)
        }
    }

    public func readCharacteristic(forPeripheralID id: String, characteristic: CBCharacteristic) async throws -> Data {
        lock.lock()
        guard let uuid = UUID(uuidString: id) else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 400, userInfo: [NSLocalizedDescriptionKey: "invalid peripheral id"])
        }
        guard let peripheral = knownPeripherals[uuid] else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 404, userInfo: [NSLocalizedDescriptionKey: "peripheral not found"])
        }
        guard peripheral.state == .connected else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 409, userInfo: [NSLocalizedDescriptionKey: "peripheral not connected"])
        }
        ensurePeripheralDelegate(peripheral)
        lock.unlock()

        let charUUID = characteristic.uuid.uuidString
        return try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Data, Error>) in
            let op = PendingOperation(type: "readCharacteristic")
            op.peripheral = peripheral
            op.characteristic = characteristic
            let key = "readCharacteristic-\(id)-\(charUUID)"
            self.lock.lock()
            let existing = self.pendingOperations[key]
            self.lock.unlock()
            if existing != nil {
                continuation.resume(throwing: NSError(domain: "BluetoothCentralStore", code: 409, userInfo: [NSLocalizedDescriptionKey: "operation already in progress"]))
                return
            }
            op.continuation = continuation as! CheckedContinuation<Any?, Error>?
            self.lock.lock()
            self.pendingOperations[key] = op
            self.lock.unlock()
            peripheral.readValue(for: characteristic)
        }
    }

    public func writeCharacteristic(forPeripheralID id: String, characteristic: CBCharacteristic, data: Data, writeType: CBCharacteristicWriteType) async throws {
        lock.lock()
        guard let uuid = UUID(uuidString: id) else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 400, userInfo: [NSLocalizedDescriptionKey: "invalid peripheral id"])
        }
        guard let peripheral = knownPeripherals[uuid] else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 404, userInfo: [NSLocalizedDescriptionKey: "peripheral not found"])
        }
        guard peripheral.state == .connected else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 409, userInfo: [NSLocalizedDescriptionKey: "peripheral not connected"])
        }
        ensurePeripheralDelegate(peripheral)
        lock.unlock()

        let charUUID = characteristic.uuid.uuidString
        return try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
            let op = PendingOperation(type: "writeCharacteristic")
            op.peripheral = peripheral
            op.characteristic = characteristic
            op.data = data
            op.writeType = writeType
            let key = "writeCharacteristic-\(id)-\(charUUID)"
            self.lock.lock()
            let existing = self.pendingOperations[key]
            self.lock.unlock()
            if existing != nil {
                continuation.resume(throwing: NSError(domain: "BluetoothCentralStore", code: 409, userInfo: [NSLocalizedDescriptionKey: "operation already in progress"]))
                return
            }
            op.continuation = continuation as! CheckedContinuation<Any?, Error>?
            self.lock.lock()
            self.pendingOperations[key] = op
            self.lock.unlock()
            peripheral.writeValue(data, for: characteristic, type: writeType)
        }
    }

    public func readDescriptor(forPeripheralID id: String, descriptor: CBDescriptor) async throws -> Any {
        lock.lock()
        guard let uuid = UUID(uuidString: id) else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 400, userInfo: [NSLocalizedDescriptionKey: "invalid peripheral id"])
        }
        guard let peripheral = knownPeripherals[uuid] else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 404, userInfo: [NSLocalizedDescriptionKey: "peripheral not found"])
        }
        guard peripheral.state == .connected else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 409, userInfo: [NSLocalizedDescriptionKey: "peripheral not connected"])
        }
        ensurePeripheralDelegate(peripheral)
        lock.unlock()

        let descUUID = descriptor.uuid.uuidString
        return try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Any, Error>) in
            let op = PendingOperation(type: "readDescriptor")
            op.peripheral = peripheral
            op.descriptor = descriptor
            let key = "readDescriptor-\(id)-\(descUUID)"
            self.lock.lock()
            let existing = self.pendingOperations[key]
            self.lock.unlock()
            if existing != nil {
                continuation.resume(throwing: NSError(domain: "BluetoothCentralStore", code: 409, userInfo: [NSLocalizedDescriptionKey: "operation already in progress"]))
                return
            }
            op.continuation = continuation as! CheckedContinuation<Any?, Error>?
            self.lock.lock()
            self.pendingOperations[key] = op
            self.lock.unlock()
            peripheral.readValue(for: descriptor)
        }
    }

    public func writeDescriptor(forPeripheralID id: String, descriptor: CBDescriptor, data: Data) async throws {
        lock.lock()
        guard let uuid = UUID(uuidString: id) else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 400, userInfo: [NSLocalizedDescriptionKey: "invalid peripheral id"])
        }
        guard let peripheral = knownPeripherals[uuid] else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 404, userInfo: [NSLocalizedDescriptionKey: "peripheral not found"])
        }
        guard peripheral.state == .connected else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 409, userInfo: [NSLocalizedDescriptionKey: "peripheral not connected"])
        }
        ensurePeripheralDelegate(peripheral)
        lock.unlock()

        let descUUID = descriptor.uuid.uuidString
        return try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
            let op = PendingOperation(type: "writeDescriptor")
            op.peripheral = peripheral
            op.descriptor = descriptor
            op.data = data
            let key = "writeDescriptor-\(id)-\(descUUID)"
            self.lock.lock()
            let existing = self.pendingOperations[key]
            self.lock.unlock()
            if existing != nil {
                continuation.resume(throwing: NSError(domain: "BluetoothCentralStore", code: 409, userInfo: [NSLocalizedDescriptionKey: "operation already in progress"]))
                return
            }
            op.continuation = continuation as! CheckedContinuation<Any?, Error>?
            self.lock.lock()
            self.pendingOperations[key] = op
            self.lock.unlock()
            peripheral.writeValue(data, for: descriptor)
        }
    }

    public func setNotify(forPeripheralID id: String, characteristic: CBCharacteristic, enabled: Bool) async throws {
        lock.lock()
        guard let uuid = UUID(uuidString: id) else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 400, userInfo: [NSLocalizedDescriptionKey: "invalid peripheral id"])
        }
        guard let peripheral = knownPeripherals[uuid] else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 404, userInfo: [NSLocalizedDescriptionKey: "peripheral not found"])
        }
        guard peripheral.state == .connected else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 409, userInfo: [NSLocalizedDescriptionKey: "peripheral not connected"])
        }
        ensurePeripheralDelegate(peripheral)
        lock.unlock()

        let charUUID = characteristic.uuid.uuidString
        try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
            let op = PendingOperation(type: "setNotify")
            op.peripheral = peripheral
            op.characteristic = characteristic
            let key = "setNotify-\(id)-\(charUUID)"
            self.lock.lock()
            let existing = self.pendingOperations[key]
            self.lock.unlock()
            if existing != nil {
                continuation.resume(throwing: NSError(domain: "BluetoothCentralStore", code: 409, userInfo: [NSLocalizedDescriptionKey: "operation already in progress"]))
                return
            }
            op.continuation = continuation as! CheckedContinuation<Any?, Error>?
            self.lock.lock()
            self.pendingOperations[key] = op
            self.lock.unlock()
            peripheral.setNotifyValue(enabled, for: characteristic)
        }
    }

    public func readRSSI(forPeripheralID id: String) async throws -> NSNumber {
        lock.lock()
        guard let uuid = UUID(uuidString: id) else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 400, userInfo: [NSLocalizedDescriptionKey: "invalid peripheral id"])
        }
        guard let peripheral = knownPeripherals[uuid] else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 404, userInfo: [NSLocalizedDescriptionKey: "peripheral not found"])
        }
        guard peripheral.state == .connected else {
            lock.unlock()
            throw NSError(domain: "BluetoothCentralStore", code: 409, userInfo: [NSLocalizedDescriptionKey: "peripheral not connected"])
        }
        ensurePeripheralDelegate(peripheral)
        lock.unlock()

        return try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<NSNumber, Error>) in
            let op = PendingOperation(type: "readRSSI")
            op.peripheral = peripheral
            let key = "readRSSI-\(id)"
            self.lock.lock()
            let existing = self.pendingOperations[key]
            self.lock.unlock()
            if existing != nil {
                continuation.resume(throwing: NSError(domain: "BluetoothCentralStore", code: 409, userInfo: [NSLocalizedDescriptionKey: "operation already in progress"]))
                return
            }
            op.continuation = continuation as! CheckedContinuation<Any?, Error>?
            self.lock.lock()
            self.pendingOperations[key] = op
            self.lock.unlock()
            peripheral.readRSSI()
        }
    }

    public func getPeripheralServices(forPeripheralID id: String) -> [CBService] {
        lock.lock()
        defer { lock.unlock() }
        guard let uuid = UUID(uuidString: id) else { return [] }
        guard let peripheral = knownPeripherals[uuid] else { return [] }
        return peripheral.services ?? []
    }

    private func ensurePeripheralDelegate(_ peripheral: CBPeripheral) {
        lock.lock()
        let alreadySet = peripheralDelegateEnabled.contains(peripheral.identifier)
        if !alreadySet {
            peripheralDelegateEnabled.insert(peripheral.identifier)
            peripheral.delegate = self
        }
        lock.unlock()
    }

    private func scheduleScanEnd(duration: TimeInterval) {
        cancelScanTimer()
        let timer = DispatchSource.makeTimerSource(queue: queue)
        timer.schedule(deadline: .now() + duration)
        timer.setEventHandler { [weak self] in
            self?.stopScan()
        }
        timer.resume()
        lock.lock()
        scanTimer = timer
        lock.unlock()
    }

    private func cancelScanTimer() {
        lock.lock()
        scanTimer?.cancel()
        scanTimer = nil
        lock.unlock()
    }

    private func completeOperation(key: String, result: Any?, error: Error?) {
        lock.lock()
        let op = pendingOperations.removeValue(forKey: key)
        lock.unlock()
        guard let op = op else { return }
        if let error = error {
            op.continuation?.resume(throwing: error)
        } else {
            op.continuation?.resume(returning: result)
        }
    }

    public func centralManagerDidUpdateState(_ central: CBCentralManager) {
        if central.state != .poweredOn {
            stopScan()
        }
        let stateString: String
        switch central.state {
        case .unknown: stateString = "unknown"
        case .resetting: stateString = "resetting"
        case .unsupported: stateString = "unsupported"
        case .unauthorized: stateString = "unauthorized"
        case .poweredOff: stateString = "poweredOff"
        case .poweredOn: stateString = "poweredOn"
        @unknown default: stateString = "unknown"
        }
        NativeEventEmitter.shared.emit(NativeEventPayload(
            domain: "bluetooth",
            event: "adapter.state_changed",
            data: ["state": stateString]
        ))
    }

    public func centralManager(_ central: CBCentralManager, didDiscover peripheral: CBPeripheral, advertisementData: [String : Any], rssi RSSI: NSNumber) {
        lock.lock()
        knownPeripherals[peripheral.identifier] = peripheral
        if let name = peripheral.name {
            peripheralNames[peripheral.identifier] = name
        }
        peripheralRSSI[peripheral.identifier] = RSSI
        advertisementData[peripheral.identifier] = advertisementData
        lock.unlock()
        NativeEventEmitter.shared.emit(NativeEventPayload(
            domain: "bluetooth",
            event: "peripheral.discovered",
            data: [
                "id": peripheral.identifier.uuidString,
                "name": peripheral.name ?? NSNull(),
                "rssi": RSSI.intValue,
                "advertisementData": advertisementData
            ],
            entityRef: peripheral.identifier.uuidString
        ))
    }

    public func centralManager(_ central: CBCentralManager, didConnect peripheral: CBPeripheral) {
        lock.lock()
        connectedPeripherals.insert(peripheral.identifier)
        let key = "connect-\(peripheral.identifier.uuidString)"
        let op = pendingOperations.removeValue(forKey: key)
        lock.unlock()
        if let op = op {
            op.continuation?.resume(returning: nil)
        }
        NativeEventEmitter.shared.emit(NativeEventPayload(
            domain: "bluetooth",
            event: "peripheral.connected",
            data: ["id": peripheral.identifier.uuidString, "name": peripheral.name ?? NSNull()],
            entityRef: peripheral.identifier.uuidString
        ))
    }

    public func centralManager(_ central: CBCentralManager, didFailToConnect peripheral: CBPeripheral, error: Error?) {
        lock.lock()
        let key = "connect-\(peripheral.identifier.uuidString)"
        let op = pendingOperations.removeValue(forKey: key)
        lock.unlock()
        if let op = op {
            op.continuation?.resume(throwing: error ?? NSError(domain: "BluetoothCentralStore", code: 500, userInfo: [NSLocalizedDescriptionKey: "connection failed"]))
        }
        NativeEventEmitter.shared.emit(NativeEventPayload(
            domain: "bluetooth",
            event: "peripheral.connect_failed",
            data: [
                "id": peripheral.identifier.uuidString,
                "name": peripheral.name ?? NSNull(),
                "error": error?.localizedDescription ?? "connection failed"
            ],
            entityRef: peripheral.identifier.uuidString
        ))
    }

    public func centralManager(_ central: CBCentralManager, didDisconnectPeripheral peripheral: CBPeripheral, error: Error?) {
        lock.lock()
        connectedPeripherals.remove(peripheral.identifier)
        let key = "disconnect-\(peripheral.identifier.uuidString)"
        let op = pendingOperations.removeValue(forKey: key)
        lock.unlock()
        if let op = op {
            if let error = error {
                op.continuation?.resume(throwing: error)
            } else {
                op.continuation?.resume(returning: nil)
            }
        }
        NativeEventEmitter.shared.emit(NativeEventPayload(
            domain: "bluetooth",
            event: "peripheral.disconnected",
            data: ["id": peripheral.identifier.uuidString, "name": peripheral.name ?? NSNull(), "error": error?.localizedDescription ?? NSNull()],
            entityRef: peripheral.identifier.uuidString
        ))
    }

    public func peripheral(_ peripheral: CBPeripheral, didDiscoverServices error: Error?) {
        let key = "discoverServices-\(peripheral.identifier.uuidString)"
        if let error = error {
            completeOperation(key: key, result: nil, error: error)
            return
        }
        completeOperation(key: key, result: peripheral.services ?? [], error: nil)
    }

    public func peripheral(_ peripheral: CBPeripheral, didDiscoverCharacteristicsFor service: CBService, error: Error?) {
        let key = "discoverCharacteristics-\(peripheral.identifier.uuidString)-\(service.uuid.uuidString)"
        if let error = error {
            completeOperation(key: key, result: nil, error: error)
            return
        }
        completeOperation(key: key, result: service.characteristics ?? [], error: nil)
    }

    public func peripheral(_ peripheral: CBPeripheral, didDiscoverDescriptorsFor characteristic: CBCharacteristic, error: Error?) {
        let key = "discoverDescriptors-\(peripheral.identifier.uuidString)-\(characteristic.uuid.uuidString)"
        if let error = error {
            completeOperation(key: key, result: nil, error: error)
            return
        }
        completeOperation(key: key, result: characteristic.descriptors ?? [], error: nil)
    }

    public func peripheral(_ peripheral: CBPeripheral, didUpdateValueFor characteristic: CBCharacteristic, error: Error?) {
        let key = "readCharacteristic-\(peripheral.identifier.uuidString)-\(characteristic.uuid.uuidString)"
        if let error = error {
            completeOperation(key: key, result: nil, error: error)
            return
        }
        completeOperation(key: key, result: characteristic.value ?? Data(), error: nil)
        if characteristic.isNotifying {
            NativeEventEmitter.shared.emit(NativeEventPayload(
                domain: "bluetooth",
                event: "characteristic.value_updated",
                data: [
                    "peripheralId": peripheral.identifier.uuidString,
                    "serviceUUID": characteristic.service?.uuid.uuidString ?? "",
                    "characteristicUUID": characteristic.uuid.uuidString,
                    "value": (characteristic.value ?? Data()).base64EncodedString()
                ],
                entityRef: peripheral.identifier.uuidString
            ))
        }
    }

    public func peripheral(_ peripheral: CBPeripheral, didWriteValueFor characteristic: CBCharacteristic, error: Error?) {
        let key = "writeCharacteristic-\(peripheral.identifier.uuidString)-\(characteristic.uuid.uuidString)"
        if let error = error {
            completeOperation(key: key, result: nil, error: error)
        } else {
            completeOperation(key: key, result: true, error: nil)
        }
    }

    public func peripheral(_ peripheral: CBPeripheral, didUpdateValueFor descriptor: CBDescriptor, error: Error?) {
        let key = "readDescriptor-\(peripheral.identifier.uuidString)-\(descriptor.uuid.uuidString)"
        if let error = error {
            completeOperation(key: key, result: nil, error: error)
            return
        }
        completeOperation(key: key, result: descriptor.value ?? NSNull(), error: nil)
    }

    public func peripheral(_ peripheral: CBPeripheral, didWriteValueFor descriptor: CBDescriptor, error: Error?) {
        let key = "writeDescriptor-\(peripheral.identifier.uuidString)-\(descriptor.uuid.uuidString)"
        if let error = error {
            completeOperation(key: key, result: nil, error: error)
        } else {
            completeOperation(key: key, result: true, error: nil)
        }
    }

    public func peripheral(_ peripheral: CBPeripheral, didUpdateNotificationStateFor characteristic: CBCharacteristic, error: Error?) {
        let key = "setNotify-\(peripheral.identifier.uuidString)-\(characteristic.uuid.uuidString)"
        if let error = error {
            completeOperation(key: key, result: nil, error: error)
        } else {
            completeOperation(key: key, result: characteristic.isNotifying, error: nil)
        }
    }

    public func peripheralDidUpdateRSSI(_ peripheral: CBPeripheral, error: Error?) {
        lock.lock()
        if let rssi = peripheral.rssi {
            peripheralRSSI[peripheral.identifier] = rssi
        }
        lock.unlock()
        let key = "readRSSI-\(peripheral.identifier.uuidString)"
        if let error = error {
            completeOperation(key: key, result: nil, error: error)
            return
        }
        lock.lock()
        let rssi = peripheralRSSI[peripheral.identifier] ?? NSNumber(value: -1)
        lock.unlock()
        completeOperation(key: key, result: rssi, error: nil)
    }

    public func peripheral(_ peripheral: CBPeripheral, didReadRSSI RSSI: NSNumber, error: Error?) {
        lock.lock()
        peripheralRSSI[peripheral.identifier] = RSSI
        lock.unlock()
        let key = "readRSSI-\(peripheral.identifier.uuidString)"
        if let error = error {
            completeOperation(key: key, result: nil, error: error)
            return
        }
        completeOperation(key: key, result: RSSI, error: nil)
    }
}</longcat_think>
