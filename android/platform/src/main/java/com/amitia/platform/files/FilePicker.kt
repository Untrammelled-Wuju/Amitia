package com.amitia.platform.files

import android.net.Uri
import kotlinx.coroutines.flow.Flow

interface FilePicker {

    suspend fun pickFile(mimeTypes: List<String>): PickResult

    suspend fun pickMultipleFiles(mimeTypes: List<String>, maxCount: Int = 10): List<PickResult>

    suspend fun pickImage(): PickResult

    suspend fun pickMultipleImages(maxCount: Int = 10): List<PickResult>

    suspend fun pickAudio(): PickResult

    suspend fun pickDirectory(): DirectoryPickResult

    suspend fun createDocument(mimeType: String, suggestedName: String): PickResult

    fun observePicks(): Flow<PickResult>

    data class PickResult(
        val uri: Uri?,
        val fileName: String?,
        val mimeType: String?,
        val sizeBytes: Long?,
        val success: Boolean,
        val error: String? = null
    )

    data class DirectoryPickResult(
        val uri: Uri?,
        val directoryName: String?,
        val success: Boolean,
        val error: String? = null
    )

    suspend fun resolveMetadata(uri: Uri): FileMetadata?

    data class FileMetadata(
        val fileName: String,
        val mimeType: String,
        val sizeBytes: Long,
        val lastModified: Long,
        val isDirectory: Boolean
    )
}
