package com.amitia.amitia_app.runtime.manifest.internal

import java.io.File
import java.security.MessageDigest

internal object InstalledTreeHasher {

    fun computeTreeSha256(rootDir: File): String {
        if (!rootDir.exists() || !rootDir.isDirectory) {
            return sha256String("")
        }
        val entries = collectEntries(rootDir).sortedBy { it.relativePath }
        val digest = MessageDigest.getInstance("SHA-256")
        for (entry in entries) {
            digest.update(entry.relativePath.toByteArray(Charsets.UTF_8))
            digest.update(0x00.toByte())
            digest.update(entry.kind.code.toByteArray(Charsets.UTF_8))
            digest.update(0x00.toByte())
            digest.update(entry.size.toString().toByteArray(Charsets.UTF_8))
            digest.update(0x00.toByte())
            digest.update(entry.sha256.toByteArray(Charsets.UTF_8))
            digest.update(0x0A.toByte())
        }
        return digest.digest().toLowerHex()
    }

    private fun collectEntries(dir: File): List<TreeEntry> {
        val result = mutableListOf<TreeEntry>()
        collectRecursive(dir, "", dir, result)
        return result
    }

    private fun collectRecursive(root: File, relPath: String, file: File, result: MutableList<TreeEntry>) {
        val isSymlink = try {
            java.nio.file.Files.isSymbolicLink(file.toPath())
        } catch (_: Exception) {
            false
        }
        when {
            isSymlink -> {
                val entryPath = relPath.ifEmpty { file.name }
                val target = readLinkSafe(file)
                result.add(
                    TreeEntry(
                        relativePath = entryPath,
                        kind = EntryKind.SYMLINK,
                        size = 0L,
                        sha256 = sha256String(target)
                    )
                )
            }
            file.isFile -> {
                val entryPath = relPath.ifEmpty { file.name }
                result.add(
                    TreeEntry(
                        relativePath = entryPath,
                        kind = EntryKind.FILE,
                        size = file.length(),
                        sha256 = InstalledFileHasher.sha256(file)
                    )
                )
            }
            file.isDirectory -> {
                if (relPath.isNotEmpty()) {
                    result.add(TreeEntry(relPath, EntryKind.DIR, 0L, ""))
                }
                file.listFiles()?.sortedBy { it.name }?.forEach { child ->
                    val childPath = if (relPath.isEmpty()) child.name else "$relPath/${child.name}"
                    collectRecursive(root, childPath, child, result)
                }
            }
        }
    }

    private fun readLinkSafe(file: File): String {
        return try {
            java.nio.file.Files.readSymbolicLink(file.toPath()).toString()
        } catch (_: Exception) {
            ""
        }
    }

    private fun sha256String(content: String): String {
        val digest = MessageDigest.getInstance("SHA-256")
        digest.update(content.toByteArray(Charsets.UTF_8))
        return digest.digest().toLowerHex()
    }

    private fun ByteArray.toLowerHex(): String {
        val sb = StringBuilder(size * 2)
        for (b in this) {
            val v = b.toInt() and 0xFF
            sb.append(HEX[v ushr 4])
            sb.append(HEX[v and 0x0F])
        }
        return sb.toString()
    }

    private data class TreeEntry(
        val relativePath: String,
        val kind: EntryKind,
        val size: Long,
        val sha256: String,
    )

    private enum class EntryKind(val code: String) {
        FILE("file"),
        DIR("dir"),
        SYMLINK("symlink"),
        HARDLINK("hardlink"),
    }

    private val HEX = "0123456789abcdef".toCharArray()
}
