package com.amitia.core.model

import kotlinx.serialization.Serializable

@Serializable
data class AuthLoginRequest(
    val username: String? = null,
    val password: String? = null,
    val token: String? = null
)

@Serializable
data class AuthLoginResponse(
    val token: String,
    val tokenType: String? = "Bearer",
    val expiresIn: Long? = null,
    val userId: String? = null,
    val username: String? = null
)

@Serializable
data class AuthProfileDto(
    val id: String,
    val username: String? = null,
    val displayName: String? = null,
    val avatar: String? = null,
    val roles: List<String> = emptyList()
)
