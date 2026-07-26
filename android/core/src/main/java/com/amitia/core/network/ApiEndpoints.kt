package com.amitia.core.network

object ApiEndpoints {
    const val BASE = "/api"

    const val WEB_CHAT_SEND_STREAM = "/api/web-chat/send-stream"
    const val WEB_CHAT_HISTORY = "/api/web-chat/history"
    const val WEB_CHAT_CONVERSATIONS = "/api/web-chat/conversations"
    const val WEB_CHAT_DELETE = "/api/web-chat/delete"

    const val AUTH_LOGIN = "/api/auth/login"
    const val AUTH_LOGOUT = "/api/auth/logout"
    const val AUTH_REFRESH = "/api/auth/refresh"
    const val AUTH_PROFILE = "/api/auth/profile"

    const val CHARACTER_LIST = "/api/characters"
    const val CHARACTER_DETAIL = "/api/characters/{id}"
    const val CHARACTER_CREATE = "/api/characters"
    const val CHARACTER_UPDATE = "/api/characters/{id}"
    const val CHARACTER_DELETE = "/api/characters/{id}"

    const val MEMORY_LIST = "/api/memory"
    const val MEMORY_CREATE = "/api/memory"
    const val MEMORY_DELETE = "/api/memory/{id}"

    const val MODELS_LIST = "/api/models"
    const val MODELS_DOWNLOAD = "/api/models/{id}/download"
    const val MODELS_DELETE = "/api/models/{id}"

    const val CHANNELS_LIST = "/api/channels"
    const val CHANNELS_BIND = "/api/channels/bind"
    const val CHANNELS_UNBIND = "/api/channels/unbind"

    const val RUNTIME_STATUS = "/api/runtime/status"
    const val RUNTIME_START = "/api/runtime/start"
    const val RUNTIME_STOP = "/api/runtime/stop"
    const val RUNTIME_RESTART = "/api/runtime/restart"
    const val RUNTIME_LOGS = "/api/runtime/logs"
    const val RUNTIME_METRICS = "/api/runtime/metrics"

    const val SETTINGS_GET = "/api/settings"
    const val SETTINGS_UPDATE = "/api/settings"

    const val HEALTH_CHECK = "/api/health"
    const val VERSION_INFO = "/api/version"
}
