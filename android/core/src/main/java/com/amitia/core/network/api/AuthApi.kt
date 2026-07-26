package com.amitia.core.network.api

import com.amitia.core.model.AuthLoginRequest
import com.amitia.core.model.AuthLoginResponse
import com.amitia.core.model.AuthProfileDto
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.Header
import retrofit2.http.POST

interface AuthApi {

    @POST("/api/auth/login")
    suspend fun login(@Body request: AuthLoginRequest): AuthLoginResponse

    @POST("/api/auth/logout")
    suspend fun logout(@Header("Authorization") authHeader: String?)

    @POST("/api/auth/refresh")
    suspend fun refresh(@Header("Authorization") authHeader: String?): AuthLoginResponse

    @GET("/api/auth/profile")
    suspend fun profile(): AuthProfileDto
}
