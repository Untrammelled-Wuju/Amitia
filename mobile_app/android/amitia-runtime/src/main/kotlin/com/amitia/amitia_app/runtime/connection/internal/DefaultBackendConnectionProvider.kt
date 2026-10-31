package com.amitia.amitia_app.runtime.connection.internal

import com.amitia.amitia_app.runtime.api.RuntimeComponentSnapshot
import com.amitia.amitia_app.runtime.api.RuntimeComponentState
import com.amitia.amitia_app.runtime.api.RuntimeSnapshot
import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.connection.BackendConnectionAvailability
import com.amitia.amitia_app.runtime.connection.BackendConnectionError
import com.amitia.amitia_app.runtime.connection.BackendConnectionErrorCode
import com.amitia.amitia_app.runtime.connection.BackendConnectionProvider
import com.amitia.amitia_app.runtime.connection.BackendEndpointPolicy
import com.amitia.amitia_app.runtime.connection.embeddedAndroidBackendPolicy
import org.json.JSONObject
import java.io.File
import java.util.concurrent.atomic.AtomicReference

internal class DefaultBackendConnectionProvider(
    private val snapshotProvider: () -> RuntimeSnapshot,
    private val dataRootProvider: () -> String,
    private val runRootProvider: () -> String = { "" },
    private val credentialResolver: RuntimeCredentialResolver = RuntimeCredentialResolver(),
    private val policy: BackendEndpointPolicy = embeddedAndroidBackendPolicy(),
) : BackendConnectionProvider {

    private val lastErrorRef = AtomicReference<BackendConnectionError?>(null)

    override fun current(): BackendConnectionAvailability {
        val effectivePolicy = resolveEndpointPolicy()
        val validationError = BackendConnectionValidator.validate(effectivePolicy)
        if (validationError != null) {
            return unavailable(validationError)
        }

        val snapshot = snapshotProvider()
        if (snapshot.state != RuntimeState.READY) {
            return unavailable(
                BackendConnectionError(
                    BackendConnectionErrorCode.RUNTIME_NOT_READY,
                    snapshot.lastError?.message
                        ?: "runtime is ${snapshot.state.name.lowercase()} for generation ${snapshot.generation}",
                )
            )
        }

        if (!isBackendComponentReady(snapshot.components)) {
            return unavailable(
                BackendConnectionError(
                    BackendConnectionErrorCode.BACKEND_NOT_READY,
                    "one or more required runtime backend components are not ready",
                )
            )
        }

        if (snapshot.generation <= 0) {
            return unavailable(
                BackendConnectionError(
                    BackendConnectionErrorCode.GENERATION_INVALID,
                    "runtime generation must be positive before exposing the backend connection",
                )
            )
        }

        val credentialResult = credentialResolver.resolve(dataRootProvider())
        if (credentialResult.isFailure) {
            val cause = credentialResult.exceptionOrNull()
            return unavailable(
                cause as? BackendConnectionError ?: BackendConnectionError(
                    BackendConnectionErrorCode.CREDENTIAL_UNAVAILABLE,
                    cause?.message ?: "local backend credential is unavailable",
                )
            )
        }
        val credential = credentialResult.getOrNull() ?: return unavailable(
            BackendConnectionError(
                BackendConnectionErrorCode.CREDENTIAL_UNAVAILABLE,
                "local backend credential resolver returned no credential",
            )
        )

        val descriptor = BackendConnectionMapper.buildDescriptor(
            policy = effectivePolicy,
            generation = snapshot.generation,
            credential = credential,
        )
        lastErrorRef.set(null)
        return BackendConnectionAvailability.Available(descriptor)
    }

    override fun lastError(): BackendConnectionError? = lastErrorRef.get()

    private fun resolveEndpointPolicy(): BackendEndpointPolicy {
        val runRoot = runRootProvider().trim()
        if (runRoot.isNotEmpty()) {
            val file = File(runRoot, "endpoint.json")
            if (file.isFile) {
                try {
                    val payload = JSONObject(file.readText())
                    val host = payload.optString("host", "127.0.0.1")
                    val port = payload.optInt("port", 0)
                    if (host == "127.0.0.1" && port in 1..65535) {
                        return BackendEndpointPolicy(host, port, "http", "ws")
                    }
                } catch (originalError103: Exception) {
                  originalError103.printStackTrace()

                }
            }
        }
        if (policy.port in 2..65535) return policy
        return BackendEndpointPolicy("127.0.0.1", 18899, "http", "ws")
    }

    private fun unavailable(error: BackendConnectionError): BackendConnectionAvailability {
        lastErrorRef.set(error)
        return BackendConnectionAvailability.Unavailable
    }

    private fun isBackendComponentReady(components: List<RuntimeComponentSnapshot>): Boolean {
        for (comp in components) {
            if (comp.required && comp.state != RuntimeComponentState.READY) {
                return false
            }
        }
        return true
    }
}
