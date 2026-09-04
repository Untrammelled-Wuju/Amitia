package com.amitia.amitia_app.runtime.connection.internal

import com.amitia.amitia_app.runtime.connection.BackendConnectionError
import com.amitia.amitia_app.runtime.connection.BackendConnectionErrorCode
import com.amitia.amitia_app.runtime.connection.BackendEndpointPolicy

internal object BackendConnectionValidator {
    fun validate(policy: BackendEndpointPolicy): BackendConnectionError? {
        if (policy.host.isBlank()) {
            return BackendConnectionError(BackendConnectionErrorCode.ENDPOINT_INVALID, "host must not be blank")
        }
        if (policy.host == "0.0.0.0") {
            return BackendConnectionError(BackendConnectionErrorCode.ENDPOINT_INVALID, "host must not be 0.0.0.0")
        }
        if (policy.host == "::") {
            return BackendConnectionError(BackendConnectionErrorCode.ENDPOINT_INVALID, "host must not be ::")
        }
        if (policy.host == "localhost") {
            return BackendConnectionError(BackendConnectionErrorCode.ENDPOINT_INVALID, "host must not be localhost")
        }
        if (policy.host.contains("://")) {
            return BackendConnectionError(BackendConnectionErrorCode.ENDPOINT_INVALID, "host must not contain scheme")
        }
        if (policy.host.contains(":")) {
            return BackendConnectionError(BackendConnectionErrorCode.ENDPOINT_INVALID, "host must not contain port")
        }
        if (policy.host.contains("/")) {
            return BackendConnectionError(BackendConnectionErrorCode.ENDPOINT_INVALID, "host must not contain path")
        }
        if (policy.host.contains("?")) {
            return BackendConnectionError(BackendConnectionErrorCode.ENDPOINT_INVALID, "host must not contain query")
        }
        if (policy.host.contains("#")) {
            return BackendConnectionError(BackendConnectionErrorCode.ENDPOINT_INVALID, "host must not contain fragment")
        }
        if (policy.port < 1 || policy.port > 65535) {
            return BackendConnectionError(BackendConnectionErrorCode.ENDPOINT_INVALID, "port must be in range 1..65535")
        }
        if (policy.httpScheme != "http" && policy.httpScheme != "https") {
            return BackendConnectionError(BackendConnectionErrorCode.ENDPOINT_INVALID, "httpScheme must be http or https")
        }
        val expectedWs = if (policy.httpScheme == "http") "ws" else "wss"
        if (policy.webSocketScheme != expectedWs) {
            return BackendConnectionError(BackendConnectionErrorCode.ENDPOINT_INVALID, "webSocketScheme must match httpScheme")
        }
        return null
    }
}
