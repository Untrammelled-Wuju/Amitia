package com.amitia.amitia_app.runtime.connection

internal class BackendConnectionCredential private constructor(
    private val token: String,
) {
    fun reveal(): String = token

    override fun toString(): String = "BackendConnectionCredential([REDACTED])"

    companion object {
        fun create(token: String): BackendConnectionCredential {
            val trimmed = token.trim()
            require(trimmed.isNotEmpty()) { "token must not be empty" }
            require(trimmed.length >= 32) { "token must be at least 32 characters" }
            require(!trimmed.contains('\u0000')) { "token must not contain NUL" }
            require(!trimmed.contains('\r')) { "token must not contain CR" }
            require(!trimmed.contains('\n')) { "token must not contain LF" }
            return BackendConnectionCredential(trimmed)
        }
    }
}
