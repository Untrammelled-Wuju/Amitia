package com.amitia.core.network.api

import com.amitia.core.model.ModelConfigDto
import com.amitia.core.model.ModelConfigUpdateRequest
import com.amitia.core.model.ModelDto
import okhttp3.ResponseBody
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.PUT
import retrofit2.http.Path
import retrofit2.http.Query

interface ModelApi {

    @GET("/api/models")
    suspend fun listModels(
        @Query("type") type: String? = null,
        @Query("provider") provider: String? = null
    ): List<ModelDto>

    @GET("/api/models/{id}")
    suspend fun getModel(@Path("id") id: String): ModelDto

    @POST("/api/models")
    suspend fun createModel(@Body request: ModelDto): ModelDto

    @PUT("/api/models/{id}")
    suspend fun updateModel(
        @Path("id") id: String,
        @Body request: ModelDto
    ): ModelDto

    @DELETE("/api/models/{id}")
    suspend fun deleteModel(@Path("id") id: String)

    @GET("/api/models/config")
    suspend fun getConfig(): ModelConfigDto

    @PUT("/api/models/config")
    suspend fun updateConfig(@Body request: ModelConfigUpdateRequest): ModelConfigDto

    @POST("/api/models/{id}/download")
    suspend fun downloadModel(@Path("id") id: String): ResponseBody
}
