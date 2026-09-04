package com.amitia.amitia_app.nativeprovider.share

import android.content.Context
import android.net.Uri
import androidx.core.content.FileProvider
import java.io.File
import java.util.UUID

internal class ShareResourceExporter(
    private val context: Context,
    private val fileProviderAuthority: String = "${context.packageName}.fileprovider",
) {

    private val exportRoot: File
        get() = File(context.cacheDir, ShareConstants.SHARE_EXPORT_DIR).apply {
            if (!exists()) mkdirs()
        }

    fun hasShareExportRoot(): Boolean {
        val dir = File(context.cacheDir, ShareConstants.SHARE_EXPORT_DIR)
        return dir.exists() && dir.isDirectory
    }

    fun exportResource(requestId: String, sourcePath: String, displayName: String, mimeType: String): ShareExportItem? {
        return try {
            val sourceFile = File(sourcePath)
            if (!sourceFile.exists() || !sourceFile.isFile || !sourceFile.canRead()) {
                return null
            }

            val stagingDir = File(exportRoot, "$requestId.staging")
            if (!stagingDir.exists() && !stagingDir.mkdirs()) {
                return null
            }

            val safeName = sanitizeFileName(displayName)
            val targetFile = File(stagingDir, safeName)
            sourceFile.copyTo(targetFile, overwrite = true)

            val commitDir = File(exportRoot, requestId)
            if (commitDir.exists()) {
                commitDir.deleteRecursively()
            }
            if (!stagingDir.renameTo(commitDir)) {
                stagingDir.deleteRecursively()
                return null
            }

            val finalFile = File(commitDir, safeName)
            val contentUri = FileProvider.getUriForFile(context, fileProviderAuthority, finalFile)

            ShareExportItem(
                contentUri = contentUri,
                mimeType = mimeType,
                displayName = safeName,
            )
        } catch (e: Exception) {
            null
        }
    }

    fun cleanup(requestId: String) {
        try {
            val stagingDir = File(exportRoot, "$requestId.staging")
            if (stagingDir.exists()) stagingDir.deleteRecursively()
            val commitDir = File(exportRoot, requestId)
            if (commitDir.exists()) commitDir.deleteRecursively()
        } catch (_: Exception) {
        }
    }

    fun cleanupExpired() {
        try {
            val now = System.currentTimeMillis()
            val ttl = ShareConstants.EXPORT_TTL_MINUTES * 60 * 1000
            val dirs = exportRoot.listFiles() ?: return
            for (dir in dirs) {
                if (dir.isDirectory && now - dir.lastModified() > ttl) {
                    dir.deleteRecursively()
                }
            }
        } catch (_: Exception) {
        }
    }

    private fun sanitizeFileName(name: String): String {
        if (name.isBlank()) return "share_${UUID.randomUUID().toString().take(8)}"
        val sb = StringBuilder()
        for (ch in name) {
            if (ch == '/' || ch == '\\' || ch == '\u0000' || ch.code < 32) {
                sb.append('_')
            } else {
                sb.append(ch)
            }
        }
        var result = sb.toString()
        result = result.replace("..", "_")
        if (result.startsWith(".")) result = "_$result"
        if (result.length > 200) {
            val ext = result.substringAfterLast('.', "")
            result = if (ext.isNotBlank()) {
                result.substring(0, 195) + "." + ext
            } else {
                result.substring(0, 200)
            }
        }
        return result.ifBlank { "share_${UUID.randomUUID().toString().take(8)}" }
    }
}
