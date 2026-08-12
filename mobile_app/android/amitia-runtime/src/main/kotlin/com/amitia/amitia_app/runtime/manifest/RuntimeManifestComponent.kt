package com.amitia.amitia_app.runtime.manifest

data class RuntimeManifestComponent(
    val id: String,
    val version: String?,
    val architecture: String?,
    val root: String,
    val entry: String?,
    val sha256: String?,
    val treeSha256: String?,
    val source: String,
) {
    init {
        require(id.isNotBlank()) { "id must not be blank" }
        require(root.isNotBlank()) { "root must not be blank" }
        require(source.isNotBlank()) { "source must not be blank" }
        if (sha256 != null) require(sha256.trim().length == 64) { "sha256 must be 64 hex chars when present" }
        if (treeSha256 != null) require(treeSha256.trim().length == 64) { "treeSha256 must be 64 hex chars when present" }
    }

    companion object {
        const val JSON_ID: String = "id"
        const val JSON_VERSION: String = "version"
        const val JSON_ARCHITECTURE: String = "architecture"
        const val JSON_ROOT: String = "root"
        const val JSON_ENTRY: String = "entry"
        const val JSON_SHA256: String = "sha256"
        const val JSON_TREE_SHA256: String = "treeSha256"
        const val JSON_SOURCE: String = "source"

        const val ID_BACKEND: String = "runtime.backend"
        const val ID_NODE: String = "runtime.node"
        const val ID_NODE_SCRIPTS: String = "runtime.node-scripts"
        const val ID_QDRANT: String = "runtime.qdrant"
        const val ID_PLUGIN_HOST: String = "runtime.plugin-host"
        const val ID_TASK_HOST: String = "runtime.task-host"
        const val ID_PROOT: String = "runtime.proot"

        const val SOURCE_PACKAGE: String = "package"
        const val SOURCE_ANDROID_NATIVE: String = "android-native"
    }
}
