package com.amitia.platform.bridge.provider

import com.amitia.platform.bridge.CapabilityProvider
import com.amitia.platform.bridge.NativeActionRequest
import com.amitia.platform.bridge.NativeActionResult
import com.amitia.platform.files.FilePicker
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class ImagePickProvider @Inject constructor(
    private val filePicker: FilePicker
) : CapabilityProvider {

    override fun action(): String = "image_pick"

    override fun requiredPermission(): String? = null

    override suspend fun execute(request: NativeActionRequest): NativeActionResult {
        val multiple = request.params["multiple"]?.toBooleanStrictOrNull() ?: false
        return if (multiple) {
            val max = request.params["max"]?.toIntOrNull() ?: 10
            val results = filePicker.pickMultipleImages(max)
            val data = results.mapIndexed { index, r ->
                "uri_$index" to (r.uri?.toString() ?: "")
            }.toMap()
            NativeActionResult.Success(data)
        } else {
            val result = filePicker.pickImage()
            if (result.success && result.uri != null) {
                NativeActionResult.Success(
                    mapOf(
                        "uri" to result.uri.toString(),
                        "name" to (result.fileName ?: ""),
                        "mime" to (result.mimeType ?: "")
                    )
                )
            } else {
                NativeActionResult.Failed(result.error ?: "image_pick_failed")
            }
        }
    }
}
