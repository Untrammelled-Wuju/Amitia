package com.amitia.core.network.client

import com.amitia.core.network.endpoint.RuntimeEndpoint
import com.amitia.core.network.endpoint.RuntimeEndpointProvider
import com.jakewharton.retrofit2.converter.kotlinx.serialization.asConverterFactory
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import retrofit2.Retrofit

@Module
@InstallIn(SingletonComponent::class)
object RetrofitModule {

    @Provides
    @Singleton
    fun provideJson(): Json = Json {
        ignoreUnknownKeys = true
        encodeDefaults = true
        explicitNulls = false
        coerceInputValues = true
    }

    @Provides
    @Singleton
    fun provideRetrofitHolder(
        client: OkHttpClient,
        endpointProvider: RuntimeEndpointProvider,
        json: Json
    ): RetrofitHolder {
        val holder = RetrofitHolder(client, endpointProvider, json)
        holder.rebuild(endpointProvider.currentEndpoint.value)
        return holder
    }
}

class RetrofitHolder internal constructor(
    private val client: OkHttpClient,
    private val endpointProvider: RuntimeEndpointProvider,
    private val json: Json
) {

    @Volatile
    private var currentRetrofit: Retrofit? = null

    @Volatile
    private var currentEndpoint: RuntimeEndpoint? = null

    fun current(): Retrofit {
        val endpoint = endpointProvider.currentEndpoint.value
        val cached = currentRetrofit
        val cachedEndpoint = currentEndpoint
        if (cached != null && cachedEndpoint == endpoint) {
            return cached
        }
        return rebuild(endpoint)
    }

    @Synchronized
    fun rebuild(endpoint: RuntimeEndpoint): Retrofit {
        val contentType = "application/json".toMediaType()
        val retrofit = Retrofit.Builder()
            .baseUrl(endpoint.baseUrl() + "/")
            .client(client)
            .addConverterFactory(json.asConverterFactory(contentType))
            .build()
        currentRetrofit = retrofit
        currentEndpoint = endpoint
        return retrofit
    }

    fun currentEndpoint(): RuntimeEndpoint = endpointProvider.currentEndpoint.value
}
