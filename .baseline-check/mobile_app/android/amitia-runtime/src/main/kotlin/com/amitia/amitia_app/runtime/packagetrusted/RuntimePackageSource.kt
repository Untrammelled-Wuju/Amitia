package com.amitia.amitia_app.runtime.packagetrusted

sealed interface RuntimePackageSourceResult {
    data class Ready(
        val reference: RuntimePackageReference,
    ) : RuntimePackageSourceResult

    data class Failed(
        val code: RuntimePackageSourceErrorCode,
        val message: String,
    ) : RuntimePackageSourceResult
}

enum class RuntimePackageSourceErrorCode {
    BUNDLED_PACKAGE_MISSING,
    BUNDLED_PACKAGE_HASH_MISMATCH,
    PACKAGE_CACHE_INVALID,
    PACKAGE_MATERIALIZE_FAILED,
}

interface RuntimePackageSource {
    fun materialize(): RuntimePackageSourceResult
}
