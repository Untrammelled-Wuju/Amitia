package com.amitia.amitia_app.nativeprovider

import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse

internal interface AndroidNativeOperationHandler {
    val operations: Set<String>
    fun supports(operation: String): Boolean = operations.contains(operation)
    suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse
}
