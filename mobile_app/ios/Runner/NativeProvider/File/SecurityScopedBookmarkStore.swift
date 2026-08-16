import Foundation

public struct MountRecord {
    public let mountId: String
    public let bookmarkData: Data
    public let displayName: String
    public let readOnly: Bool
    public let providerHint: String
    public let createdAt: Date
    public let isSingleFile: Bool

    public init(mountId: String, bookmarkData: Data, displayName: String, readOnly: Bool, providerHint: String, createdAt: Date, isSingleFile: Bool) {
        self.mountId = mountId
        self.bookmarkData = bookmarkData
        self.displayName = displayName
        self.readOnly = readOnly
        self.providerHint = providerHint
        self.createdAt = createdAt
        self.isSingleFile = isSingleFile
    }
}

public enum BookmarkStoreError: Error, LocalizedError {
    case mountNotFound
    case pathResolutionFailed
    case pathEscapesRoot
    case pathTraversal
    case readOnlyMount
    case bookmarkStale
    case securityScopeUnavailable
    case fileNotAccessible
    case invalidArgument
    case createBookmarkFailed

    public var errorCode: String {
        switch self {
        case .mountNotFound: return "MOUNT_NOT_FOUND"
        case .pathResolutionFailed: return "PATH_RESOLUTION_FAILED"
        case .pathEscapesRoot: return "PATH_ESCAPE"
        case .pathTraversal: return "PATH_TRAVERSAL"
        case .readOnlyMount: return "READONLY_MOUNT"
        case .bookmarkStale: return "BOOKMARK_STALE"
        case .securityScopeUnavailable: return "SECURITY_SCOPE_UNAVAILABLE"
        case .fileNotAccessible: return "FILE_NOT_ACCESSIBLE"
        case .invalidArgument: return "INVALID_ARGUMENT"
        case .createBookmarkFailed: return "CREATE_BOOKMARK_FAILED"
        }
    }

    public var errorDescription: String? {
        switch self {
        case .mountNotFound: return "mount not found"
        case .pathResolutionFailed: return "failed to resolve path"
        case .pathEscapesRoot: return "path escapes mount root"
        case .pathTraversal: return "path traversal detected"
        case .readOnlyMount: return "mount is read-only"
        case .bookmarkStale: return "bookmark is stale and needs reauthorization"
        case .securityScopeUnavailable: return "security scope unavailable"
        case .fileNotAccessible: return "file not accessible"
        case .invalidArgument: return "invalid argument"
        case .createBookmarkFailed: return "failed to create bookmark"
        }
    }
}

public class SecurityScopedBookmarkStore: NSObject {
    public static let shared = SecurityScopedBookmarkStore()

    private var mountRecords: [String: MountRecord] = [:]
    private var accessingURLs: Set<String> = []
    private let queue = DispatchQueue(label: "com.amitia.securitybookmark", attributes: .concurrent)

    private override init() {
        super.init()
    }

    public func createBookmark(for url: URL) throws -> MountRecord {
        guard url.startAccessingSecurityScopedResource() else {
            throw BookmarkStoreError.securityScopeUnavailable
        }
        defer { url.stopAccessingSecurityScopedResource() }

        let bookmarkData = try url.bookmarkData(
            options: .withSecurityScope,
            includingResourceValuesForKeys: nil,
            relativeTo: nil
        )

        let mountId = UUID().uuidString
        let isDirectory = (try? url.resourceValues(forKeys: [.isDirectoryKey]))?.isDirectory ?? false

        let record = MountRecord(
            mountId: mountId,
            bookmarkData: bookmarkData,
            displayName: url.lastPathComponent,
            readOnly: false,
            providerHint: "user-selected",
            createdAt: Date(),
            isSingleFile: !isDirectory
        )

        queue.async(flags: .barrier) {
            self.mountRecords[mountId] = record
        }

        return record
    }

