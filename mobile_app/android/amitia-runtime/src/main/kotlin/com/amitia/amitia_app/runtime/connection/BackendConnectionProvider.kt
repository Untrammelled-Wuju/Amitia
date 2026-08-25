package com.amitia.amitia_app.runtime.connection

interface BackendConnectionProvider {
    fun current(): BackendConnectionAvailability

    /** Last reason an unavailable connection was returned, when known. */
    fun lastError(): BackendConnectionError? = null
}
