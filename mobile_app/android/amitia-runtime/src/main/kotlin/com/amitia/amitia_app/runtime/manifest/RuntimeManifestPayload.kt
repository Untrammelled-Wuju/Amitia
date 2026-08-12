package com.amitia.amitia_app.runtime.manifest

data class RuntimeManifestPayload(
    val id: String,
    val role: String,
    val sha256: String,
    val size: Long,
) {
    init {
        require(id.isNotBlank()) { "id must not be blank" }
        require(role.isNotBlank()) { "role must not be blank" }
        require(sha256.trim().length == 64) { "sha256 must be 64 hex chars" }
        require(size >= 0L) { "size must not be negative" }
    }

    companion object {
        const val JSON_ID: String = "id"
        const val JSON_ROLE: String = "role"
        const val JSON_SHA256: String = "sha256"
        const val JSON_SIZE: String = "size"

        const val ROLE_ROOTFS: String = "rootfs"
        const val ROLE_RUNTIME: String = "runtime"
    }
}
