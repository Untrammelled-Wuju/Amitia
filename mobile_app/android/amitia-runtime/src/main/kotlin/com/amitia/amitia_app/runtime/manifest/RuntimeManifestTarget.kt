package com.amitia.amitia_app.runtime.manifest

internal data class RuntimeManifestTarget(
    val hostPlatform: String,
    val hostAbi: String,
    val runtimeKind: String,
    val guestPlatform: String,
    val guestArchitecture: String,
    val distribution: String,
    val distributionRelease: String,
) {
    init {
        require(hostPlatform.isNotBlank()) { "hostPlatform must not be blank" }
        require(hostAbi.isNotBlank()) { "hostAbi must not be blank" }
        require(runtimeKind.isNotBlank()) { "runtimeKind must not be blank" }
        require(guestPlatform.isNotBlank()) { "guestPlatform must not be blank" }
        require(guestArchitecture.isNotBlank()) { "guestArchitecture must not be blank" }
        require(distribution.isNotBlank()) { "distribution must not be blank" }
        require(distributionRelease.isNotBlank()) { "distributionRelease must not be blank" }
    }

    companion object {
        const val JSON_HOST_PLATFORM: String = "hostPlatform"
        const val JSON_HOST_ABI: String = "hostAbi"
        const val JSON_RUNTIME_KIND: String = "runtimeKind"
        const val JSON_GUEST_PLATFORM: String = "guestPlatform"
        const val JSON_GUEST_ARCHITECTURE: String = "guestArchitecture"
        const val JSON_DISTRIBUTION: String = "distribution"
        const val JSON_DISTRIBUTION_RELEASE: String = "distributionRelease"

        const val HOST_PLATFORM_ANDROID: String = "android"
        const val RUNTIME_KIND_PROOT: String = "proot"
        const val GUEST_PLATFORM_LINUX: String = "linux"
    }
}
