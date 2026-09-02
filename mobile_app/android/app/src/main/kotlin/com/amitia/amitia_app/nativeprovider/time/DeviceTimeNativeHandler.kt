package com.amitia.amitia_app.nativeprovider.time

import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse
import java.util.TimeZone

internal class DeviceTimeNativeHandler : AndroidNativeOperationHandler {
    override val operations: Set<String> = setOf(OP_GET_TIMEZONE)

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse {
        if (request.operation != OP_GET_TIMEZONE) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = NativeBridgeProtocol.ERR_OPERATION_NOT_SUPPORTED,
                    message = "unknown device time operation: ${request.operation}",
                ),
            )
        }
        val timezone = TimeZone.getDefault().id.orEmpty().trim()
        if (timezone.isEmpty()) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "TIMEZONE_UNAVAILABLE",
                    message = "device timezone is unavailable",
                ),
            )
        }
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "timezone" to timezone,
                "ianaTimezone" to timezone,
                "source" to "android.system",
            ),
        )
    }

    companion object {
        const val OP_GET_TIMEZONE = "device.timezone.get"
    }
}
