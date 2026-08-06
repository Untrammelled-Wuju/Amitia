package com.amitia.amitia_app.runtime.api

enum class RuntimeState {
    UNKNOWN,
    NOT_INSTALLED,
    INSTALLING,
    INSTALLED,
    VERIFYING,
    STARTING,
    READY,
    DEGRADED,
    STOPPING,
    STOPPED,
    REPAIRING,
    CORRUPTED,
    FAILED
}
