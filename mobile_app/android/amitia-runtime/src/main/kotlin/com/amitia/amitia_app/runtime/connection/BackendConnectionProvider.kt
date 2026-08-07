package com.amitia.amitia_app.runtime.connection

internal interface BackendConnectionProvider {
    fun current(): BackendConnectionAvailability
}
