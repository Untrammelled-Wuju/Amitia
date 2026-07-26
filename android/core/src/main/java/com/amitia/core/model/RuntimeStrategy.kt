package com.amitia.core.model

enum class RuntimeStrategy {
    ALWAYS_ON,
    ON_DEMAND,
    REMOTE_ONLY;

    companion object {
        val DEFAULT: RuntimeStrategy = ON_DEMAND
    }
}
