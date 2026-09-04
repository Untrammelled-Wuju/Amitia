package com.amitia.amitia_app.nativeprovider.workspace

import android.content.ContentResolver
import android.content.Context
import android.content.Intent
import android.database.Cursor
import android.net.Uri
import android.os.Build
import android.provider.DocumentsContract
import android.util.Base64
import com.amitia.amitia_app.MainActivity
import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.ByteArrayOutputStream
import java.text.SimpleDateFormat
import java.util.ArrayDeque
import java.util.Date
import java.util.Locale
import java.util.TimeZone

/** Android Storage Access Framework provider used by device-local Workspace mounts. */
internal class WorkspaceSafNativeHandler(context: Context) : AndroidNativeOperationHandler {
    private val appContext = context.applicationContext
    private val resolver: ContentResolver = appContext.contentResolver

    override val operations: Set<String> = setOf(
        OP_PICK_TREE,
        OP_GRANT_STATUS,
        OP_STAT,
        OP_LIST,
        OP_READ,
        OP_WRITE,
        OP_MKDIR,
        OP_RENAME,
        OP_MOVE,
        OP_COPY,
        OP_DELETE,
        OP_RESOLVE_PATH,
        OP_CREATE_FILE,
    )

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse = try {
        when (request.operation) {
            OP_PICK_TREE -> pickTree(request)
            OP_GRANT_STATUS -> grantStatus(request)
            OP_STAT -> stat(request)
            OP_LIST -> list(request)
            OP_READ -> read(request)
            OP_WRITE -> write(request)
            OP_MKDIR -> mkdir(request)
            OP_RENAME -> rename(request)
            OP_MOVE -> move(request)
            OP_COPY -> copy(request)
            OP_DELETE -> delete(request)
            OP_RESOLVE_PATH -> resolvePath(request)
            OP_CREATE_FILE -> createFile(request)
            else -> failure(request, "OPERATION_NOT_SUPPORTED", "unsupported SAF operation: ${request.operation}")
        }
    } catch (security: SecurityException) {
        failure(request, "PERMISSION_REVOKED", security.message ?: "workspace permission revoked", "PERMISSION_REVOKED")
    } catch (error: IllegalArgumentException) {
        failure(request, "INVALID_ARGUMENT", error.message ?: "invalid SAF argument", "INVALID_ARGUMENT")
    } catch (error: Exception) {
        failure(request, "SAF_FAILED", error.message ?: error.javaClass.simpleName, "SAF_FAILED")
    }

    private suspend fun pickTree(request: NativeBridgeRequest): NativeBridgeResponse {
        val activity = MainActivity.currentActivity()
            ?: return failure(request, "PROVIDER_UNAVAILABLE", "foreground activity is unavailable", "PROVIDER_UNAVAILABLE")
        val selection = activity.selectWorkspaceDocumentTree() ?: return success(
            request,
            mapOf("cancelled" to true),
        )
        val treeUri = selection.first
        val returnedFlags = selection.second
        if (!DocumentsContract.isTreeUri(treeUri)) {
            return failure(request, "INVALID_TREE_URI", "selected URI is not a document tree", "INVALID_ARGUMENT")
        }
        val requestedFlags = returnedFlags and (
            Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_GRANT_WRITE_URI_PERMISSION
        )
        val takeFlags = requestedFlags and (
            Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_GRANT_WRITE_URI_PERMISSION
        )
        if (takeFlags and Intent.FLAG_GRANT_READ_URI_PERMISSION == 0) {
            return failure(request, "PERMISSION_REVOKED", "selected directory did not grant read access", "PERMISSION_REVOKED")
        }
        resolver.takePersistableUriPermission(treeUri, takeFlags)
        val rootId = DocumentsContract.getTreeDocumentId(treeUri)
        val root = queryDocument(treeUri, rootId)
            ?: return failure(request, "FILE_NOT_FOUND", "selected directory is no longer available", "FILE_NOT_FOUND")
        return success(
            request,
            mapOf(
                "cancelled" to false,
                "grantId" to treeUri.toString(),
                "name" to root.name.ifBlank { "工作目录" },
                "readOnly" to (takeFlags and Intent.FLAG_GRANT_WRITE_URI_PERMISSION == 0),
            ),
        )
    }

