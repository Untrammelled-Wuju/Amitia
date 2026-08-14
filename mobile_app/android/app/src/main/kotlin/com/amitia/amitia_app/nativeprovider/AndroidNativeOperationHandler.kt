package com.amitia.amitia_app.nativeprovider

import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse

internal interface AndroidNativeOperationHandler {
    fun supports(operation: String): Boolean
    suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse
}
