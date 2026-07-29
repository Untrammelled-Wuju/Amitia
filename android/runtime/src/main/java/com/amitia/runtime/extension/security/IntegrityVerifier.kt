package com.amitia.runtime.extension.security

import java.security.MessageDigest

class IntegrityVerifier {

    fun computeFileHash(data: ByteArray): String {
        val digest = MessageDigest.getInstance("SHA-256")
        val hash = digest.digest(data)
        return hash.joinToString("") { "%02x".format(it) }
    }

    fun computeTreeHash(files: List<FileEntry>): String {
        val sorted = files.sortedBy { it.path }
        val digest = MessageDigest.getInstance("SHA-256")
        for (f in sorted) {
            if (f.isDir) continue
            digest.update(f.path.toByteArray(Charsets.UTF_8))
            digest.update(0)
            digest.update(f.hash.toByteArray(Charsets.UTF_8))
            digest.update(0)
        }
        return digest.digest().joinToString("") { "%02x".format(it) }
    }

    fun computeManifestHash(manifestRaw: String): String {
        val digest = MessageDigest.getInstance("SHA-256")
        val hash = digest.digest(manifestRaw.toByteArray(Charsets.UTF_8))
        return hash.joinToString("") { "%02x".format(it) }
    }

    fun computeArchiveHash(raw: ByteArray): String {
        val digest = MessageDigest.getInstance("SHA-256")
        val hash = digest.digest(raw)
        return "sha256:" + hash.joinToString("") { "%02x".format(it) }
    }

    fun computePackageHash(entries: Map<String, ByteArray>): String {
        val digest = MessageDigest.getInstance("SHA-256")
        entries.toSortedMap().forEach { (name, data) ->
            digest.update(name.toByteArray(Charsets.UTF_8))
            digest.update(0)
            digest.update(data)
            digest.update(0)
        }
        return digest.digest().joinToString("") { "%02x".format(it) }
    }

    data class VerifiedEntry(
        val path: String,
        val size: Long,
        val hash: String
    )

    fun verifyIntegrity(
        packageFiles: Map<String, ByteArray>,
        integrityFiles: IntegrityFilesDoc?,
        integrityTree: IntegrityTreeDoc?,
        skipPaths: Set<String> = emptySet()
    ): List<VerifiedEntry> {
        if (integrityFiles == null) {
            throw IntegrityMissingException("integrity/files.json missing")
        }
        if (integrityTree == null) {
            throw IntegrityMissingException("integrity/content-tree.json missing")
        }

        val verifiedFiles = mutableListOf<VerifiedEntry>()

        for ((path, data) in packageFiles) {
            if (path in skipPaths) continue
            if (path == PackageFileConstants.INTEGRITY_FILES) continue
            if (path == PackageFileConstants.INTEGRITY_TREE) continue
            if (path == PackageFileConstants.SIGNATURE_FILE) continue
            if (path == PackageFileConstants.V2_SIGNATURE_FILE) continue

            val entry = integrityFiles.files[path]
                ?: throw IntegrityMismatchException("$path not in integrity manifest")

            val actualHash = computeFileHash(data)
            if (entry.hash != actualHash) {
                throw IntegrityMismatchException("$path hash mismatch: expected ${entry.hash}, got $actualHash")
            }

            if (entry.size != data.size.toLong()) {
                throw IntegrityMismatchException("$path size mismatch: expected ${entry.size}, got ${data.size}")
            }

            verifiedFiles.add(VerifiedEntry(path, data.size.toLong(), actualHash))
        }

        val treeEntries = verifiedFiles.map { FileEntry(it.path, it.size, it.hash) }
        val computedTree = computeTreeHash(treeEntries)

        if (integrityTree.treeHash.isNotEmpty() && computedTree != integrityTree.treeHash) {
            throw IntegrityMismatchException("tree hash mismatch: expected ${integrityTree.treeHash}, got $computedTree")
        }

        return verifiedFiles
    }

    fun verifyManifestContentTreeHash(
        manifestContentTreeHash: String?,
        treeHash: String
    ) {
        if (manifestContentTreeHash.isNullOrEmpty()) return
        if (treeHash.isNotEmpty() && manifestContentTreeHash != treeHash) {
            throw IntegrityMismatchException("manifest content tree hash mismatch: expected $manifestContentTreeHash, got $treeHash")
        }
    }
}
