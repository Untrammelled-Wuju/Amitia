package com.amitia.core.network.client

import com.amitia.core.network.endpoint.RuntimeEndpoint
import com.amitia.core.network.endpoint.RuntimeEndpointProvider
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.flow.Flow
import retrofit2.Retrofit

@Singleton
class AmitiaApiClient @Inject constructor(
    private val retrofitHolder: RetrofitHolder,
    private val endpointProvider: RuntimeEndpointProvider
) {

    fun <T> service(serviceClass: Class<T>): T {
        return retrofitHolder.current().create(serviceClass)
    }

    fun retrofit(): Retrofit = retrofitHolder.current()

    fun endpoint(): RuntimeEndpoint = endpointProvider.currentEndpoint.value

    fun observeEndpoint(): Flow<RuntimeEndpoint> = endpointProvider.observeEndpoint()

    fun rebuildIfNeeded() {
        val current = endpointProvider.currentEndpoint.value
        retrofitHolder.rebuild(current)
    }
}