    private fun grantStatus(request: NativeBridgeRequest): NativeBridgeResponse {
        val treeUri = grantUri(request)
        val persisted = resolver.persistedUriPermissions.firstOrNull { it.uri == treeUri }
        if (persisted == null || !persisted.isReadPermission) {
            return success(
                request,
                mapOf(
                    "valid" to false,
                    "readable" to false,
                    "writable" to false,
                    "providerAvailable" to true,
                    "rootExists" to false,
                ),
            )
        }
        val rootId = runCatching { DocumentsContract.getTreeDocumentId(treeUri) }.getOrNull()
        val exists = rootId != null && runCatching { queryDocument(treeUri, rootId) != null }.getOrDefault(false)
        return success(
            request,
            mapOf(
                "valid" to exists,
                "readable" to persisted.isReadPermission,
                "writable" to persisted.isWritePermission,
                "providerAvailable" to true,
                "rootExists" to exists,
            ),
        )
    }

    private fun stat(request: NativeBridgeRequest): NativeBridgeResponse {
        val treeUri = grantUri(request)
        ensureReadable(treeUri)
        val documentId = request.payload.string("documentId")
        if (documentId.isBlank()) return invalid(request, "documentId is required")
        val stat = queryDocument(treeUri, documentId)
            ?: return failure(request, "FILE_NOT_FOUND", "document not found", "FILE_NOT_FOUND")
        return success(request, stat.toStatMap())
    }

    private fun list(request: NativeBridgeRequest): NativeBridgeResponse {
        val treeUri = grantUri(request)
        ensureReadable(treeUri)
        val parentId = request.payload.string("documentId")
        if (parentId.isBlank()) return invalid(request, "documentId is required")
        val limit = request.payload.int("limit", 500).coerceIn(1, 5000)
        val entries = queryChildren(treeUri, parentId, limit).map { it.toEntryMap() }
        return success(request, mapOf("entries" to entries, "cursor" to ""))
    }

    private fun read(request: NativeBridgeRequest): NativeBridgeResponse {
        val treeUri = grantUri(request)
        ensureReadable(treeUri)
        val documentId = request.payload.string("documentId")
        if (documentId.isBlank()) return invalid(request, "documentId is required")
        val offset = request.payload.long("offset", 0L).coerceAtLeast(0L)
        val maxBytes = request.payload.long("maxBytes", 1024L * 1024L).coerceIn(1L, 16L * 1024L * 1024L)
        val stat = queryDocument(treeUri, documentId)
            ?: return failure(request, "FILE_NOT_FOUND", "document not found", "FILE_NOT_FOUND")
        if (stat.isDirectory) return failure(request, "INVALID_ARGUMENT", "cannot read a directory", "INVALID_ARGUMENT")
        val uri = documentUri(treeUri, documentId)
        val bytes = resolver.openInputStream(uri)?.use { input ->
            skipFully(input, offset)
            val buffer = ByteArray(64 * 1024)
            val output = ByteArrayOutputStream()
            var remaining = maxBytes
            while (remaining > 0) {
                val count = input.read(buffer, 0, minOf(buffer.size.toLong(), remaining).toInt())
                if (count <= 0) break
                output.write(buffer, 0, count)
                remaining -= count.toLong()
            }
            output.toByteArray()
        } ?: return failure(request, "FILE_NOT_FOUND", "unable to open document", "FILE_NOT_FOUND")
        return success(
            request,
            mapOf(
                // Go encoding/json expects []byte as base64 JSON text.
                "data" to Base64.encodeToString(bytes, Base64.NO_WRAP),
                "resource" to "",
                "isText" to isTextMime(stat.mimeType, stat.name),
            ),
        )
    }

    private fun write(request: NativeBridgeRequest): NativeBridgeResponse {
        val treeUri = grantUri(request)
        ensureWritable(treeUri)
        val documentId = request.payload.string("documentId")
        if (documentId.isBlank()) return invalid(request, "documentId is required")
        val source = request.payload.map("source")
        val bytes = source.decodeBytes("Stream", "stream")
            ?: return invalid(request, "source.stream is required")
        val uri = documentUri(treeUri, documentId)
        resolver.openOutputStream(uri, "wt")?.use { it.write(bytes) }
            ?: return failure(request, "WRITE_FAILED", "unable to open document for writing", "WRITE_FAILED")
        val stat = queryDocument(treeUri, documentId)
            ?: return failure(request, "FILE_NOT_FOUND", "written document is no longer available", "FILE_NOT_FOUND")
        return success(request, stat.toStatMap())
    }

