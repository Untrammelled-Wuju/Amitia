package com.amitia.core.repository

import com.amitia.core.model.CharacterCreateRequest
import com.amitia.core.model.CharacterDto
import com.amitia.core.model.CharacterSwitchRequest
import com.amitia.core.model.CharacterUpdateRequest
import com.amitia.core.network.api.CharacterApi
import com.amitia.core.network.client.AmitiaApiClient
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class CharacterRepository @Inject constructor(
    private val apiClient: AmitiaApiClient
) {

    private val api: CharacterApi by lazy { apiClient.service(CharacterApi::class.java) }

    suspend fun list(page: Int = 1, pageSize: Int = 20): List<CharacterDto> {
        return api.listCharacters(page, pageSize)
    }

    suspend fun get(id: String): CharacterDto {
        return api.getCharacter(id)
    }

    suspend fun create(request: CharacterCreateRequest): CharacterDto {
        return api.createCharacter(request)
    }

    suspend fun update(id: String, request: CharacterUpdateRequest): CharacterDto {
        return api.updateCharacter(id, request)
    }

    suspend fun delete(id: String) {
        api.deleteCharacter(id)
    }

    suspend fun switchCurrent(characterId: String): CharacterDto {
        return api.switchCurrent(CharacterSwitchRequest(characterId = characterId))
    }

    suspend fun getCurrent(): CharacterDto {
        return api.getCurrentCharacter()
    }
}
