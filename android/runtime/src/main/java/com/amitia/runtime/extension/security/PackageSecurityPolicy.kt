package com.amitia.runtime.extension.security

data class ArchivePolicy(
    val maxArchiveBytes: Long,
    val maxEntryCount: Int,
    val maxSingleEntryBytes: Long,
    val maxTotalUncompressedBytes: Long,
    val maxCompressionRatio: Double,
    val maxPathLength: Int,
    val maxDirectoryDepth: Int,
    val allowedFileTypes: List<String> = emptyList(),
    val forbiddenFileTypes: List<String> = emptyList(),
    val allowSymlink: Boolean = false,
    val allowHardlink: Boolean = false,
    val allowNestedArchive: Boolean = false,
    val allowExecutable: Boolean = false
) {
    companion object {
        fun default(): ArchivePolicy = ArchivePolicy(
            maxArchiveBytes = 100L * 1024 * 1024,
            maxEntryCount = 2000,
            maxSingleEntryBytes = 50L * 1024 * 1024,
            maxTotalUncompressedBytes = 200L * 1024 * 1024,
            maxCompressionRatio = 100.0,
            maxPathLength = 512,
            maxDirectoryDepth = 32,
            allowSymlink = false,
            allowHardlink = false,
            allowNestedArchive = false,
            allowExecutable = false
        )

        fun restricted(): ArchivePolicy = ArchivePolicy(
            maxArchiveBytes = 10L * 1024 * 1024,
            maxEntryCount = 200,
            maxSingleEntryBytes = 5L * 1024 * 1024,
            maxTotalUncompressedBytes = 20L * 1024 * 1024,
            maxCompressionRatio = 50.0,
            maxPathLength = 256,
            maxDirectoryDepth = 16,
            allowSymlink = false,
            allowHardlink = false,
            allowNestedArchive = false,
            allowExecutable = false
        )
    }
}

object AllowedRootDirs {
    val allowedRoots: Set<String> = setOf(
        "manifest.json",
        "integrity",
        "modules",
        "resources",
        "assets",
        "migrations",
        "licenses",
        "docs",
        "signatures",
        "META-INF"
    )

    fun isAllowed(path: String): Boolean {
        val root = path.substringBefore("/")
        return allowedRoots.contains(root)
    }
}

object PackageFileConstants {
    const val MANIFEST_FILE = "manifest.json"
    const val INTEGRITY_DIR = "integrity/"
    const val INTEGRITY_FILES = "integrity/files.json"
    const val INTEGRITY_TREE = "integrity/content-tree.json"
    const val MODULES_DIR = "modules/"
    const val RESOURCES_DIR = "resources/"
    const val ASSETS_DIR = "assets/"
    const val MIGRATIONS_DIR = "migrations/"
    const val LICENSES_DIR = "licenses/"
    const val DOCS_DIR = "docs/"
    const val SIGNATURES_DIR = "signatures/"
    const val SIGNATURE_FILE = "signatures/signature.json"
    const val V2_SIGNATURE_FILE = "META-INF/amitia-signature.json"
}
