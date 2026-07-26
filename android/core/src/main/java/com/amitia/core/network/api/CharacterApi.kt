package com.amitia.core.network.api

import com.amitia.core.model.CharacterCreateRequest
import com.amitia.core.model.CharacterDto
import com.amitia.core.model.CharacterSwitchRequest
import com.amitia.core.model.CharacterUpdateRequest
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.PUT
import retrofit2.http.Path
import retrofit2.http.Query

interface CharacterApi {

    @GET("/api/characters")
    suspend fun listCharacters(
        @Query("page") page: Int = 1,
        @Query("pageSize") pageSize: Int = 20
    ): List<CharacterDto>

    @GET("/api/characters/{id}")
    suspend fun getCharacter(@Path("id") id: String): CharacterDto

    @POST("/api/characters")
    suspend fun createCharacter(@Body request: CharacterCreateRequest): CharacterDto

    @PUT("/api/characters/{id}")
    suspend fun updateCharacter(
        @Path("id") id: String,
        @Body request: CharacterUpdateRequest
    ): CharacterDto

    @DELETE("/api/characters/{id}")
    suspend fun deleteCharacter(@Path("id") id: String)

    @POST("/api/characters/switch")
    suspend fun switchCurrent(@Body request: CharacterSwitchRequest): CharacterDto

    @GET("/api/characters/current")
    suspend fun getCurrentCharacter(): CharacterDto
}
