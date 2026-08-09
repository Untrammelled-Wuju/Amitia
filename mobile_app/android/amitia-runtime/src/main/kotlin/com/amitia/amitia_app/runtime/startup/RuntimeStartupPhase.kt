package com.amitia.amitia_app.runtime.startup

internal enum class RuntimeStartupPhase {
    WAITING_FOR_PROOT,
    WAITING_FOR_BACKEND_LIVE,
    WAITING_FOR_BACKEND_READY,
    READY,
    FAILED,
    CANCELLED
}
