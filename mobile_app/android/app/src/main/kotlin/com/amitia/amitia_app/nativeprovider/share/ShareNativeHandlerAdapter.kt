package com.amitia.amitia_app.nativeprovider.share

import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse

internal class ShareNativeHandlerAdapter(
    private val delegate: ShareNativeHandler,
) : AndroidNativeOperationHandler {

    override fun supports(operation: String): Boolean {
        return operation == ShareConstants.OP_STATUS ||
            operation == ShareConstants.OP_SEND ||
            operation == ShareConstants.OP_RECEIVE_PENDING ||
            operation == ShareConstants.OP_RECEIVE_CONSUME
    }

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse {
        val domainRequest = ShareNativeRequest(
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