    public func resolveRootURL(mountId: String) throws -> URL {
        var record: MountRecord?
        queue.sync {
            record = self.mountRecords[mountId]
        }

        guard let rec = record else {
            throw BookmarkStoreError.mountNotFound
        }

        var isStale = false
        let url = try NSURL(
            resolvingBookmarkData: rec.bookmarkData,
            options: .withSecurityScope,
            relativeTo: nil,
            bookmarkDataIsStale: &isStale
        ) as URL

        if isStale {
            throw BookmarkStoreError.bookmarkStale
        }

        return url
    }

    public func resolvePath(mountId: String, relativePath: String) throws -> ResolvedFileReference {
        let rootURL = try resolveRootURL(mountId: mountId)
        return try FilePathResolver.shared.resolve(mountId: mountId, relativePath: relativePath, rootURL: rootURL)
    }

    public func startAccessing(mountId: String) throws -> Bool {
        let rootURL = try resolveRootURL(mountId: mountId)
        let key = mountId

        var alreadyAccessing = false
        queue.sync {
            alreadyAccessing = self.accessingURLs.contains(key)
        }

        if alreadyAccessing {
            return true
        }

        let success = rootURL.startAccessingSecurityScopedResource()
        if success {
            queue.async(flags: .barrier) {
                self.accessingURLs.insert(key)
            }
        }
        return success
    }

    public func stopAccessing(mountId: String) {
        var wasAccessing = false
        queue.sync {
            wasAccessing = self.accessingURLs.contains(mountId)
        }
        if wasAccessing {
            do {
                let rootURL = try resolveRootURL(mountId: mountId)
                rootURL.stopAccessingSecurityScopedResource()
            } catch {}
            queue.async(flags: .barrier) {
                self.accessingURLs.remove(mountId)
            }
        }
    }

    public func reauthorize(mountId: String) -> Bool {
        stopAccessing(mountId: mountId)
        do {
            let rootURL = try resolveRootURL(mountId: mountId)
            return rootURL.startAccessingSecurityScopedResource()
        } catch {
            return false
        }
    }

    public func remove(mountId: String) {
        stopAccessing(mountId: mountId)
        queue.async(flags: .barrier) {
            self.mountRecords.removeValue(forKey: mountId)
        }
    }

    public func stat(mountId: String, relativePath: String) throws -> [String: Any] {
        let resolved = try resolvePath(mountId: mountId, relativePath: relativePath)
        let url = resolved.resolvedURL

        try ensureAccessing(mountId: mountId)

        let attributes = try FileManager.default.attributesOfItem(atPath: url.path)
        var result: [String: Any] = [:]
        result["size"] = attributes[.size] as? Int ?? 0
        result["creationDate"] = (attributes[.creationDate] as? Date)?.timeIntervalSince1970 ?? 0
        result["modificationDate"] = (attributes[.modificationDate] as? Date)?.timeIntervalSince1970 ?? 0
        result["isDirectory"] = (attributes[.type] as? FileAttributeType) == .typeDirectory
        result["isFile"] = (attributes[.type] as? FileAttributeType) == .typeRegular
        result["name"] = url.lastPathComponent
        result["mountId"] = mountId
        result["relativePath"] = relativePath
        return result
    }

    public func list(mountId: String, relativePath: String) throws -> [[String: Any]] {
        let resolved = try resolvePath(mountId: mountId, relativePath: relativePath)
        let url = resolved.resolvedURL

        try ensureAccessing(mountId: mountId)

        let contents = try FileManager.default.contentsOfDirectory(
            at: url,
            includingPropertiesForKeys: [.isDirectoryKey, .fileSizeKey, .contentModificationDateKey, .isSymbolicLinkKey],
            options: []
        )

        return contents.map { item in
            var info: [String: Any] = [:]
            info["name"] = item.lastPathComponent
            if let values = try? item.resourceValues(forKeys: [.isDirectoryKey, .fileSizeKey, .contentModificationDateKey, .isSymbolicLinkKey]) {
                info["isDirectory"] = values.isDirectory ?? false
                info["size"] = values.fileSize ?? 0
                info["modificationDate"] = values.contentModificationDate?.timeIntervalSince1970 ?? 0
                info["isSymlink"] = values.isSymbolicLink ?? false
            }
            return info
        }
    }

