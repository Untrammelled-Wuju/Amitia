package com.amitia.core.common

object Constants {
    const val LOCAL_HOST = "127.0.0.1"
    const val BACKEND_PORT = 18899
    const val QDRANT_PORT = 19178
    const val SURREALDB_PORT = 18000

    const val BACKEND_BASE_URL = "http://127.0.0.1:18899"
    const val QDRANT_BASE_URL = "http://127.0.0.1:19178"
    const val SURREALDB_BASE_URL = "http://127.0.0.1:18000"

    const val DEFAULT_PAGE_SIZE = 20
    const val CONNECT_TIMEOUT_SECONDS = 30L
    const val READ_TIMEOUT_SECONDS = 60L
    const val WRITE_TIMEOUT_SECONDS = 60L

    const val DATABASE_NAME = "amitia.db"
    const val DATASTORE_NAME = "amitia_preferences"
    const val SECURE_DATASTORE_NAME = "amitia_secure"

    const val NOTIFICATION_CHANNEL_CORE = "amitia_core_service"
    const val NOTIFICATION_CHANNEL_CHAT = "amitia_chat"
    const val NOTIFICATION_CHANNEL_DOWNLOAD = "amitia_download"
}
