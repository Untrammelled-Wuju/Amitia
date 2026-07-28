package com.amitia.runtime.extension

import com.amitia.core.network.endpoint.RuntimeEndpointProvider
import javax.inject.Inject
import javax.inject.Singleton

interface BaseUrlProvider {
    fun baseUrl(): String
}

@Singleton
class RuntimeBaseUrlProvider @Inject constructor(
    private val endpointProvider: RuntimeEndpointProvider
) : BaseUrlProvider {
    override fun baseUrl(): String =
        endpointProvider.currentEndpoint.value.baseUrl()
}
