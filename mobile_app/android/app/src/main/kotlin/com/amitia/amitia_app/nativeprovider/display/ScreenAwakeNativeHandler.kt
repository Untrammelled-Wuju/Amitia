package com.amitia.amitia_app.nativeprovider.display

import android.content.Context
import com.amitia.amitia_app.MainActivity
import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

internal class ScreenAwakeNativeHandler(private val context: Context) : AndroidNativeOperationHandler {
    override val operations = setOf("display.keep_awake.status", "display.keep_awake.set")

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse {
        if (request.operation == "display.keep_awake.set") {
            val enabled = request.payload["enabled"] as? Boolean
                ?: return error(request, "INVALID_REQUEST", "enabled must be a boolean")
            val saved = withContext(Dispatchers.IO) { ScreenAwakeController.setEnabled(context, enabled) }
            if (!saved) return error(request, "PREFERENCE_SAVE_FAILED", "screen awake preference could not be saved")
        } else if (request.operation != "display.keep_awake.status") {
            return error(request, "OPERATION_NOT_SUPPORTED", "unsupported screen awake operation")
        }
        return withContext(Dispatchers.Main.immediate) {
            val activity = MainActivity.currentActivity()
            if (activity != null) ScreenAwakeController.apply(activity, true)
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_SUCCESS,
                result = mapOf("enabled" to ScreenAwakeController.enabled(context)),
            )
        }
    }

    private fun error(request: NativeBridgeRequest, code: String, message: String) = NativeBridgeResponse(
        protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
        requestId = request.requestId,
        status = NativeBridgeProtocol.STATUS_ERROR,
        error = NativeBridgeError(code, message),
    )
}
