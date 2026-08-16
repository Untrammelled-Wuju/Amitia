import Foundation

public enum PathResolverError: Error, LocalizedError {
    case emptyPath
    case absolutePath
    case pathTraversal
    case nullByte
    case carriageReturn
    case lineFeed
    case windowsDrive
    case urlScheme
    case escapesRoot
    case invalidComponent
    case symlinkEscape
    case pathTooLong

    public var errorDescription: String? {
        switch self {
        case .emptyPath: return "Path is empty"
        case .absolutePath: return "Absolute path not allowed"
        case .pathTraversal: return "Path traversal (..) not allowed"
        case .nullByte: return "Path contains NUL byte"
        case .carriageReturn: return "Path contains CR"
        case .lineFeed: return "Path contains LF"
        case .windowsDrive: return "Windows drive letter not allowed"
        case .urlScheme: return "URL scheme not allowed"
        case .escapesRoot: return "Resolved path escapes mount root"
        case .invalidComponent: return "Invalid path component"
        case .symlinkEscape: return "Symlink points outside mount root"
        case .pathTooLong: return "Path exceeds maximum length"
        }
    }
}

public struct ResolvedFileReference {
    public let mountId: String
    public let relativePath: String
    public let resolvedURL: URL
    public let isDirectory: Bool
    public let isSymlink: Bool

    public init(mountId: String, relativePath: String, resolvedURL: URL, isDirectory: Bool = false, isSymlink: Bool = false) {
        self.mountId = mountId
        self.relativePath = relativePath
        self.resolvedURL = resolvedURL
        self.isDirectory = isDirectory
        self.isSymlink = isSymlink
    }
}

public final class FilePathResolver {
    public static let shared = FilePathResolver()
    public static let maxPathLength = 4096
    public static let maxComponentLength = 255

    private init() {}

    public func resolve(mountId: String, relativePath: String, rootURL: URL) throws -> ResolvedFileReference {
        let normalized = try normalize(relativePath)

        var components = normalized.split(separator: "/", omittingEmptySubsequences: true).map(String.init)

        var resolvedURL = rootURL
        for component in components {
            resolvedURL.appendPathComponent(component)

            var isDirectory: ObjCBool = false
            let exists = FileManager.default.fileExists(atPath: resolvedURL.path, isDirectory: &isDirectory)
            if exists {
                if let attributes = try? FileManager.default.attributesOfItem(atPath: resolvedURL.path),
                   attributes[.type] as? FileAttributeType == .typeSymbolicLink {
                    let destination = try? FileManager.default.destinationOfSymbolicLink(atPath: resolvedURL.path)
                    if let dest = destination {
                        let parentDir = (resolvedURL.path as NSString).deletingLastPathComponent
                        let absoluteDest = (dest as NSString).hasPrefix("/") ? dest : parentDir + "/" + dest
                        let destURL = URL(fileURLWithPath: absoluteDest).standardizedFileURL
                        let rootStandardized = rootURL.standardizedFileURL
                        if !isWithinRoot(destURL.path, rootPath: rootStandardized.path) {
                            throw PathResolverError.symlinkEscape
                        }
                    }
                }
            }
        }

        let resolvedPath = resolvedURL.standardizedFileURL.path
        let rootPath = rootURL.standardizedFileURL.path

        guard isWithinRoot(resolvedPath, rootPath: rootPath) else {
            throw PathResolverError.escapesRoot
        }

        var isDirectory: ObjCBool = false
        _ = FileManager.default.fileExists(atPath: resolvedPath, isDirectory: &isDirectory)

        var isSymlink = false
        if let attributes = try? FileManager.default.attributesOfItem(atPath: resolvedPath),
           attributes[.type] as? FileAttributeType == .typeSymbolicLink {
            isSymlink = true
        }

        return ResolvedFileReference(
            mountId: mountId,
            relativePath: normalized,
            resolvedURL: resolvedURL,
            isDirectory: isDirectory.boolValue,
            isSymlink: isSymlink
        )
    }

    public func normalize(_ path: String) throws -> String {
        guard !path.isEmpty else {
            throw PathResolverError.emptyPath
        }

        guard path.count <= FilePathResolver.maxPathLength else {
            throw PathResolverError.pathTooLong
        }

        if path.hasPrefix("/") {
            throw PathResolverError.absolutePath
        }

        if path.hasPrefix("~") {
            throw PathResolverError.absolutePath
        }

        if path.contains("\u{0000}") {
            throw PathResolverError.nullByte
        }

        if path.contains("\r") {
            throw PathResolverError.carriageReturn
        }

        if path.contains("\n") {
            throw PathResolverError.lineFeed
        }

        if path.contains("://") {
            throw PathResolverError.urlScheme
        }

        if path.count >= 2 && path[path.startIndex].isASCII && path[path.index(path.startIndex, offsetBy: 1)] == ":" {
            throw PathResolverError.windowsDrive
        }

        let components = path.split(separator: "/", omittingEmptySubsequences: false).map(String.init)
        var normalizedComponents: [String] = []

        for component in components {
            if component == "." {
                continue
            }
            if component == ".." {
                throw PathResolverError.pathTraversal
            }
            if component.isEmpty {
                continue
            }
            guard component.count <= FilePathResolver.maxComponentLength else {
                throw PathResolverError.pathTooLong
            }
            normalizedComponents.append(component)
        }

        guard !normalizedComponents.isEmpty else {
            throw PathResolverError.emptyPath
        }

        return normalizedComponents.joined(separator: "/")
    }

    public func isWithinRoot(_ resolvedPath: String, rootPath: String) -> Bool {
        let standardizedResolved = URL(fileURLWithPath: resolvedPath).standardizedFileURL.path
        let standardizedRoot = URL(fileURLWithPath: rootPath).standardizedFileURL.path

        if standardizedResolved == standardizedRoot {
            return true
        }

        let rootWithSlash = standardizedRoot.hasSuffix("/") ? standardizedRoot : standardizedRoot + "/"
        return standardizedResolved.hasPrefix(rootWithSlash)
    }
}
