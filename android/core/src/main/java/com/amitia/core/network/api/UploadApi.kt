package com.amitia.core.network.api

import com.amitia.core.model.UploadResponse
import okhttp3.MultipartBody
import retrofit2.http.Multipart
import retrofit2.http.POST
import retrofit2.http.Part
import retrofit2.http.Query

interface UploadApi {

    @Multipart
    @POST("/api/upload")
    suspend fun upload(
        @Part file: MultipartBody.Part,
        @Query("type") type: String = "image"
    ): UploadResponse

    @Multipart
    @POST("/api/asr/upload")
    suspend fun uploadAudio(
        @Part file: MultipartBody.Part
    ): UploadResponse
}
