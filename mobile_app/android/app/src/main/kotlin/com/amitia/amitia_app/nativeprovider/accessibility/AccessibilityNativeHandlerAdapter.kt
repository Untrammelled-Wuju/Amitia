package com.amitia.amitia_app.nativeprovider.accessibility

import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse

internal class AccessibilityNativeHandlerAdapter(
    private val delegate: AccessibilityNativeHandler,
) : AndroidNativeOperationHandler {

    override fun supports(operation: String): Boolean {
        return operation == AccessibilityNativeHandler.OP_STATUS ||
            operation == AccessibilityNativeHandler.OP_OPEN_SETTINGS
    }

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse {
        val domainRequest = NativeAccessibilityRequest(
            requestId = request.requestId,
            operation = request.operation,
            payload = request.payload,
        )
        val domainResponse = delegate.execute(domainRequest)
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = domainResponse.requestId,
            status = domainResponse.status,
            result = domainResponse.result,
            error = domainResponse.error?.let {
                NativeBridgeError(
                    code = it.code,
                    message = it.message,
                    domainCode = it.domainCode,
                )
            },
        )
    }
}
