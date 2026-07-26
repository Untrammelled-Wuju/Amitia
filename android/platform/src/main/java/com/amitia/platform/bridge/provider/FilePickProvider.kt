package com.amitia.platform.bridge.provider

import android.net.Uri
import android.provider.OpenableColumns
import com.amitia.platform.bridge.CapabilityProvider
import com.amitia.platform.bridge.NativeActionRequest
import com.amitia.platform.bridge.NativeActionResult
import com.amitia.platform.files.FilePicker
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class FilePickProvider @Inject constructor(
    private val filePicker: FilePicker
) : CapabilityProvider {

    override fun action(): String = "file_pick"

    override fun requiredPermission(): String? = null

    override suspend fun execute(request: NativeActionRequest): NativeActionResult {
        val mimeTypes = request.params["mime_types"]?.split(",")?.filter { it.isNotBlank() }
            ?: listOf("*/*")
        val result = filePicker.pickFile(mimeTypes)
        return if (result.success && result.uri != null) {
            NativeActionResult.Success(
                mapOf(
                    "uri" to result.uri.toString(),
                    "name" to (result.fileName ?: ""),
                    "mime" to (result.mimeType ?: ""),
                    "size" to (result.sizeBytes?.toString() ?: "")
                )
            )
        } else {
            NativeActionResult.Failed(result.error ?: "file_pick_failed")
        }
    }
}
