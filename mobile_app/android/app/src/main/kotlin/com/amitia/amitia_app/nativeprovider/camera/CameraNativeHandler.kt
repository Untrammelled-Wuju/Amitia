package com.amitia.amitia_app.nativeprovider.camera

import android.content.Context
import android.content.pm.PackageManager
import android.hardware.camera2.CameraCharacteristics
import android.hardware.camera2.CameraManager
import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse

internal class CameraNativeHandler(
    private val context: Context,
) : AndroidNativeOperationHandler {

    override val operations: Set<String> = setOf(OP_STATUS, OP_LIST, OP_CAPTURE)

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse {
        return when (request.operation) {
            OP_STATUS -> handleStatus(request)
            OP_LIST -> handleList(request)
            OP_CAPTURE -> handleCapture(request)
            else -> unsupportedOperation(request)
        }
    }

    private fun handleStatus(request: NativeBridgeRequest): NativeBridgeResponse {
        val state = detectCameraState()
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "supported" to state.supported,
                "hasCamera" to state.hasCamera,
                "hasFrontCamera" to state.hasFrontCamera,
                "hasBackCamera" to state.hasBackCamera,
                "camera2Level" to state.camera2Level,
                "state" to state.state,
                "reason" to state.reason,
            ),
        )
    }

    private fun handleList(request: NativeBridgeRequest): NativeBridgeResponse {
        val cameras = queryCameras()
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "cameras" to cameras.map { cam ->
                    mapOf(
                        "cameraId" to cam.cameraId,
                        "facing" to cam.facing,
                        "supportedSizes" to cam.supportedSizes,
                    )
                },
            ),
        )
    }

    private fun handleCapture(request: NativeBridgeRequest): NativeBridgeResponse {
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_ERROR,
            error = NativeBridgeError(
                code = "CAMERA_NOT_IMPLEMENTED",
                message = "camera capture requires CameraX/Camera2 integration",
            ),
        )
    }

    private fun detectCameraState(): CameraCapabilityState {
        val hasCameraFeature = try {
            context.packageManager.hasSystemFeature(PackageManager.FEATURE_CAMERA_ANY)
        } catch (_: Exception) {
            false
        }

        if (!hasCameraFeature) {
            return CameraCapabilityState(
                supported = false,
                hasCamera = false,
                state = "unsupported",
                reason = "no camera feature on this device",
            )
        }

        val cameras = queryCameras()
        val hasFront = cameras.any { it.facing == "front" }
        val hasBack = cameras.any { it.facing == "back" }

        return CameraCapabilityState(
            supported = true,
            hasCamera = cameras.isNotEmpty(),
            hasFrontCamera = hasFront,
            hasBackCamera = hasBack,
            camera2Level = "limited",
            state = if (cameras.isNotEmpty()) "available" else "unavailable",
            reason = if (cameras.isNotEmpty()) "" : "no camera devices found",
        )
    }

    private fun queryCameras(): List<CameraInfo> {
        return try {
            val cameraManager = context.getSystemService(Context.CAMERA_SERVICE) as? CameraManager
                ?: return emptyList()
            cameraManager.cameraIdList.mapNotNull { id ->
                try {
                    val characteristics = cameraManager.getCameraCharacteristics(id)
                    val facing = characteristics.get(CameraCharacteristics.LENS_FACING)
                    val facingStr = when (facing) {
                        CameraCharacteristics.LENS_FACING_FRONT -> "front"
                        CameraCharacteristics.LENS_FACING_BACK -> "back"
                        else -> "external"
                    }
                    CameraInfo(
                        cameraId = id,
                        facing = facingStr,
                        supportedSizes = emptyList(),
                    )
                } catch (_: Exception) {
                    null
                }
            }
        } catch (_: Exception) {
            emptyList()
        }
    }

    private fun unsupportedOperation(request: NativeBridgeRequest): NativeBridgeResponse {
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_ERROR,
            error = NativeBridgeError(
                code = NativeBridgeProtocol.ERR_OPERATION_NOT_SUPPORTED,
                message = "unknown camera operation: ${request.operation}",
            ),
        )
    }

    companion object {
        const val OP_STATUS = "media.camera.status"
        const val OP_LIST = "media.camera.list"
        const val OP_CAPTURE = "media.camera.capture"
    }
}
