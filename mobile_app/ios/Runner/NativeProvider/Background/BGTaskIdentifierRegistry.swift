import Foundation

public struct BGCatalogEntry: Codable, Sendable {
    public let systemClass: String
    public let identifier: String
    public let requiresNetwork: Bool
    public let requiresExternalPower: Bool
    public let maxRetryCount: Int

    public init(systemClass: String, identifier: String, requiresNetwork: Bool = true, requiresExternalPower: Bool = false, maxRetryCount: Int = 3) {
        self.systemClass = systemClass
        self.identifier = identifier
        self.requiresNetwork = requiresNetwork
        self.requiresExternalPower = requiresExternalPower
        self.maxRetryCount = maxRetryCount
    }
}

public struct TaskRunMapping: Codable, Sendable {
    public let taskRunId: String
    public let systemClass: String
    public let identifier: String
    public let submittedAt: Date
    public var generation: Int64

    public init(taskRunId: String, systemClass: String, identifier: String, submittedAt: Date = Date(), generation: Int64 = 0) {
        self.taskRunId = taskRunId
        self.systemClass = systemClass
        self.identifier = identifier
        self.submittedAt = submittedAt
        self.generation = generation
    }
}

public actor BGTaskIdentifierRegistry {
    public static let shared = BGTaskIdentifierRegistry()

    private var catalog: [String: BGCatalogEntry] = [:]
    private var taskRunMappings: [String: TaskRunMapping] = [:]
    private var identifierToTaskRun: [String: String] = [:]

    private init() {
        registerDefaultCatalog()
    }

    private func registerDefaultCatalog() {
        catalog["app_refresh"] = BGCatalogEntry(
            systemClass: "app_refresh",
            identifier: "com.amitia.background.refresh",
            requiresNetwork: true,
            requiresExternalPower: false
        )
        catalog["processing"] = BGCatalogEntry(
            systemClass: "processing",
            identifier: "com.amitia.background.processing",
            requiresNetwork: true,
            requiresExternalPower: true
        )
    }

    public func register(systemClass: String, identifier: String, requiresNetwork: Bool = true, requiresExternalPower: Bool = false) {
        catalog[systemClass] = BGCatalogEntry(
            systemClass: systemClass,
            identifier: identifier,
            requiresNetwork: requiresNetwork,
            requiresExternalPower: requiresExternalPower
        )
    }

    public func resolveIdentifier(systemClass: String) -> String? {
        return catalog[systemClass]?.identifier
    }

    public func resolveSystemClass(identifier: String) -> String? {
        for (key, entry) in catalog {
            if entry.identifier == identifier {
                return key
            }
        }
        return nil
    }

    public func createMapping(taskRunId: String, systemClass: String, identifier: String) {
        let mapping = TaskRunMapping(
            taskRunId: taskRunId,
            systemClass: systemClass,
            identifier: identifier
        )
        taskRunMappings[taskRunId] = mapping
        identifierToTaskRun[identifier] = taskRunId
    }

    public func mappingForTaskRun(taskRunId: String) -> TaskRunMapping? {
        return taskRunMappings[taskRunId]
    }

    public func mappingForIdentifier(identifier: String) -> TaskRunMapping? {
        guard let taskRunId = identifierToTaskRun[identifier] else {
            return nil
        }
        return taskRunMappings[taskRunId]
    }

    public func taskRunId(forIdentifier identifier: String) -> String? {
        return identifierToTaskRun[identifier]
    }

    public func identifier(forTaskRunId taskRunId: String) -> String? {
        return taskRunMappings[taskRunId]?.identifier
    }

    public func updateGeneration(taskRunId: String, generation: Int64) {
        if var mapping = taskRunMappings[taskRunId] {
            mapping.generation = generation
            taskRunMappings[taskRunId] = mapping
        }
    }

    public func removeMapping(taskRunId: String) {
        if let mapping = taskRunMappings[taskRunId] {
            identifierToTaskRun.removeValue(forKey: mapping.identifier)
        }
        taskRunMappings.removeValue(forKey: taskRunId)
    }

    public func removeMappingForIdentifier(identifier: String) {
        if let taskRunId = identifierToTaskRun[identifier] {
            taskRunMappings.removeValue(forKey: taskRunId)
        }
        identifierToTaskRun.removeValue(forKey: identifier)
    }

    public func isIdentifierRegistered(_ identifier: String) -> Bool {
        return catalog.values.contains { $0.identifier == identifier }
    }

    public func allIdentifiers() -> [String] {
        return catalog.values.map { $0.identifier }
    }

    public func catalogSnapshot() -> [BGCatalogEntry] {
        return Array(catalog.values)
    }
}
