package com.amitia.amitia_app.runtime.connection

internal sealed class BackendConnectionAvailability {
    data object Unavailable : BackendConnectionAvailability()
    data class Available(val descriptor: BackendConnectionDescriptor) : BackendConnectionAvailability()
    data object Resolving : BackendConnectionAvailability()
}
