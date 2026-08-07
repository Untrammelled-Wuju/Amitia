package com.amitia.amitia_app.runtime.connection.internal

import com.amitia.amitia_app.runtime.connection.BackendConnectionAvailability
import com.amitia.amitia_app.runtime.connection.BackendConnectionCredential
import com.amitia.amitia_app.runtime.connection.BackendConnectionDescriptor
import com.amitia.amitia_app.runtime.connection.BackendConnectionError
import com.amitia.amitia_app.runtime.connection.BackendConnectionErrorCode
import com.amitia.amitia_app.runtime.connection.BackendEndpointPolicy

internal object BackendConnectionMapper {
    private const val SCHEMA_VERSION = 1
    private const val LIVENESS_PATH = "/livez"
    private const val READINESS_PATH = "/readyz"
    private const val AUTH_TYPE = "local_token"
    private const val AUTH_HEADER = "X-Amitia-Local-Token"

    fun toPayload(
        available: Boolean,
        descriptor: BackendConnectionDescriptor?,
        error: BackendConnectionError?,
    ): Map<String, Any?> {
        val result = LinkedHashMap<String, Any?>()
        result["schemaVersion"] = SCHEMA_VERSION
        if (available && descriptor != null) {
            result["status"] = "available"
            result["generation"] = descriptor.generation
            val endpoint = LinkedHashMap<String, Any?>()
            endpoint["host"] = descriptor.host
            endpoint["port"] = descriptor.port
            endpoint["httpScheme"] = descriptor.httpScheme
            endpoint["webSocketScheme"] = descriptor.webSocketScheme
            endpoint["livenessPath"] = descriptor.livenessPath
            endpoint["readinessPath"] = descriptor.readinessPath
            result["endpoint"] = endpoint
            val auth = LinkedHashMap<String, Any?>()
            auth["type"] = AUTH_TYPE
            auth["header"] = AUTH_HEADER
            auth["token"] = descriptor.credential.reveal()
            result["authentication"] = auth
        } else {
            result["status"] = "unavailable"
            val errMap = LinkedHashMap<String, Any?>()
            errMap["code"] = error?.code?.name ?: BackendConnectionErrorCode.ENDPOINT_UNAVAILABLE.name
            result["error"] = errMap
        }
        return result
    }

    fun buildDescriptor(
        policy: BackendEndpointPolicy,
        generation: Long,
        credential: BackendConnectionCredential,
    ): BackendConnectionDescriptor {
        return BackendConnectionDescriptor(
            schemaVersion = SCHEMA_VERSION,
            generation = generation,
            host = policy.host,
            port = policy.port,
            httpScheme = policy.httpScheme,
            webSocketScheme = policy.webSocketScheme,
            livenessPath = LIVENESS_PATH,
            readinessPath = READINESS_PATH,
            credential = credential,
        )
    }
}
