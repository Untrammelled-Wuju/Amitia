import Foundation

public class SecurityScopedBookmarkStore: NSObject {
    public static let shared = SecurityScopedBookmarkStore()

    private var bookmarks: [String: Data] = [:]
    private var mounts: [String: [String: Any]] = [:]
    private let queue = DispatchQueue(label: "com.amitia.securitybookmark", attributes: .concurrent)

    private override init() {
        super.init()
    }

    public func createBookmark(for url: URL) throws -> String {
        guard url.startAccessingSecurityScopedResource() else {
            throw NSError(domain: "SecurityScopedBookmarkStore", code: 1, userInfo: [NSLocalizedDescriptionKey: "failed to start accessing security scoped resource"])
        }
        defer { url.stopAccessingSecurityScopedResource() }

        let bookmarkData = try url.bookmarkData(
            options: .withSecurityScope,
            includingResourceValuesForKeys: nil,
            relativeTo: nil
        )
        let grantId = UUID().uuidString
        queue.async(flags: .barrier) {
            self.bookmarks[grantId] = bookmarkData
        }
        return grantId
    }

    public func store(identifier: String, bookmark: Data) {
        queue.async(flags: .barrier) {
            self.bookmarks[identifier] = bookmark
        }
    }

    public func resolve(identifier: String) -> URL? {
        var result: URL?
        queue.sync {
            guard let data = self.bookmarks[identifier] else {
                return
            }
            var isStale = false
            do {
                let url = try NSURL(
                    resolvingBookmarkData: data,
                    options: .withSecurityScope,
                    relativeTo: nil,
                    bookmarkDataIsStale: &isStale
                )
                result = url as URL
            } catch {
                result = nil
            }
        }
        return result
    }

    public func reauthorize(grantId: String) -> Bool {
        _ = stopAccessing(identifier: grantId)
        return startAccessing(identifier: grantId)
    }

    public func startAccessing(identifier: String) -> Bool {
        guard let url = resolve(identifier: identifier) else {
            return false
        }
        return url.startAccessingSecurityScopedResource()
    }

    public func stopAccessing(identifier: String) {
        guard let url = resolve(identifier: identifier) else {
            return
        }
        url.stopAccessingSecurityScopedResource()
    }

    public func remove(identifier: String) {
        stopAccessing(identifier: identifier)
        queue.async(flags: .barrier) {
            self.bookmarks.removeValue(forKey: identifier)
        }
    }

    public func isStale(identifier: String) -> Bool {
        var stale = true
        queue.sync {
            guard let data = self.bookmarks[identifier] else {
                return
            }
            var isStale = false
            do {
                _ = try NSURL(
                    resolvingBookmarkData: data,
                    options: .withSecurityScope,
                    relativeTo: nil,
                    bookmarkDataIsStale: &isStale
                )
                stale = isStale
            } catch {
                stale = true
            }
        }
        return stale
    }

    public func stat(grantId: String) -> [String: Any]? {
        guard let url = resolve(identifier: grantId) else {
            return nil
        }
        _ = url.startAccessingSecurityScopedResource()
        defer { url.stopAccessingSecurityScopedResource() }

        do {
            let attributes = try FileManager.default.attributesOfItem(atPath: url.path)
            var result: [String: Any] = [:]
            result["size"] = attributes[.size] as? Int ?? 0
            result["creationDate"] = (attributes[.creationDate] as? Date)?.timeIntervalSince1970 ?? 0
            result["modificationDate"] = (attributes[.modificationDate] as? Date)?.timeIntervalSince1970 ?? 0
            result["isDirectory"] = attributes[.type] as? FileAttributeType == .typeDirectory
            result["isFile"] = attributes[.type] as? FileAttributeType == .typeRegular
            result["path"] = url.path
            result["name"] = url.lastPathComponent
            return result
        } catch {
            return nil
        }
    }

    public func list(grantId: String) -> [[String: Any]]? {
        guard let url = resolve(identifier: grantId) else {
            return nil
        }
        _ = url.startAccessingSecurityScopedResource()
        defer { url.stopAccessingSecurityScopedResource() }

        do {
            let contents = try FileManager.default.contentsOfDirectory(at: url, includingPropertiesForKeys: [.isDirectoryKey, .fileSizeKey, .contentModificationDateKey], options: [])
            return contents.map { item in
                var info: [String: Any] = [:]
                info["name"] = item.lastPathComponent
                info["path"] = item.path
                if let values = try? item.resourceValues(forKeys: [.isDirectoryKey, .fileSizeKey, .contentModificationDateKey]) {
                    info["isDirectory"] = values.isDirectory ?? false
                    info["size"] = values.fileSize ?? 0
                    info["modificationDate"] = values.contentModificationDate?.timeIntervalSince1970 ?? 0
                }
                return info
            }
        } catch {
            return nil
        }
    }

    public func read(grantId: String) -> Data? {
        guard let url = resolve(identifier: grantId) else {
            return nil
        }
        _ = url.startAccessingSecurityScopedResource()
        defer { url.stopAccessingSecurityScopedResource() }

        do {
            return try Data(contentsOf: url)
        } catch {
            return nil
        }
    }

