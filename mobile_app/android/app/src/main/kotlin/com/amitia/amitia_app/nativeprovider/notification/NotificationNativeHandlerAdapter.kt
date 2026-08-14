package com.amitia.amitia_app.nativeprovider.notification

import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse

internal class NotificationNativeHandlerAdapter(
    private val delegate: NotificationNativeHandler,
) : AndroidNativeOperationHandler {

    override fun supports(operation: String): Boolean {
        return operation == NotificationNativeHandler.OP_STATUS ||
            operation == NotificationNativeHandler.OP_LIST ||
            operation == NotificationNativeHandler.OP_GET ||
            operation == NotificationNativeHandler.OP_POST ||
            operation == NotificationNativeHandler.OP_CANCEL_OWN ||
            operation == NotificationNativeHandler.OP_DISMISS ||
            operation == NotificationNativeHandler.OP_OPEN ||
            operation == NotificationNativeHandler.OP_INVOKE_ACTION
    }

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse {
        val domainRequest = NativeNotificationRequest(
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
