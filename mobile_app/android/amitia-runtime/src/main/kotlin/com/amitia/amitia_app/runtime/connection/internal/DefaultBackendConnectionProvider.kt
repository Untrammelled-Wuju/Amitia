package com.amitia.amitia_app.runtime.connection.internal

import com.amitia.amitia_app.runtime.api.RuntimeComponentSnapshot
import com.amitia.amitia_app.runtime.api.RuntimeComponentState
import com.amitia.amitia_app.runtime.api.RuntimeSnapshot
import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.connection.BackendConnectionAvailability
import com.amitia.amitia_app.runtime.connection.BackendConnectionDescriptor
import com.amitia.amitia_app.runtime.connection.BackendConnectionError
import com.amitia.amitia_app.runtime.connection.BackendConnectionErrorCode
import com.amitia.amitia_app.runtime.connection.BackendConnectionProvider
import com.amitia.amitia_app.runtime.connection.BackendEndpointPolicy
import com.amitia.amitia_app.runtime.connection.embeddedAndroidBackendPolicy
import java.util.concurrent.atomic.AtomicLong

internal class DefaultBackendConnectionProvider(
    private val snapshotProvider: () -> RuntimeSnapshot,
    private val dataRootProvider: () -> String,
    private val credentialResolver: RuntimeCredentialResolver = RuntimeCredentialResolver(),
    private val policy: BackendEndpointPolicy = embeddedAndroidBackendPolicy(),
) : BackendConnectionProvider {

    private val lastSeenRuntimeGeneration = AtomicLong(0L)

    override fun current(): BackendConnectionAvailability {
        val validationError = BackendConnectionValidator.validate(policy)
        if (validationError != null) {
            return BackendConnectionAvailability.Unavailable
        }

        val snapshot = snapshotProvider()
        if (snapshot.state != RuntimeState.READY && snapshot.state != RuntimeState.DEGRADED) {
            return BackendConnectionAvailability.Unavailable
        }

        if (!isBackendComponentReady(snapshot.components)) {
            return BackendConnectionAvailability.Unavailable
        }

        if (snapshot.generation <= 0) {
            return BackendConnectionAvailability.Unavailable
        }

        val credentialResult = credentialResolver.resolve(dataRootProvider())
        if (credentialResult.isFailure) {
            return BackendConnectionAvailability.Unavailable
        }
        val credential = credentialResult.getOrNull() ?: return BackendConnectionAvailability.Unavailable

        val runtimeGeneration = snapshot.generation
        lastSeenRuntimeGeneration.set(runtimeGeneration)
        val descriptor = BackendConnectionMapper.buildDescriptor(
            policy = policy,
            generation = runtimeGeneration,
            credential = credential,
        )

        return BackendConnectionAvailability.Available(descriptor)
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