    private fun mkdir(request: NativeBridgeRequest): NativeBridgeResponse {
        val treeUri = grantUri(request)
        ensureWritable(treeUri)
        val input = request.payload.map("input")
        val parentId = input.stringAny("ParentDocumentID", "parentDocumentId")
        val displayName = input.stringAny("DisplayName", "displayName")
        if (parentId.isBlank() || displayName.isBlank()) return invalid(request, "parentDocumentId and displayName are required")
        val created = DocumentsContract.createDocument(
            resolver,
            documentUri(treeUri, parentId),
            DocumentsContract.Document.MIME_TYPE_DIR,
            displayName,
        ) ?: return failure(request, "WRITE_FAILED", "provider refused to create directory", "WRITE_FAILED")
        return success(request, requireDocument(treeUri, DocumentsContract.getDocumentId(created)).toStatMap())
    }

    private fun createFile(request: NativeBridgeRequest): NativeBridgeResponse {
        val treeUri = grantUri(request)
        ensureWritable(treeUri)
        val input = request.payload.map("input")
        val parentId = input.stringAny("ParentDocumentID", "parentDocumentId")
        val displayName = input.stringAny("DisplayName", "displayName")
        val mimeType = input.stringAny("MIMEType", "mimeType").ifBlank { "application/octet-stream" }
        if (parentId.isBlank() || displayName.isBlank()) return invalid(request, "parentDocumentId and displayName are required")
        val created = DocumentsContract.createDocument(
            resolver,
            documentUri(treeUri, parentId),
            mimeType,
            displayName,
        ) ?: return failure(request, "WRITE_FAILED", "provider refused to create file", "WRITE_FAILED")
        return success(request, requireDocument(treeUri, DocumentsContract.getDocumentId(created)).toStatMap())
    }

    private fun rename(request: NativeBridgeRequest): NativeBridgeResponse {
        val treeUri = grantUri(request)
        ensureWritable(treeUri)
        val documentId = request.payload.string("documentId")
        val newName = request.payload.string("newName")
        if (documentId.isBlank() || newName.isBlank()) return invalid(request, "documentId and newName are required")
        val renamed = DocumentsContract.renameDocument(resolver, documentUri(treeUri, documentId), newName)
            ?: return failure(request, "WRITE_FAILED", "provider refused to rename document", "WRITE_FAILED")
        return success(request, requireDocument(treeUri, DocumentsContract.getDocumentId(renamed)).toStatMap())
    }

