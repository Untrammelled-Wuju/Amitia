package com.amitia.amitia_app.runtime.connection

interface BackendConnectionProvider {
    fun current(): BackendConnectionAvailability
}
