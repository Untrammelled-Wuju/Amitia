package com.amitia.platform.files

import android.content.Context
import android.net.Uri
import android.provider.OpenableColumns
import com.amitia.platform.bridge.ActivityResultBridge
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.asSharedFlow

@Singleton
class FilePickerImpl @Inject constructor(
    @ApplicationContext private val context: Context,
    private val activityResultBridge: ActivityResultBridge
) : FilePicker {

    private val pickFlow = MutableSharedFlow<FilePicker.PickResult>(extraBufferCapacity = 16)

    override suspend fun pickFile(mimeTypes: List<String>): FilePicker.PickResult {
        if (!activityResultBridge.hasActivity()) {
            return failureResult("file_pick_no_activity", mimeTypes.firstOrNull())
        }
        return runCatching {
            val uri = activityResultBridge.pickFile(mimeTypes.toTypedArray())
            uri?.let { buildResult(it) } ?: failureResult("file_pick_cancelled", mimeTypes.firstOrNull())
        }.getOrElse { failureResult(it.message ?: "file_pick_failed", mimeTypes.firstOrNull()) }
    }

    override suspend fun pickMultipleFiles(mimeTypes: List<String>, maxCount: Int): List<FilePicker.PickResult> {
        if (!activityResultBridge.hasActivity()) return emptyList()
        return runCatching {
            val uris = activityResultBridge.pickMultipleFiles(mimeTypes.toTypedArray(), maxCount)
            uris.take(maxCount).map { buildResult(it) }
        }.getOrElse { emptyList() }
    }

    override suspend fun pickImage(): FilePicker.PickResult {
        if (!activityResultBridge.hasActivity()) {
            return failureResult("image_pick_no_activity", "image/*")
        }
        return runCatching {
            val uri = activityResultBridge.pickImage()
            uri?.let { buildResult(it) } ?: failureResult("image_pick_cancelled", "image/*")
        }.getOrElse { failureResult(it.message ?: "image_pick_failed", "image/*") }
    }

    override suspend fun pickMultipleImages(maxCount: Int): List<FilePicker.PickResult> {
        return pickMultipleFiles(listOf("image/*"), maxCount)
    }

    override suspend fun pickAudio(): FilePicker.PickResult {
        if (!activityResultBridge.hasActivity()) {
            return failureResult("audio_pick_no_activity", "audio/*")
        }
        return runCatching {
            val uri = activityResultBridge.pickAudio()
            uri?.let { buildResult(it) } ?: failureResult("audio_pick_cancelled", "audio/*")
        }.getOrElse { failureResult(it.message ?: "audio_pick_failed", "audio/*") }
    }

    override suspend fun pickDirectory(): FilePicker.DirectoryPickResult {
        return FilePicker.DirectoryPickResult(
            uri = null,
            directoryName = null,
            success = false,
            error = "dir_pick_not_supported"
        )
    }

    override suspend fun createDocument(mimeType: String, suggestedName: String): FilePicker.PickResult {
        return failureResult("create_document_not_supported", mimeType)
    }

    override fun observePicks(): SharedFlow<FilePicker.PickResult> = pickFlow.asSharedFlow()

    override suspend fun resolveMetadata(uri: Uri): FilePicker.FileMetadata? {
        return runCatching {
            val resolver = context.contentResolver
            val mime = resolver.getType(uri) ?: "application/octet-stream"
            var name: String? = null
            var size: Long = 0L
            var lastModified = 0L
            resolver.query(uri, null, null, null, null)?.use { cursor ->
                val nameIndex = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
                val sizeIndex = cursor.getColumnIndex(OpenableColumns.SIZE)
                if (cursor.moveToFirst()) {
                    if (nameIndex >= 0) name = cursor.getString(nameIndex)
                    if (sizeIndex >= 0) size = cursor.getLong(sizeIndex)
                }
            }
            FilePicker.FileMetadata(
                fileName = name ?: uri.lastPathSegment.orEmpty(),
                mimeType = mime,
                sizeBytes = size,
                lastModified = lastModified,
                isDirectory = false
            )
        }.getOrNull()
    }

    private fun buildResult(uri: Uri): FilePicker.PickResult {
        val metadata = runCatching {
            kotlinx.coroutines.runBlocking { resolveMetadata(uri) }
        }.getOrNull()
        return FilePicker.PickResult(
            uri = uri,
            fileName = metadata?.fileName,
            mimeType = metadata?.mimeType,
            sizeBytes = metadata?.sizeBytes,
            success = true
        )
    }

    private fun failureResult(error: String, mimeType: String?): FilePicker.PickResult {
        return FilePicker.PickResult(
            uri = null,
            fileName = null,
            mimeType = mimeType,
            sizeBytes = null,
            success = false,
            error = error
        )
    }
}
