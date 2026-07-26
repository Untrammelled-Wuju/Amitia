package com.amitia.core.network.endpoint

import com.google.common.truth.Truth.assertThat
import org.junit.Test

class LocalAuthTokenProviderTest {

    @Test
    fun generateToken_returns_64_char_hex_string() {
        val provider = LocalAuthTokenProvider()

        val token = provider.generateToken()

        assertThat(token).hasLength(64)
        assertThat(token.matches(Regex("^[0-9a-f]{64}$"))).isTrue()
    }

    @Test
    fun generateToken_emits_token_to_stateFlow() {
        val provider = LocalAuthTokenProvider()

        val token = provider.generateToken()

        assertThat(provider.token.value).isEqualTo(token)
        assertThat(provider.currentToken()).isEqualTo(token)
    }

    @Test
    fun generateToken_produces_different_tokens_on_sequential_calls() {
        val provider = LocalAuthTokenProvider()

        val first = provider.generateToken()
        val second = provider.generateToken()

        assertThat(first).isNotEqualTo(second)
    }

    @Test
    fun persistToken_overwrites_existing_token() {
        val provider = LocalAuthTokenProvider()
        provider.generateToken()

        provider.persistToken("custom-token-xyz")

        assertThat(provider.currentToken()).isEqualTo("custom-token-xyz")
        assertThat(provider.token.value).isEqualTo("custom-token-xyz")
    }

    @Test
    fun clearToken_resets_token_to_null() {
        val provider = LocalAuthTokenProvider()
        provider.generateToken()

        provider.clearToken()

        assertThat(provider.currentToken()).isNull()
        assertThat(provider.token.value).isNull()
    }

    @Test
    fun initial_token_state_is_null() {
        val provider = LocalAuthTokenProvider()

        assertThat(provider.currentToken()).isNull()
        assertThat(provider.token.value).isNull()
    }
}