    private fun copy(request: NativeBridgeRequest): NativeBridgeResponse {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.N) {
            return failure(request, "OPERATION_NOT_SUPPORTED", "document copy requires Android 7.0 or newer", "OPERATION_NOT_SUPPORTED")
        }
        val treeUri = grantUri(request)
        ensureWritable(treeUri)
        val documentId = request.payload.string("documentId")
        val targetParentId = request.payload.string("targetParentDocumentId")
        if (documentId.isBlank() || targetParentId.isBlank()) return invalid(request, "documentId and targetParentDocumentId are required")
        val copied = DocumentsContract.copyDocument(
            resolver,
            documentUri(treeUri, documentId),
            documentUri(treeUri, targetParentId),
        ) ?: return failure(request, "OPERATION_NOT_SUPPORTED", "provider does not support copy", "OPERATION_NOT_SUPPORTED")
        return success(request, requireDocument(treeUri, DocumentsContract.getDocumentId(copied)).toStatMap())
    }

    private fun move(request: NativeBridgeRequest): NativeBridgeResponse {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.N) {
            return failure(request, "OPERATION_NOT_SUPPORTED", "document move requires Android 7.0 or newer", "OPERATION_NOT_SUPPORTED")
        }
        val treeUri = grantUri(request)
        ensureWritable(treeUri)
        val documentId = request.payload.string("documentId")
        val targetParentId = request.payload.string("targetParentDocumentId")
        if (documentId.isBlank() || targetParentId.isBlank()) return invalid(request, "documentId and targetParentDocumentId are required")
        val sourceParentId = findParentDocumentId(treeUri, documentId)
            ?: return failure(request, "FILE_NOT_FOUND", "source parent was not found inside granted tree", "FILE_NOT_FOUND")
        val moved = DocumentsContract.moveDocument(
            resolver,
            documentUri(treeUri, documentId),
            documentUri(treeUri, sourceParentId),
            documentUri(treeUri, targetParentId),
        ) ?: return failure(request, "OPERATION_NOT_SUPPORTED", "provider does not support move", "OPERATION_NOT_SUPPORTED")
        return success(request, requireDocument(treeUri, DocumentsContract.getDocumentId(moved)).toStatMap())
    }

    private fun delete(request: NativeBridgeRequest): NativeBridgeResponse {
        val treeUri = grantUri(request)
        ensureWritable(treeUri)
        val documentId = request.payload.string("documentId")
        if (documentId.isBlank()) return invalid(request, "documentId is required")
        if (!DocumentsContract.deleteDocument(resolver, documentUri(treeUri, documentId))) {
            return failure(request, "WRITE_FAILED", "provider refused to delete document", "WRITE_FAILED")
        }
        return success(request, emptyMap())
    }

    private fun resolvePath(request: NativeBridgeRequest): NativeBridgeResponse {
        val treeUri = grantUri(request)
        ensureReadable(treeUri)
        val relativePath = request.payload.string("relativePath").replace('\\', '/').trim('/')
        if (relativePath.split('/').any { it == "." || it == ".." }) {
            return invalid(request, "relativePath may not contain traversal segments")
        }
        var currentId = DocumentsContract.getTreeDocumentId(treeUri)
        var current = requireDocument(treeUri, currentId)
        if (relativePath.isNotEmpty()) {
            for (segment in relativePath.split('/').filter { it.isNotBlank() }) {
                current = queryChildren(treeUri, currentId, 5000).firstOrNull { it.name == segment }
                    ?: return failure(request, "FILE_NOT_FOUND", "path segment not found: $segment", "FILE_NOT_FOUND")
                currentId = current.documentId
            }
        }
        return success(
            request,
            mapOf(
                "grantId" to treeUri.toString(),
                "documentId" to current.documentId,
                "name" to current.name,
                "mimeType" to current.mimeType,
                "flags" to current.flags,
                "isDirectory" to current.isDirectory,
            ),
        )
    }

    private fun grantUri(request: NativeBridgeRequest): Uri {
        val grantId = request.payload.string("grantId")
        require(grantId.isNotBlank()) { "grantId is required" }
        val uri = Uri.parse(grantId)
        require(DocumentsContract.isTreeUri(uri)) { "grantId is not a document tree URI" }
        return uri
    }

    private fun ensureReadable(treeUri: Uri) {
        val permission = resolver.persistedUriPermissions.firstOrNull { it.uri == treeUri }
        if (permission == null || !permission.isReadPermission) throw SecurityException("persisted read permission is unavailable")
    }

    private fun ensureWritable(treeUri: Uri) {
        val permission = resolver.persistedUriPermissions.firstOrNull { it.uri == treeUri }
        if (permission == null || !permission.isWritePermission) throw SecurityException("persisted write permission is unavailable")
    }

    private fun documentUri(treeUri: Uri, documentId: String): Uri =
        DocumentsContract.buildDocumentUriUsingTree(treeUri, documentId)

    private fun requireDocument(treeUri: Uri, documentId: String): DocumentInfo =
        queryDocument(treeUri, documentId) ?: throw IllegalArgumentException("document not found: $documentId")

    private fun queryDocument(treeUri: Uri, documentId: String): DocumentInfo? =
        resolver.query(documentUri(treeUri, documentId), PROJECTION, null, null, null)?.use { cursor ->
            if (!cursor.moveToFirst()) return@use null
            cursor.toDocumentInfo()
        }

    private fun queryChildren(treeUri: Uri, parentId: String, limit: Int): List<DocumentInfo> {
        val childUri = DocumentsContract.buildChildDocumentsUriUsingTree(treeUri, parentId)
        return resolver.query(childUri, PROJECTION, null, null, null)?.use { cursor ->
            val result = ArrayList<DocumentInfo>(minOf(cursor.count.coerceAtLeast(0), limit))
            while (cursor.moveToNext() && result.size < limit) {
                result.add(cursor.toDocumentInfo())
            }
            result
        } ?: emptyList()
    }

    private fun findParentDocumentId(treeUri: Uri, targetDocumentId: String): String? {
        val rootId = DocumentsContract.getTreeDocumentId(treeUri)
        if (targetDocumentId == rootId) return null
        val queue = ArrayDeque<String>()
        val seen = HashSet<String>()
        queue.add(rootId)
        var visited = 0
        while (queue.isNotEmpty() && visited < 10000) {
            val parentId = queue.removeFirst()
            if (!seen.add(parentId)) continue
            visited++
            for (child in queryChildren(treeUri, parentId, 5000)) {
                if (child.documentId == targetDocumentId) return parentId
                if (child.isDirectory) queue.addLast(child.documentId)
            }
        }
        return null
    }

    private fun Cursor.toDocumentInfo(): DocumentInfo {
        val documentId = getString(indexOrThrow(DocumentsContract.Document.COLUMN_DOCUMENT_ID)) ?: ""
        val name = getString(indexOrThrow(DocumentsContract.Document.COLUMN_DISPLAY_NAME)) ?: ""
        val mime = getString(indexOrThrow(DocumentsContract.Document.COLUMN_MIME_TYPE)) ?: "application/octet-stream"
        val flags = getLongOrZero(DocumentsContract.Document.COLUMN_FLAGS)
        val size = getLongOrNull(DocumentsContract.Document.COLUMN_SIZE)
        val modified = getLongOrNull(DocumentsContract.Document.COLUMN_LAST_MODIFIED)
        return DocumentInfo(
            documentId = documentId,
            name = name,
            mimeType = mime,
            flags = flags,
            size = size,
            modifiedMillis = modified,
            isDirectory = mime == DocumentsContract.Document.MIME_TYPE_DIR,
            isVirtual = flags and DocumentsContract.Document.FLAG_VIRTUAL_DOCUMENT.toLong() != 0L,
        )
    }

    private fun Cursor.indexOrThrow(column: String): Int = getColumnIndex(column).also {
        if (it < 0) throw IllegalStateException("document provider omitted required column: $column")
    }

    private fun Cursor.getLongOrNull(column: String): Long? {
        val index = getColumnIndex(column)
        if (index < 0 || isNull(index)) return null
        return getLong(index)
    }

    private fun Cursor.getLongOrZero(column: String): Long = getLongOrNull(column) ?: 0L

    private fun skipFully(input: java.io.InputStream, bytes: Long) {
        var remaining = bytes
        while (remaining > 0) {
            val skipped = input.skip(remaining)
            if (skipped > 0) {
                remaining -= skipped
            } else if (input.read() >= 0) {
                remaining--
            } else {
                break
            }
        }
    }

    private fun isTextMime(mime: String, name: String): Boolean {
        if (mime.startsWith("text/")) return true
        if (mime in setOf("application/json", "application/xml", "application/javascript", "application/x-yaml")) return true
        val lower = name.lowercase()
        return lower.endsWith(".md") || lower.endsWith(".txt") || lower.endsWith(".json") || lower.endsWith(".yaml") ||
            lower.endsWith(".yml") || lower.endsWith(".toml") || lower.endsWith(".xml") || lower.endsWith(".csv") ||
            lower.endsWith(".go") || lower.endsWith(".dart") || lower.endsWith(".kt") || lower.endsWith(".java") ||
            lower.endsWith(".js") || lower.endsWith(".ts") || lower.endsWith(".tsx") || lower.endsWith(".jsx") ||
            lower.endsWith(".vue") || lower.endsWith(".html") || lower.endsWith(".css") || lower.endsWith(".scss") ||
            lower.endsWith(".py") || lower.endsWith(".rs") || lower.endsWith(".c") || lower.endsWith(".h") || lower.endsWith(".cpp")
    }

    private fun success(request: NativeBridgeRequest, result: Map<String, Any?>): NativeBridgeResponse = NativeBridgeResponse(
        protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
        requestId = request.requestId,
        status = NativeBridgeProtocol.STATUS_SUCCESS,
        result = result,
    )

    private fun invalid(request: NativeBridgeRequest, message: String): NativeBridgeResponse =
        failure(request, "INVALID_ARGUMENT", message, "INVALID_ARGUMENT")

    private fun failure(
        request: NativeBridgeRequest,
        code: String,
        message: String,
        domainCode: String? = code,
    ): NativeBridgeResponse = NativeBridgeResponse(
        protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
        requestId = request.requestId,
        status = NativeBridgeProtocol.STATUS_ERROR,
        error = NativeBridgeError(code = code, message = message, domainCode = domainCode),
    )

    private data class DocumentInfo(
        val documentId: String,
        val name: String,
        val mimeType: String,
        val flags: Long,
        val size: Long?,
        val modifiedMillis: Long?,
        val isDirectory: Boolean,
        val isVirtual: Boolean,
    ) {
        fun toStatMap(): Map<String, Any?> = mapOf(
            "name" to name,
            "mimeType" to mimeType,
            "sizeBytes" to size,
            "modifiedAt" to modifiedMillis?.let(::formatTimestamp),
            "flags" to flags,
            "isDirectory" to isDirectory,
            "isVirtual" to isVirtual,
        )

        fun toEntryMap(): Map<String, Any?> = toStatMap() + mapOf("documentId" to documentId)
    }

    companion object {
        private val RFC3339_FORMAT = SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss.SSS'Z'", Locale.US).apply {
            timeZone = TimeZone.getTimeZone("UTC")
        }

        private fun formatTimestamp(epochMillis: Long): String = synchronized(RFC3339_FORMAT) {
            RFC3339_FORMAT.format(Date(epochMillis))
        }

        private val PROJECTION = arrayOf(
            DocumentsContract.Document.COLUMN_DOCUMENT_ID,
            DocumentsContract.Document.COLUMN_DISPLAY_NAME,
            DocumentsContract.Document.COLUMN_MIME_TYPE,
            DocumentsContract.Document.COLUMN_SIZE,
            DocumentsContract.Document.COLUMN_LAST_MODIFIED,
            DocumentsContract.Document.COLUMN_FLAGS,
        )

        const val OP_PICK_TREE = "workspace.saf.pick_tree"
        const val OP_GRANT_STATUS = "workspace.saf.grant_status"
        const val OP_STAT = "workspace.saf.stat"
        const val OP_LIST = "workspace.saf.list"
        const val OP_READ = "workspace.saf.read"
        const val OP_WRITE = "workspace.saf.write"
        const val OP_MKDIR = "workspace.saf.mkdir"
        const val OP_RENAME = "workspace.saf.rename"
        const val OP_MOVE = "workspace.saf.move"
        const val OP_COPY = "workspace.saf.copy"
        const val OP_DELETE = "workspace.saf.delete"
        const val OP_RESOLVE_PATH = "workspace.saf.resolve_path"
        const val OP_CREATE_FILE = "workspace.saf.create_file"
    }
}

