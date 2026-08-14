package com.amitia.amitia_app.nativeprovider.clipboard

import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse

internal class ClipboardNativeHandlerAdapter(
    private val delegate: ClipboardNativeHandler,
) : AndroidNativeOperationHandler {

    override fun supports(operation: String): Boolean {
        return operation == ClipboardNativeHandler.OP_STATUS ||
            operation == ClipboardNativeHandler.OP_READ ||
            operation == ClipboardNativeHandler.OP_WRITE ||
            operation == ClipboardNativeHandler.OP_CLEAR
    }

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse {
        val domainRequest = ClipboardNativeRequest(
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
