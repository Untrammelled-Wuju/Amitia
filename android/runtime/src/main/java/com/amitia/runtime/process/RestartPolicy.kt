package com.amitia.runtime.process

enum class RestartPolicy(val maxRetries: Int) {

    NEVER(maxRetries = 0),

    ON_FAILURE(maxRetries = 3),

    ALWAYS(maxRetries = 3);

    fun shouldRestart(currentCrashCount: Int): Boolean = when (this) {
        NEVER -> false
        ON_FAILURE -> currentCrashCount <= maxRetries
        ALWAYS -> currentCrashCount <= maxRetries
    }

    companion object {
        val DEFAULT: RestartPolicy = ON_FAILURE
    }
}