private fun Map<String, Any?>.string(key: String): String = this[key]?.toString()?.trim().orEmpty()

private fun Map<String, Any?>.int(key: String, fallback: Int): Int = when (val value = this[key]) {
    is Number -> value.toInt()
    is String -> value.toIntOrNull() ?: fallback
    else -> fallback
}

private fun Map<String, Any?>.long(key: String, fallback: Long): Long = when (val value = this[key]) {
    is Number -> value.toLong()
    is String -> value.toLongOrNull() ?: fallback
    else -> fallback
}

@Suppress("UNCHECKED_CAST")
private fun Map<String, Any?>.map(key: String): Map<String, Any?> {
    val value = this[key]
    if (value !is Map<*, *>) return emptyMap()
    return value.entries.associate { (k, v) -> k.toString() to v }
}

private fun Map<String, Any?>.stringAny(vararg keys: String): String {
    for (key in keys) {
        val value = this[key]?.toString()?.trim().orEmpty()
        if (value.isNotEmpty()) return value
    }
    return ""
}

private fun Map<String, Any?>.decodeBytes(vararg keys: String): ByteArray? {
    for (key in keys) {
        when (val value = this[key]) {
            is ByteArray -> return value
            is String -> return runCatching { Base64.decode(value, Base64.DEFAULT) }.getOrNull()
            is List<*> -> return runCatching { value.map { (it as Number).toByte() }.toByteArray() }.getOrNull()
        }
    }
    return null
}
