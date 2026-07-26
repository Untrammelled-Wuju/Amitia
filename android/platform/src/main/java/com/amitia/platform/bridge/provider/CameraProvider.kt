package com.amitia.platform.bridge.provider

import com.amitia.platform.bridge.ActivityResultBridge
import com.amitia.platform.bridge.CapabilityProvider
import com.amitia.platform.bridge.NativeActionRequest
import com.amitia.platform.bridge.NativeActionResult
import com.amitia.platform.permissions.PermissionBroker
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class CameraProvider @Inject constructor(
    private val permissionBroker: PermissionBroker,
    private val activityResultBridge: ActivityResultBridge
) : CapabilityProvider {

    override fun action(): String = "camera"

    override fun requiredPermission(): String? = PermissionBroker.Permissions.CAMERA

    override suspend fun execute(request: NativeActionRequest): NativeActionResult {
        val op = request.params["op"] ?: "status"
        return when (op) {
            "status" -> NativeActionResult.Success(
                mapOf(
                    "status" to "ready",
                    "intent" to "android.media.action.IMAGE_CAPTURE"
                )
            )
            "capture_image" -> {
                if (!activityResultBridge.hasActivity()) {
                    return NativeActionResult.Failed("camera_no_activity")
                }
                val targetUri = activityResultBridge.createTempImageUri()
                val ok = activityResultBridge.captureImage(targetUri)
                if (ok) {
                    NativeActionResult.Success(mapOf("uri" to targetUri.toString()))
                } else {
                    NativeActionResult.Failed("capture_image_failed")
                }
            }
            "capture_video" -> {
                if (!activityResultBridge.hasActivity()) {
                    return NativeActionResult.Failed("camera_no_activity")
                }
                val targetUri = activityResultBridge.createTempVideoUri()
                val ok = activityResultBridge.captureVideo(targetUri)
                if (ok) {
                    NativeActionResult.Success(mapOf("uri" to targetUri.toString()))
                } else {
                    NativeActionResult.Failed("capture_video_failed")
                }
            }
            else -> NativeActionResult.Failed("unsupported op: $op")
        }
    }
}
