package com.amitia.core.network.endpoint

import java.security.SecureRandom
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow

@Singleton
class LocalAuthTokenProvider @Inject constructor() {

    private val secureRandom = SecureRandom()
    private val tokenState = MutableStateFlow<String?>(null)

    val token: StateFlow<String?> = tokenState

    fun generateToken(): String {
        val bytes = ByteArray(TOKEN_BYTE_LENGTH)
        secureRandom.nextBytes(bytes)
        val hex = bytes.joinToString("") { byte ->
            "%02x".format(byte.toInt() and 0xFF)
        }
        tokenState.value = hex
        return hex
    }

    fun persistToken(token: String) {
        tokenState.value = token
    }

    fun currentToken(): String? = tokenState.value

    fun clearToken() {
        tokenState.value = null
    }

    companion object {
        private const val TOKEN_BYTE_LENGTH = 32
    }
}