    public func readFile(mountId: String, relativePath: String, offset: Int64, length: Int64) throws -> Data {
        let resolved = try resolvePath(mountId: mountId, relativePath: relativePath)
        let url = resolved.resolvedURL

        try ensureAccessing(mountId: mountId)

        let handle = try FileHandle(forReadingFrom: url)
        if offset > 0 {
            handle.seek(toOffset: UInt64(offset))
        }
        let data: Data
        if length > 0 {
            data = handle.readData(ofLength: Int(length))
        } else {
            data = handle.readDataToEndOfFile()
        }
        handle.closeFile()
        return data
    }

    public func writeFile(mountId: String, relativePath: String, contentBase64: String, offset: Int64) throws -> Bool {
        let resolved = try resolvePath(mountId: mountId, relativePath: relativePath)

        var record: MountRecord?
        queue.sync {
            record = self.mountRecords[mountId]
        }
        if record?.readOnly == true {
            throw BookmarkStoreError.readOnlyMount
        }

        guard let data = Data(base64Encoded: contentBase64) else {
            throw BookmarkStoreError.invalidArgument
        }

        let url = resolved.resolvedURL

        try ensureAccessing(mountId: mountId)

        let parentURL = url.deletingLastPathComponent()
        try FileManager.default.createDirectory(at: parentURL, withIntermediateDirectories: true, attributes: nil)

        if FileManager.default.fileExists(atPath: url.path) {
            let handle = try FileHandle(forWritingTo: url)
            if offset > 0 {
                handle.seek(toOffset: UInt64(offset))
            }
            handle.write(data)
            handle.closeFile()
        } else {
            try data.write(to: url, options: .atomic)
        }

        return true
    }

    public func mkdir(mountId: String, parentRelativePath: String, name: String) throws -> String {
        let parentResolved = try resolvePath(mountId: mountId, relativePath: parentRelativePath)

        var record: MountRecord?
        queue.sync {
            record = self.mountRecords[mountId]
        }
        if record?.readOnly == true {
            throw BookmarkStoreError.readOnlyMount
        }

        let newDirURL = parentResolved.resolvedURL.appendingPathComponent(name)

        try ensureAccessing(mountId: mountId)
        try FileManager.default.createDirectory(at: newDirURL, withIntermediateDirectories: false, attributes: nil)

        let mountRecord = MountRecord(
            mountId: UUID().uuidString,
            bookmarkData: try newDirURL.bookmarkData(options: .withSecurityScope, includingResourceValuesForKeys: nil, relativeTo: nil),
            displayName: name,
            readOnly: false,
            providerHint: "child-of-\(mountId)",
            createdAt: Date(),
            isSingleFile: false
        )

        queue.async(flags: .barrier) {
            self.mountRecords[mountRecord.mountId] = mountRecord
        }

        return mountRecord.mountId
    }

    public func rename(mountId: String, relativePath: String, newName: String) throws {
        let resolved = try resolvePath(mountId: mountId, relativePath: relativePath)

        var record: MountRecord?
        queue.sync {
            record = self.mountRecords[mountId]
        }
        if record?.readOnly == true {
            throw BookmarkStoreError.readOnlyMount
        }

        let parentURL = resolved.resolvedURL.deletingLastPathComponent()
        let newNameNormalized = try FilePathResolver.shared.normalize(newName)
        let newURL = parentURL.appendingPathComponent(newNameNormalized)

        try ensureAccessing(mountId: mountId)
        try FileManager.default.moveItem(at: resolved.resolvedURL, to: newURL)
    }