    public func write(grantId: String, data: Data) -> Bool {
        guard let url = resolve(identifier: grantId) else {
            return false
        }
        _ = url.startAccessingSecurityScopedResource()
        defer { url.stopAccessingSecurityScopedResource() }

        do {
            try data.write(to: url, options: .atomic)
            return true
        } catch {
            return false
        }
    }

    public func mkdir(parentGrantId: String, name: String) -> String? {
        guard let parentUrl = resolve(identifier: parentGrantId) else {
            return nil
        }
        _ = parentUrl.startAccessingSecurityScopedResource()
        defer { parentUrl.stopAccessingSecurityScopedResource() }

        let newDirUrl = parentUrl.appendingPathComponent(name)
        do {
            try FileManager.default.createDirectory(at: newDirUrl, withIntermediateDirectories: true, attributes: nil)
            let bookmarkData = try newDirUrl.bookmarkData(
                options: .withSecurityScope,
                includingResourceValuesForKeys: nil,
                relativeTo: nil
            )
            let newGrantId = UUID().uuidString
            queue.async(flags: .barrier) {
                self.bookmarks[newGrantId] = bookmarkData
            }
            return newGrantId
        } catch {
            return nil
        }
    }

    public func rename(grantId: String, newName: String) -> Bool {
        guard let url = resolve(identifier: grantId) else {
            return false
        }
        _ = url.startAccessingSecurityScopedResource()
        defer { url.stopAccessingSecurityScopedResource() }

        let parentUrl = url.deletingLastPathComponent()
        let newUrl = parentUrl.appendingPathComponent(newName)
        do {
            try FileManager.default.moveItem(at: url, to: newUrl)
            let bookmarkData = try newUrl.bookmarkData(
                options: .withSecurityScope,
                includingResourceValuesForKeys: nil,
                relativeTo: nil
            )
            queue.async(flags: .barrier) {
                self.bookmarks[grantId] = bookmarkData
            }
            return true
        } catch {
            return false
        }
    }

    public func move(sourceGrantId: String, destGrantId: String) -> Bool {
        guard let sourceUrl = resolve(identifier: sourceGrantId),
              let destDirUrl = resolve(identifier: destGrantId) else {
            return false
        }
        _ = sourceUrl.startAccessingSecurityScopedResource()
        _ = destDirUrl.startAccessingSecurityScopedResource()
        defer {
            sourceUrl.stopAccessingSecurityScopedResource()
            destDirUrl.stopAccessingSecurityScopedResource()
        }

        let destUrl = destDirUrl.appendingPathComponent(sourceUrl.lastPathComponent)
        do {
            try FileManager.default.moveItem(at: sourceUrl, to: destUrl)
            let bookmarkData = try destUrl.bookmarkData(
                options: .withSecurityScope,
                includingResourceValuesForKeys: nil,
                relativeTo: nil
            )
            queue.async(flags: .barrier) {
                self.bookmarks[sourceGrantId] = bookmarkData
            }
            return true
        } catch {
            return false
        }
    }

    public func copy(sourceGrantId: String, destGrantId: String) -> String? {
        guard let sourceUrl = resolve(identifier: sourceGrantId),
              let destDirUrl = resolve(identifier: destGrantId) else {
            return nil
        }
        _ = sourceUrl.startAccessingSecurityScopedResource()
        _ = destDirUrl.startAccessingSecurityScopedResource()
        defer {
            sourceUrl.stopAccessingSecurityScopedResource()
            destDirUrl.stopAccessingSecurityScopedResource()
        }

        let destUrl = destDirUrl.appendingPathComponent(sourceUrl.lastPathComponent)
        do {
            try FileManager.default.copyItem(at: sourceUrl, to: destUrl)
            let bookmarkData = try destUrl.bookmarkData(
                options: .withSecurityScope,
                includingResourceValuesForKeys: nil,
                relativeTo: nil
            )
            let newGrantId = UUID().uuidString
            queue.async(flags: .barrier) {
                self.bookmarks[newGrantId] = bookmarkData
            }
            return newGrantId
        } catch {
            return nil
        }
    }

    public func delete(grantId: String) -> Bool {
        guard let url = resolve(identifier: grantId) else {
            return false
        }
        _ = url.startAccessingSecurityScopedResource()
        defer { url.stopAccessingSecurityScopedResource() }

        do {
            try FileManager.default.removeItem(at: url)
            remove(identifier: grantId)
            return true
        } catch {
            return false
        }
    }

    public func getMount(mountId: String) -> [String: Any]? {
        var result: [String: Any]?
        queue.sync {
            result = self.mounts[mountId]
        }
        return result
    }

    public func listMounts() -> [[String: Any]] {
        var result: [[String: Any]] = []
        queue.sync {
            for (id, info) in self.mounts {
                var entry = info
                entry["mountId"] = id
                result.append(entry)
            }
        }
        return result
    }

    public func removeMount(mountId: String) -> Bool {
        var existed = false
        queue.sync(flags: .barrier) {
            existed = self.mounts.removeValue(forKey: mountId) != nil
        }
        return existed
    }

    public func addMount(mountId: String, info: [String: Any]) {
        queue.async(flags: .barrier) {
            self.mounts[mountId] = info
        }
    }
}
