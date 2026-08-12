package com.amitia.amitia_app.runtime.manifest

data class RuntimeManifestPaths(
    val rootfsHostPath: String,
    val runtimeRootHostPath: String,
    val configHostPath: String,
    val dataHostPath: String,
    val cacheHostPath: String,
    val logHostPath: String,
    val runHostPath: String,
    val guestRuntimeRoot: String,
    val guestConfigRoot: String,
    val guestDataRoot: String,
    val guestCacheRoot: String,
    val guestLogRoot: String,
    val guestRunRoot: String,
) {
    init {
        require(rootfsHostPath.isNotBlank()) { "rootfsHostPath must not be blank" }
        require(runtimeRootHostPath.isNotBlank()) { "runtimeRootHostPath must not be blank" }
        require(configHostPath.isNotBlank()) { "configHostPath must not be blank" }
        require(dataHostPath.isNotBlank()) { "dataHostPath must not be blank" }
        require(cacheHostPath.isNotBlank()) { "cacheHostPath must not be blank" }
        require(logHostPath.isNotBlank()) { "logHostPath must not be blank" }
        require(runHostPath.isNotBlank()) { "runHostPath must not be blank" }
        require(guestRuntimeRoot.isNotBlank()) { "guestRuntimeRoot must not be blank" }
        require(guestConfigRoot.isNotBlank()) { "guestConfigRoot must not be blank" }
        require(guestDataRoot.isNotBlank()) { "guestDataRoot must not be blank" }
        require(guestCacheRoot.isNotBlank()) { "guestCacheRoot must not be blank" }
        require(guestLogRoot.isNotBlank()) { "guestLogRoot must not be blank" }
        require(guestRunRoot.isNotBlank()) { "guestRunRoot must not be blank" }
    }

    companion object {
        const val JSON_ROOTFS_HOST_PATH: String = "rootfsHostPath"
        const val JSON_RUNTIME_ROOT_HOST_PATH: String = "runtimeRootHostPath"
        const val JSON_CONFIG_HOST_PATH: String = "configHostPath"
        const val JSON_DATA_HOST_PATH: String = "dataHostPath"
        const val JSON_CACHE_HOST_PATH: String = "cacheHostPath"
        const val JSON_LOG_HOST_PATH: String = "logHostPath"
        const val JSON_RUN_HOST_PATH: String = "runHostPath"
        const val JSON_GUEST_RUNTIME_ROOT: String = "guestRuntimeRoot"
        const val JSON_GUEST_CONFIG_ROOT: String = "guestConfigRoot"
        const val JSON_GUEST_DATA_ROOT: String = "guestDataRoot"
        const val JSON_GUEST_CACHE_ROOT: String = "guestCacheRoot"
        const val JSON_GUEST_LOG_ROOT: String = "guestLogRoot"
        const val JSON_GUEST_RUN_ROOT: String = "guestRunRoot"

        const val GUEST_RUNTIME_ROOT: String = "/opt/amitia"
        const val GUEST_CONFIG_ROOT: String = "/etc/amitia"
        const val GUEST_DATA_ROOT: String = "/var/lib/amitia"
        const val GUEST_CACHE_ROOT: String = "/var/cache/amitia"
        const val GUEST_LOG_ROOT: String = "/var/log/amitia"
        const val GUEST_RUN_ROOT: String = "/run/amitia"
    }
}
