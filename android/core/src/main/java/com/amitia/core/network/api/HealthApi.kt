package com.amitia.core.network.api

import com.amitia.core.model.HealthResponse
import retrofit2.http.GET

interface HealthApi {

    @GET("/api/health")
    suspend fun health(): HealthResponse

    @GET("/api/health/circuit-breakers")
    suspend fun circuitBreakers(): HealthResponse

    @GET("/api/version")
    suspend fun version(): HealthResponse
}
