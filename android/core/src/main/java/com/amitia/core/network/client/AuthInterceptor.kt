package com.amitia.core.network.client

import com.amitia.core.network.endpoint.RuntimeEndpointProvider
import javax.inject.Inject
import javax.inject.Singleton
import okhttp3.Interceptor
import okhttp3.Response

@Singleton
class AuthInterceptor @Inject constructor(
    private val endpointProvider: RuntimeEndpointProvider
) : Interceptor {

    override fun intercept(chain: Interceptor.Chain): Response {
        val endpoint = endpointProvider.currentEndpoint.value
        val token = endpoint.authHeader()
        val request = if (!token.isNullOrBlank()) {
            chain.request().newBuilder()
                .header(HEADER_AUTH, "$BEARER_PREFIX$token")
                .build()
        } else {
            chain.request()
        }
        return chain.proceed(request)
    }

    companion object {
        private const val HEADER_AUTH = "Authorization"
        private const val BEARER_PREFIX = "Bearer "
    }
}