    public func move(sourceMountId: String, sourceRelativePath: String, destMountId: String, destRelativePath: String) throws {
        let sourceResolved = try resolvePath(mountId: sourceMountId, relativePath: sourceRelativePath)
        let destResolved = try resolvePath(mountId: destMountId, relativePath: destRelativePath)

        var destRecord: MountRecord?
        queue.sync {
            destRecord = self.mountRecords[destMountId]
        }
        if destRecord?.readOnly == true {
            throw BookmarkStoreError.readOnlyMount
        }

        try ensureAccessing(mountId: sourceMountId)
        try ensureAccessing(mountId: destMountId)

        let destParentURL = destResolved.resolvedURL.deletingLastPathComponent()
        try FileManager.default.createDirectory(at: destParentURL, withIntermediateDirectories: true, attributes: nil)

        try FileManager.default.moveItem(at: sourceResolved.resolvedURL, to: destResolved.resolvedURL)
    }

    public func copy(sourceMountId: String, sourceRelativePath: String, destMountId: String, destRelativePath: String) throws -> String {
        let sourceResolved = try resolvePath(mountId: sourceMountId, relativePath: sourceRelativePath)
        let destResolved = try resolvePath(mountId: destMountId, relativePath: destRelativePath)

        var destRecord: MountRecord?
        queue.sync {
            destRecord = self.mountRecords[destMountId]
        }
        if destRecord?.readOnly == true {
            throw BookmarkStoreError.readOnlyMount
        }

        try ensureAccessing(mountId: sourceMountId)
        try ensureAccessing(mountId: destMountId)

        let destParentURL = destResolved.resolvedURL.deletingLastPathComponent()
        try FileManager.default.createDirectory(at: destParentURL, withIntermediateDirectories: true, attributes: nil)

        try FileManager.default.copyItem(at: sourceResolved.resolvedURL, to: destResolved.resolvedURL)

        let newMount = MountRecord(
            mountId: UUID().uuidString,
            bookmarkData: try destResolved.resolvedURL.bookmarkData(options: .withSecurityScope, includingResourceValuesForKeys: nil, relativeTo: nil),
            displayName: destResolved.resolvedURL.lastPathComponent,
            readOnly: false,
            providerHint: "copy-of-\(sourceMountId)",
            createdAt: Date(),
            isSingleFile: true
        )

        queue.async(flags: .barrier) {
            self.mountRecords[newMount.mountId] = newMount
        }

        return newMount.mountId
    }

    public func delete(mountId: String, relativePath: String) throws {
        let resolved = try resolvePath(mountId: mountId, relativePath: relativePath)

        var record: MountRecord?
        queue.sync {
            record = self.mountRecords[mountId]
        }
        if record?.readOnly == true {
            throw BookmarkStoreError.readOnlyMount
        }

        try ensureAccessing(mountId: mountId)
        try FileManager.default.removeItem(at: resolved.resolvedURL)
    }

    public func listMounts() -> [[String: Any]] {
        var result: [[String: Any]] = []
        queue.sync {
            for (id, record) in self.mountRecords {
                var entry: [String: Any] = [:]
                entry["mountId"] = id
                entry["displayName"] = record.displayName
                entry["readOnly"] = record.readOnly
                entry["providerHint"] = record.providerHint
                entry["createdAt"] = record.createdAt.timeIntervalSince1970
                entry["isSingleFile"] = record.isSingleFile
                result.append(entry)
            }
        }
        return result
    }

    public func getMountRecord(mountId: String) -> MountRecord? {
        var record: MountRecord?
        queue.sync {
            record = self.mountRecords[mountId]
        }
        return record
    }

    public func isReadOnly(mountId: String) -> Bool {
        return getMountRecord(mountId: mountId)?.readOnly ?? false
    }

    public func isStale(mountId: String) -> Bool {
        do {
            _ = try resolveRootURL(mountId: mountId)
            return false
        } catch {
            return true
        }
    }

    private func ensureAccessing(mountId: String) throws {
        var needsAccess = false
        queue.sync {
            needsAccess = !self.accessingURLs.contains(mountId)
        }
        if needsAccess {
            let rootURL = try resolveRootURL(mountId: mountId)
            guard rootURL.startAccessingSecurityScopedResource() else {
                throw BookmarkStoreError.securityScopeUnavailable
            }
            queue.async(flags: .barrier) {
                self.accessingURLs.insert(mountId)
            }
        }
    }
}
